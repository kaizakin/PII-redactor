package detector

import "testing"

func TestPhoneDetector(t *testing.T) {
	d := NewPhoneDetector("US")

	t.Run("finds US numbers in common formats", func(t *testing.T) {
		// Area code 555 does not exist in the NANP; 415-555-01XX is the
		// range NANP reserves specifically for fictional/test numbers, so
		// libphonenumber accepts it as valid while still being obviously fake.
		text := "Call us at (415) 555-0132 or +1-415-555-0148 for support."
		matches := d.Detect(text)
		if len(matches) != 2 {
			t.Fatalf("expected 2 matches, got %d: %+v", len(matches), matches)
		}
		for _, m := range matches {
			if m.Type != TypePhone {
				t.Errorf("expected TypePhone, got %s", m.Type)
			}
		}
	})

	t.Run("finds an international number with country code", func(t *testing.T) {
		text := "UK office: +44 20 7946 0958."
		if matches := d.Detect(text); len(matches) != 1 {
			t.Errorf("expected 1 match, got %d: %+v", len(matches), matches)
		}
	})

	t.Run("ignores IP addresses and invalid area codes", func(t *testing.T) {
		text := "Server 192.168.1.100 rejected connection from 999.999.999.999."
		if matches := d.Detect(text); len(matches) != 0 {
			t.Errorf("expected no matches, got %+v", matches)
		}
	})

	t.Run("ignores short digit runs", func(t *testing.T) {
		text := "The meeting is scheduled for 12-25 at gate 42."
		if matches := d.Detect(text); len(matches) != 0 {
			t.Errorf("expected no matches, got %+v", matches)
		}
	})
}
