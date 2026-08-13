package detector

import "testing"

func TestIPDetector(t *testing.T) {
	d := NewIPDetector()

	t.Run("finds valid IPv4 and IPv6 addresses", func(t *testing.T) {
		text := "Server at 192.168.1.100 forwarded to 2001:db8::1 and back to ::1."
		matches := d.Detect(text)
		want := map[string]bool{"192.168.1.100": false, "2001:db8::1": false, "::1": false}
		if len(matches) != len(want) {
			t.Fatalf("expected %d matches, got %d: %+v", len(want), len(matches), matches)
		}
		for _, m := range matches {
			if _, ok := want[m.Value]; !ok {
				t.Errorf("unexpected match value %q", m.Value)
			}
			want[m.Value] = true
			if m.Type != TypeIPAddress {
				t.Errorf("expected TypeIPAddress, got %s", m.Type)
			}
			if text[m.Start:m.End] != m.Value {
				t.Errorf("offsets do not match Value: %q vs %q", text[m.Start:m.End], m.Value)
			}
		}
		for v, found := range want {
			if !found {
				t.Errorf("expected to find %q", v)
			}
		}
	})

	t.Run("rejects out-of-range octets and clock times", func(t *testing.T) {
		text := "Invalid address 999.999.999.999, meeting time is 12:30:45."
		if matches := d.Detect(text); len(matches) != 0 {
			t.Errorf("expected no matches, got %+v", matches)
		}
	})
}
