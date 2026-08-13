package detector

import (
	"regexp"
	"strings"
)

// creditCardCandidate matches runs of 13-19 digits, tolerating the spaces
// or dashes typically used to group card numbers (e.g. "4111 1111 1111
// 1111"). This alone is not precise enough — plenty of 16-digit order or
// invoice numbers would match — so every candidate is additionally
// verified with the Luhn checksum below before being reported.
var creditCardCandidate = regexp.MustCompile(`\b\d(?:[ -]?\d){12,18}\b`)

// CreditCardDetector finds credit card numbers. It combines a digit-count
// regex with the Luhn checksum used by every major card network. This is
// the detector's precision flex: a random 16-digit ticket or order number
// has only a 1-in-10 chance of passing Luhn by coincidence, so requiring
// it eliminates the vast majority of false positives that a bare
// "13-19 digits" regex would produce.
type CreditCardDetector struct{}

func NewCreditCardDetector() *CreditCardDetector { return &CreditCardDetector{} }

func (d *CreditCardDetector) Type() PIIType { return TypeCreditCard }

func (d *CreditCardDetector) Detect(text string) []Match {
	idx := creditCardCandidate.FindAllStringIndex(text, -1)
	if idx == nil {
		return nil
	}
	matches := make([]Match, 0, len(idx))
	for _, loc := range idx {
		candidate := text[loc[0]:loc[1]]
		digits := stripSeparators(candidate)
		if len(digits) < 13 || len(digits) > 19 {
			continue
		}
		if !passesLuhn(digits) {
			continue
		}
		matches = append(matches, Match{
			Type:  TypeCreditCard,
			Start: loc[0],
			End:   loc[1],
			Value: candidate,
		})
	}
	return matches
}

func stripSeparators(s string) string {
	return strings.NewReplacer(" ", "", "-", "").Replace(s)
}

// passesLuhn implements the Luhn checksum (ISO/IEC 7812): starting from the
// rightmost digit, every second digit is doubled, and digits over 9 have 9
// subtracted. A valid number's digits sum to a multiple of 10.
func passesLuhn(digits string) bool {
	sum := 0
	double := false
	for i := len(digits) - 1; i >= 0; i-- {
		d := int(digits[i] - '0')
		if d < 0 || d > 9 {
			return false
		}
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}
