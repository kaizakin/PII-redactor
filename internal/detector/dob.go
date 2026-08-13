package detector

import (
	"regexp"
	"strings"
	"time"
)

// numericDatePattern matches any of the common slash/dash separated date
// shapes (MM/DD/YYYY, DD-MM-YYYY, YYYY-MM-DD, ...). It does not try to
// disambiguate the field order itself — that's what the layout list in
// numericLayouts is for; a candidate is only accepted once one of those
// layouts actually parses it into a valid calendar date.
var numericDatePattern = regexp.MustCompile(`\b\d{1,4}[/-]\d{1,2}[/-]\d{1,4}\b`)

const monthNames = `January|February|March|April|May|June|July|August|September|October|November|December|` +
	`Jan|Feb|Mar|Apr|Jun|Jul|Aug|Sep|Sept|Oct|Nov|Dec`

// monthDayYearPattern matches "January 5, 1990" / "Jan 5 1990" style dates.
var monthDayYearPattern = regexp.MustCompile(`(?i)\b(?:` + monthNames + `)\.?\s+\d{1,2},?\s+\d{4}\b`)

// dayMonthYearPattern matches "5 January 1990" / "5 Jan, 1990" style dates.
var dayMonthYearPattern = regexp.MustCompile(`(?i)\b\d{1,2}\s+(?:` + monthNames + `)\.?,?\s+\d{4}\b`)

// numericLayouts covers slash- and dash-separated dates in both
// month-first (US) and year-first (ISO 8601) order. Go's reference layout
// digits without a leading zero (e.g. "1" for month) accept both padded
// and unpadded input, so each separator only needs one layout per field
// order.
var numericLayouts = []string{
	"1/2/2006",
	"1-2-2006",
	"2006-1-2",
	"2006/1/2",
}

var textualLayouts = []string{
	"January 2 2006",
	"Jan 2 2006",
	"2 January 2006",
	"2 Jan 2006",
}

// minBirthYear bounds how far back a plausible date of birth can go. This
// filters out numeric dates that parse fine but clearly aren't birth dates
// (e.g. a 1785 founding date in a company history blurb).
const minBirthYear = 1900

// DOBDetector finds dates of birth. A regex locates date-shaped candidates
// (both numeric and written-out month names); each candidate is then
// parsed against a set of concrete layouts and only kept if it resolves to
// a real calendar date within a plausible human lifetime. Precision comes
// from that parse step: "13/45/2020" or "February 30, 1990" look like
// dates but fail to parse as one and are correctly discarded.
type DOBDetector struct {
	now func() time.Time
}

func NewDOBDetector() *DOBDetector {
	return &DOBDetector{now: time.Now}
}

func (d *DOBDetector) Type() PIIType { return TypeDOB }

func (d *DOBDetector) Detect(text string) []Match {
	var matches []Match
	matches = append(matches, d.findDates(text, numericDatePattern, numericLayouts, false)...)
	matches = append(matches, d.findDates(text, monthDayYearPattern, textualLayouts, true)...)
	matches = append(matches, d.findDates(text, dayMonthYearPattern, textualLayouts, true)...)
	return matches
}

func (d *DOBDetector) findDates(text string, pattern *regexp.Regexp, layouts []string, stripPunctuation bool) []Match {
	idx := pattern.FindAllStringIndex(text, -1)
	if idx == nil {
		return nil
	}
	maxYear := d.now().Year()
	matches := make([]Match, 0, len(idx))
	for _, loc := range idx {
		candidate := text[loc[0]:loc[1]]
		parseable := candidate
		if stripPunctuation {
			parseable = strings.NewReplacer(".", "", ",", "").Replace(candidate)
			parseable = strings.Join(strings.Fields(parseable), " ")
		}
		t, ok := parseWithLayouts(parseable, layouts)
		if !ok || t.Year() < minBirthYear || t.Year() > maxYear {
			continue
		}
		matches = append(matches, Match{
			Type:  TypeDOB,
			Start: loc[0],
			End:   loc[1],
			Value: candidate,
		})
	}
	return matches
}

func parseWithLayouts(s string, layouts []string) (time.Time, bool) {
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
