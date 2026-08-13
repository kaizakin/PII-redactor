package detector

import (
	"testing"
	"time"
)

func fixedClock(year int) func() time.Time {
	return func() time.Time { return time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC) }
}

func TestDOBDetector(t *testing.T) {
	d := NewDOBDetector()
	d.now = fixedClock(2026)

	t.Run("finds numeric dates in month-first and ISO order", func(t *testing.T) {
		text := "DOB: 05/12/1990. Registered on 1990-05-12 in the system."
		matches := d.Detect(text)
		if len(matches) != 2 {
			t.Fatalf("expected 2 matches, got %d: %+v", len(matches), matches)
		}
	})

	t.Run("finds written-out month dates", func(t *testing.T) {
		text := "Born on January 5, 1990, and again referenced as 5 January 1990."
		matches := d.Detect(text)
		if len(matches) != 2 {
			t.Fatalf("expected 2 matches, got %d: %+v", len(matches), matches)
		}
	})

	t.Run("finds abbreviated month with period", func(t *testing.T) {
		text := "Date of birth: Jan. 5, 1990."
		if matches := d.Detect(text); len(matches) != 1 {
			t.Fatalf("expected 1 match, got %d: %+v", len(matches), matches)
		}
	})

	t.Run("rejects impossible calendar dates", func(t *testing.T) {
		text := "Invalid entries: 13/45/2020 and February 30, 1990."
		if matches := d.Detect(text); len(matches) != 0 {
			t.Errorf("expected no matches, got %+v", matches)
		}
	})

	t.Run("rejects dates outside a plausible birth year range", func(t *testing.T) {
		text := "Founded 07/04/1785. Delivery scheduled for 12/31/2099."
		if matches := d.Detect(text); len(matches) != 0 {
			t.Errorf("expected no matches, got %+v", matches)
		}
	})
}
