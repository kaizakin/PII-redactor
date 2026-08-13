package detector

import "testing"

func TestCreditCardDetector(t *testing.T) {
	d := NewCreditCardDetector()

	t.Run("finds a Luhn-valid card number", func(t *testing.T) {
		text := "Card on file: 4111 1111 1111 1111 (Visa test number)."
		matches := d.Detect(text)
		if len(matches) != 1 {
			t.Fatalf("expected 1 match, got %d: %+v", len(matches), matches)
		}
		if matches[0].Value != "4111 1111 1111 1111" {
			t.Errorf("unexpected match: %q", matches[0].Value)
		}
		if matches[0].Type != TypeCreditCard {
			t.Errorf("expected TypeCreditCard, got %s", matches[0].Type)
		}
	})

	t.Run("finds a dash-separated card number", func(t *testing.T) {
		text := "4111-1111-1111-1111 was charged."
		if matches := d.Detect(text); len(matches) != 1 {
			t.Errorf("expected 1 match, got %d: %+v", len(matches), matches)
		}
	})

	t.Run("rejects a 16-digit number that fails Luhn", func(t *testing.T) {
		text := "Reference order number 4111111111111112 was shipped today."
		if matches := d.Detect(text); len(matches) != 0 {
			t.Errorf("expected no matches for non-Luhn number, got %+v", matches)
		}
	})
}

func TestPassesLuhn(t *testing.T) {
	cases := map[string]bool{
		"4111111111111111": true,
		"4111111111111112": false,
		"79927398713":      true,
		"79927398710":      false,
	}
	for digits, want := range cases {
		if got := passesLuhn(digits); got != want {
			t.Errorf("passesLuhn(%q) = %v, want %v", digits, got, want)
		}
	}
}
