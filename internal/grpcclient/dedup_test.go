package grpcclient

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type countingClient struct {
	calls     atomic.Int64
	startOnce sync.Once
	started   chan struct{}
	// gate lets the test hold the first call open until every concurrent
	// duplicate has had a chance to join it, so the dedup is proven against
	// calls that are genuinely in-flight together, not just ones that
	// happen to run one after another.
	gate chan struct{}
}

func (c *countingClient) Analyze(ctx context.Context, text string) ([]Entity, error) {
	c.calls.Add(1)
	c.startOnce.Do(func() { close(c.started) })
	<-c.gate
	return []Entity{{Type: "PERSON", Start: 0, End: len(text), Text: text, Confidence: 0.9}}, nil
}

func (c *countingClient) Close() error { return nil }

func TestDedupingClientCollapsesConcurrentIdenticalCalls(t *testing.T) {
	inner := &countingClient{started: make(chan struct{}), gate: make(chan struct{})}
	client := NewDedupingClient(inner)

	const callers = 3
	var wg sync.WaitGroup
	results := make([][]Entity, callers)

	// Start the first caller alone and wait until it's confirmed inside
	// the underlying Analyze call (and therefore registered in
	// singleflight) before starting the rest, so they're guaranteed to
	// find an in-flight call to join rather than racing to start their own.
	wg.Go(func() {
		entities, err := client.Analyze(context.Background(), "Rashi Patil works at Acme Corp")
		if err != nil {
			t.Errorf("Analyze: %v", err)
			return
		}
		results[0] = entities
	})
	<-inner.started

	for i := 1; i < callers; i++ {
		wg.Go(func() {
			entities, err := client.Analyze(context.Background(), "Rashi Patil works at Acme Corp")
			if err != nil {
				t.Errorf("Analyze: %v", err)
				return
			}
			results[i] = entities
		})
	}
	// Give the just-launched callers a moment to reach singleflight.Do
	// and register as waiters on the first call before it's released.
	time.Sleep(50 * time.Millisecond)
	close(inner.gate)
	wg.Wait()

	if got := inner.calls.Load(); got != 1 {
		t.Errorf("expected exactly 1 underlying call, got %d", got)
	}
	for i, r := range results {
		if len(r) != 1 || r[0].Text != "Rashi Patil works at Acme Corp" {
			t.Errorf("caller %d got unexpected result: %+v", i, r)
		}
	}
}

func TestDedupingClientDistinctTextsBothCall(t *testing.T) {
	inner := &countingClient{started: make(chan struct{}), gate: make(chan struct{})}
	close(inner.gate) // don't need to synchronize concurrency for this case
	client := NewDedupingClient(inner)

	if _, err := client.Analyze(context.Background(), "text one"); err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if _, err := client.Analyze(context.Background(), "text two"); err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if got := inner.calls.Load(); got != 2 {
		t.Errorf("expected 2 underlying calls for 2 distinct texts, got %d", got)
	}
}

func TestDedupingClientCachesCompletedCallsAcrossNonOverlappingRequests(t *testing.T) {
	// This is the case singleflight alone misses: two calls for the same
	// text that are sequential, not concurrent — e.g. two different runs
	// in a document carrying identical boilerplate text, or (as measured
	// against a real 4800-run stress document) the three per-entity-type
	// calls simply not overlapping because the RPC itself is faster than
	// the jitter between when each goroutine gets scheduled.
	inner := &countingClient{started: make(chan struct{}), gate: make(chan struct{})}
	close(inner.gate)
	client := NewDedupingClient(inner)

	first, err := client.Analyze(context.Background(), "Rashi Patil works at Acme Corp")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	second, err := client.Analyze(context.Background(), "Rashi Patil works at Acme Corp")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if got := inner.calls.Load(); got != 1 {
		t.Errorf("expected exactly 1 underlying call, the second should have been a cache hit; got %d", got)
	}
	if len(first) != len(second) || first[0].Text != second[0].Text {
		t.Errorf("expected identical results from cache, got %+v and %+v", first, second)
	}
}

func TestDedupingClientClose(t *testing.T) {
	inner := &countingClient{started: make(chan struct{}), gate: make(chan struct{})}
	client := NewDedupingClient(inner)
	if err := client.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}
