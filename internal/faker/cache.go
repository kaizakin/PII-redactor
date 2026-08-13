// Package faker generates deterministic, consistent fake replacement
// values for detected PII. "Deterministic" here means two things: the same
// original value always maps to the same fake value within a document (so
// entity relationships stay intact — every occurrence of one SSN becomes
// the same fake SSN), and that mapping is reproducible even across a cold
// cache, since it's derived from a hash of the original value rather than
// from insertion order.
package faker

import (
	"crypto/sha256"
	"encoding/binary"
	"sync"

	"github.com/brianvoe/gofakeit/v6"
)

// Generator produces a fake value using a deterministically seeded Faker.
type Generator func(f *gofakeit.Faker) string

// Cache maps original PII values to their generated fake replacements. A
// zero Cache is not usable; construct one with NewCache. Cache is safe for
// concurrent use, since the processor's detectors and replacement pass can
// run across goroutines.
type Cache struct {
	mu    sync.RWMutex
	store map[string]string
}

func NewCache() *Cache {
	return &Cache{store: make(map[string]string)}
}

// Get returns the fake value for original, generating one with gen on a
// cache miss and remembering it for subsequent calls. Concurrent calls for
// the same original value are safe: at most one of them generates the
// value, and every caller — including the ones that lost the race — sees
// the same result.
func (c *Cache) Get(original string, gen Generator) string {
	c.mu.RLock()
	if v, ok := c.store[original]; ok {
		c.mu.RUnlock()
		return v
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	if v, ok := c.store[original]; ok {
		return v
	}
	fake := gen(gofakeit.New(seedFor(original)))
	c.store[original] = fake
	return fake
}

// seedFor derives a stable int64 seed from original so that the fake value
// generated for a given input is reproducible even in a fresh process with
// an empty cache — the mapping lives in the input itself, not in cache
// state.
func seedFor(original string) int64 {
	sum := sha256.Sum256([]byte(original))
	return int64(binary.BigEndian.Uint64(sum[:8]))
}
