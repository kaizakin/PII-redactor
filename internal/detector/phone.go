package detector

import (
	"regexp"

	"github.com/nyaruka/phonenumbers"
)

// phoneCandidate loosely matches digit runs interleaved with the
// punctuation real phone numbers use (spaces, dashes, dots, parens, a
// leading +). It is intentionally permissive — nowhere near enough on its
// own to claim a match — because the real validation is delegated to
// Google's libphonenumber (via the nyaruka/phonenumbers port) below, which
// knows actual per-country number-length and area-code rules far better
// than a hand-rolled regex ever could.
var phoneCandidate = regexp.MustCompile(`\+?\(?\d{1,4}\)?[\d().\-\s]{6,17}\d`)

// PhoneDetector finds phone numbers using regex candidates validated
// against libphonenumber. DefaultRegion resolves numbers written without a
// country code (e.g. "(555) 123-4567") the same way a person dialing from
// that country would read them.
type PhoneDetector struct {
	defaultRegion string
}

// NewPhoneDetector creates a PhoneDetector. defaultRegion is an ISO 3166-1
// alpha-2 region code (e.g. "US", "GB") used to interpret numbers that
// don't include an explicit country calling code; it defaults to "US".
func NewPhoneDetector(defaultRegion string) *PhoneDetector {
	if defaultRegion == "" {
		defaultRegion = "US"
	}
	return &PhoneDetector{defaultRegion: defaultRegion}
}

func (d *PhoneDetector) Type() PIIType { return TypePhone }

func (d *PhoneDetector) Detect(text string) []Match {
	idx := phoneCandidate.FindAllStringIndex(text, -1)
	if idx == nil {
		return nil
	}
	matches := make([]Match, 0, len(idx))
	for _, loc := range idx {
		candidate := text[loc[0]:loc[1]]
		if !hasEnoughDigits(candidate, 7) {
			continue
		}
		num, err := phonenumbers.Parse(candidate, d.defaultRegion)
		if err != nil || !phonenumbers.IsValidNumber(num) {
			continue
		}
		matches = append(matches, Match{
			Type:  TypePhone,
			Start: loc[0],
			End:   loc[1],
			Value: candidate,
		})
	}
	return matches
}

func hasEnoughDigits(s string, min int) bool {
	count := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			count++
			if count >= min {
				return true
			}
		}
	}
	return false
}
