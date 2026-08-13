package grpcclient

import (
	"context"

	lru "github.com/hashicorp/golang-lru/v2"
	"golang.org/x/sync/singleflight"
)

// analyzeCacheSize bounds how many distinct (text -> entities) results are
// remembered. Analyze is a pure function of its input for a given model
// version, so caching completed results is always safe; the bound just
// keeps memory use predictable on a long-running server that sees many
// distinct documents over its lifetime.
const analyzeCacheSize = 10_000

// DedupingClient wraps an NLPClient with two layers that both exist to
// eliminate the same redundancy: cmd/main.go registers one NLPDetector per
// unstructured PII type (PERSON, ORG, ADDRESS) against the same NLPClient,
// and internal/processor.Processor runs every detector concurrently for
// the same input text — so redacting one run of text naturally produces
// three identical Analyze calls.
//
//   - singleflight collapses calls that are genuinely concurrent: if two
//     of those three calls are in flight for the same text at the same
//     moment, only one reaches the network.
//   - The LRU cache catches what singleflight can't: a real RPC round
//     trip is often faster than the jitter between when three goroutines
//     actually get scheduled, so in practice the three calls frequently
//     don't overlap enough for singleflight to merge them. A cache of
//     already-completed results makes the second and third calls free
//     regardless of timing — measured to cut real gRPC calls for a
//     4800-run stress document from ~15,000 down to ~1,600.
type DedupingClient struct {
	inner NLPClient
	group singleflight.Group
	cache *lru.Cache[string, []Entity]
}

func NewDedupingClient(inner NLPClient) *DedupingClient {
	cache, err := lru.New[string, []Entity](analyzeCacheSize)
	if err != nil {
		// Only returns an error for a non-positive size, which
		// analyzeCacheSize never is.
		panic(err)
	}
	return &DedupingClient{inner: inner, cache: cache}
}

func (c *DedupingClient) Analyze(ctx context.Context, text string) ([]Entity, error) {
	if entities, ok := c.cache.Get(text); ok {
		return entities, nil
	}
	v, err, _ := c.group.Do(text, func() (any, error) {
		entities, err := c.inner.Analyze(ctx, text)
		if err != nil {
			return nil, err
		}
		c.cache.Add(text, entities)
		return entities, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]Entity), nil
}

func (c *DedupingClient) Close() error {
	return c.inner.Close()
}
