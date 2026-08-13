package grpcclient

import (
	"context"

	"golang.org/x/sync/singleflight"
)

// DedupingClient wraps an NLPClient so that concurrent Analyze calls for
// the exact same text share one underlying call instead of each hitting
// the network separately.
//
// This matters because cmd/main.go registers one NLPDetector per
// unstructured PII type (PERSON, ORG, ADDRESS) against the same
// NLPClient, and internal/processor.Processor runs every detector
// concurrently for the same input text — without deduping, redacting one
// run of text triggers three identical Analyze calls to the Python worker
// instead of one, and that 3x multiplies across every run in a document.
type DedupingClient struct {
	inner NLPClient
	group singleflight.Group
}

func NewDedupingClient(inner NLPClient) *DedupingClient {
	return &DedupingClient{inner: inner}
}

func (c *DedupingClient) Analyze(ctx context.Context, text string) ([]Entity, error) {
	v, err, _ := c.group.Do(text, func() (any, error) {
		return c.inner.Analyze(ctx, text)
	})
	if err != nil {
		return nil, err
	}
	return v.([]Entity), nil
}

func (c *DedupingClient) Close() error {
	return c.inner.Close()
}
