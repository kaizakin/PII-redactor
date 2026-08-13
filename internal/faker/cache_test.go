package faker

import (
	"fmt"
	"sync"
	"testing"
)

func TestCacheGet(t *testing.T) {
	t.Run("same original always returns the same fake value", func(t *testing.T) {
		c := NewCache()
		first := c.Get("rashi.patil@gmail.com", Email)
		for i := range 5 {
			if got := c.Get("rashi.patil@gmail.com", Email); got != first {
				t.Errorf("call %d: got %q, want %q", i, got, first)
			}
		}
	})

	t.Run("different originals get independent fake values", func(t *testing.T) {
		c := NewCache()
		a := c.Get("523-45-6789", SSN)
		b := c.Get("111-22-3333", SSN)
		if a == b {
			t.Errorf("expected distinct fake values, both were %q", a)
		}
	})

	t.Run("mapping is reproducible across a cold cache", func(t *testing.T) {
		c1 := NewCache()
		c2 := NewCache()
		v1 := c1.Get("4111 1111 1111 1111", CreditCard)
		v2 := c2.Get("4111 1111 1111 1111", CreditCard)
		if v1 != v2 {
			t.Errorf("expected reproducible fake value across cache instances, got %q vs %q", v1, v2)
		}
	})

	t.Run("concurrent access is safe and consistent", func(t *testing.T) {
		c := NewCache()
		const workers = 50
		originals := []string{"a@example.com", "b@example.com", "c@example.com"}

		results := make([][]string, workers)
		var wg sync.WaitGroup
		for w := range workers {
			wg.Add(1)
			go func(w int) {
				defer wg.Done()
				res := make([]string, len(originals))
				for i, o := range originals {
					res[i] = c.Get(o, Email)
				}
				results[w] = res
			}(w)
		}
		wg.Wait()

		for i := range originals {
			want := results[0][i]
			for w := 1; w < workers; w++ {
				if results[w][i] != want {
					t.Errorf("original %q: worker %d got %q, want %q", originals[i], w, results[w][i], want)
				}
			}
		}
	})
}

func TestGenerators(t *testing.T) {
	c := NewCache()
	cases := []struct {
		name string
		gen  Generator
	}{
		{"Email", Email},
		{"Phone", Phone},
		{"SSN", SSN},
		{"CreditCard", CreditCard},
		{"IPv4", IPv4},
		{"DateOfBirth", DateOfBirth},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := c.Get(fmt.Sprintf("original-%s", tc.name), tc.gen)
			if v == "" {
				t.Errorf("expected a non-empty fake value for %s", tc.name)
			}
		})
	}
}
