package detector

import (
	"regexp"
	"strconv"
)

// ssnPattern matches the standard AAA-GG-SSSS format. SSNs without dashes
// are indistinguishable from arbitrary 9-digit numbers (order IDs, account
// numbers, ...), so we deliberately require the dashes to keep precision
// high rather than guessing at bare digit runs.
var ssnPattern = regexp.MustCompile(`\b(\d{3})-(\d{2})-(\d{4})\b`)

// SSNDetector finds U.S. Social Security Numbers. Beyond the AAA-GG-SSSS
// shape, it applies the SSA's own allocation rules to reject numbers that
// can never have been issued (e.g. area 000/666/900-999, group 00, or
// serial 0000) — the same "structural validation beats bare regex"
// precision trick used by the credit card Luhn check.
type SSNDetector struct{}

func NewSSNDetector() *SSNDetector { return &SSNDetector{} }

func (d *SSNDetector) Type() PIIType { return TypeSSN }

func (d *SSNDetector) Detect(text string) []Match {
	all := ssnPattern.FindAllStringSubmatchIndex(text, -1)
	if all == nil {
		return nil
	}
	matches := make([]Match, 0, len(all))
	for _, loc := range all {
		area := text[loc[2]:loc[3]]
		group := text[loc[4]:loc[5]]
		serial := text[loc[6]:loc[7]]
		if !isValidSSNParts(area, group, serial) {
			continue
		}
		matches = append(matches, Match{
			Type:  TypeSSN,
			Start: loc[0],
			End:   loc[1],
			Value: text[loc[0]:loc[1]],
		})
	}
	return matches
}

func isValidSSNParts(area, group, serial string) bool {
	areaNum, _ := strconv.Atoi(area)
	groupNum, _ := strconv.Atoi(group)
	serialNum, _ := strconv.Atoi(serial)

	if areaNum == 0 || areaNum == 666 || areaNum >= 900 {
		return false
	}
	if groupNum == 0 {
		return false
	}
	if serialNum == 0 {
		return false
	}
	return true
}
