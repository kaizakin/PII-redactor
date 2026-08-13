package detector

import "testing"

func TestSSNDetector(t *testing.T) {
	d := NewSSNDetector()

	t.Run("finds a valid SSN", func(t *testing.T) {
		text := "Employee SSN on file: 523-45-6789."
		matches := d.Detect(text)
		if len(matches) != 1 {
			t.Fatalf("expected 1 match, got %d: %+v", len(matches), matches)
		}
		if matches[0].Value != "523-45-6789" {
			t.Errorf("unexpected match: %q", matches[0].Value)
		}
		if matches[0].Type != TypeSSN {
			t.Errorf("expected TypeSSN, got %s", matches[0].Type)
		}
	})

	t.Run("rejects structurally invalid SSNs", func(t *testing.T) {
		cases := []string{
			"000-45-6789", // area 000
			"666-45-6789", // area 666
			"901-45-6789", // area in 900-999
			"523-00-6789", // group 00
			"523-45-0000", // serial 0000
		}
		for _, c := range cases {
			if matches := d.Detect(c); len(matches) != 0 {
				t.Errorf("expected %q to be rejected, got %+v", c, matches)
			}
		}
	})

	t.Run("ignores dashless digit runs", func(t *testing.T) {
		text := "Order number 523456789 was shipped."
		if matches := d.Detect(text); len(matches) != 0 {
			t.Errorf("expected no matches, got %+v", matches)
		}
	})
}
