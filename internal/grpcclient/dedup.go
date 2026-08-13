package grpcclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	lru "github.com/hashicorp/golang-lru/v2"
	"golang.org/x/sync/singleflight"
)

// analyzeCacheSize bounds how many distinct (text -> entities) results are
// remembered. Analyze is a pure function of its input for a given model
// version, so caching completed results is always safe; the bound just
// keeps memory use predictable on a long-running server that sees many
// distinct documents over its lifetime.
const analyzeCacheSize = 10_000

// imageCacheSize is smaller than analyzeCacheSize because images are much
// larger than a run of text; the same total memory budget holds far fewer
// of them. It still pays for itself on documents with a repeated
// letterhead or logo image across many pages.
const imageCacheSize = 512

// imageResult is what gets cached and singleflight-shared per image.
type imageResult struct {
	data       []byte
	redactions int
}

// DedupingClient wraps an NLPClient with two layers that both exist to
// eliminate the same kind of redundancy: cmd/main.go registers one
// NLPDetector per unstructured PII type (PERSON, ORG, ADDRESS) against the
// same NLPClient, and internal/processor.Processor runs every detector
// concurrently for the same input text — so redacting one run of text
// naturally produces three identical Analyze calls. Repeated images
// (a scanned letterhead or logo on every page) create the same
// redundancy for RedactImage.
//
//   - singleflight collapses calls that are genuinely concurrent: if two
//     calls are in flight for the same text/image at the same moment,
//     only one reaches the network.
//   - The LRU caches catch what singleflight can't: a real RPC round trip
//     is often faster than the jitter between when concurrent goroutines
//     actually get scheduled, so in practice those calls frequently don't
//     overlap enough for singleflight to merge them. A cache of
//     already-completed results makes repeat calls free regardless of
//     timing — measured to cut real gRPC calls for a 4800-run stress
//     document from ~15,000 down to ~1,600.
type DedupingClient struct {
	inner NLPClient

	textGroup singleflight.Group
	textCache *lru.Cache[string, []Entity]

	imageGroup singleflight.Group
	imageCache *lru.Cache[string, imageResult]
}

func NewDedupingClient(inner NLPClient) *DedupingClient {
	textCache, err := lru.New[string, []Entity](analyzeCacheSize)
	if err != nil {
		// Only returns an error for a non-positive size, which
		// analyzeCacheSize never is.
		panic(err)
	}
	imageCache, err := lru.New[string, imageResult](imageCacheSize)
	if err != nil {
		panic(err)
	}
	return &DedupingClient{inner: inner, textCache: textCache, imageCache: imageCache}
}

func (c *DedupingClient) Analyze(ctx context.Context, text string) ([]Entity, error) {
	if entities, ok := c.textCache.Get(text); ok {
		return entities, nil
	}
	v, err, _ := c.textGroup.Do(text, func() (any, error) {
		entities, err := c.inner.Analyze(ctx, text)
		if err != nil {
			return nil, err
		}
		c.textCache.Add(text, entities)
		return entities, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]Entity), nil
}

func (c *DedupingClient) RedactImage(ctx context.Context, data []byte, format string) ([]byte, int, error) {
	key := imageCacheKey(data, format)
	if r, ok := c.imageCache.Get(key); ok {
		return r.data, r.redactions, nil
	}
	v, err, _ := c.imageGroup.Do(key, func() (any, error) {
		redacted, count, err := c.inner.RedactImage(ctx, data, format)
		if err != nil {
			return nil, err
		}
		result := imageResult{data: redacted, redactions: count}
		c.imageCache.Add(key, result)
		return result, nil
	})
	if err != nil {
		return nil, 0, err
	}
	result := v.(imageResult)
	return result.data, result.redactions, nil
}

// imageCacheKey fingerprints image content with a hash rather than using
// the raw bytes as a map/singleflight key directly — cheaper to compare
// and copy, and collision-proof in practice at any realistic scale.
func imageCacheKey(data []byte, format string) string {
	sum := sha256.Sum256(data)
	return format + ":" + hex.EncodeToString(sum[:])
}

func (c *DedupingClient) Close() error {
	return c.inner.Close()
}
