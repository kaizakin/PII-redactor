package detector

import "testing"

func TestEmailDetector(t *testing.T) {
	d := NewEmailDetector()

	t.Run("finds plain and tagged addresses", func(t *testing.T) {
		text := "Contact rashi.patil@gmail.com or support+billing@example.co.uk for help."
		matches := d.Detect(text)
		if len(matches) != 2 {
			t.Fatalf("expected 2 matches, got %d: %+v", len(matches), matches)
		}
		if matches[0].Value != "rashi.patil@gmail.com" {
			t.Errorf("unexpected first match: %q", matches[0].Value)
		}
		if matches[1].Value != "support+billing@example.co.uk" {
			t.Errorf("unexpected second match: %q", matches[1].Value)
		}
		for _, m := range matches {
			if m.Type != TypeEmail {
				t.Errorf("expected TypeEmail, got %s", m.Type)
			}
			if text[m.Start:m.End] != m.Value {
				t.Errorf("offsets do not match Value: %q vs %q", text[m.Start:m.End], m.Value)
			}
		}
	})

	t.Run("ignores non-email text", func(t *testing.T) {
		text := "Ticket #1234 was resolved on version 2.0 of the app."
		if matches := d.Detect(text); len(matches) != 0 {
			t.Errorf("expected no matches, got %+v", matches)
		}
	})
}
