package detector

import "regexp"

// emailPattern is intentionally pragmatic rather than a full RFC 5322
// implementation: RFC-complete email regexes are famously unreadable and
// mostly match strings no real mail system would accept. This pattern
// covers the local-part/domain shapes seen in real-world PII (including
// +tags and subdomains) while staying auditable.
var emailPattern = regexp.MustCompile(`(?i)\b[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}\b`)

// EmailDetector finds email addresses using a regular expression. Email
// addresses are highly structured, so a well-scoped regex gives near-total
// precision without the cost or nondeterminism of an ML model.
type EmailDetector struct{}

func NewEmailDetector() *EmailDetector { return &EmailDetector{} }

func (d *EmailDetector) Type() PIIType { return TypeEmail }

func (d *EmailDetector) Detect(text string) []Match {
	idx := emailPattern.FindAllStringIndex(text, -1)
	if idx == nil {
		return nil
	}
	matches := make([]Match, 0, len(idx))
	for _, loc := range idx {
		matches = append(matches, Match{
			Type:  TypeEmail,
			Start: loc[0],
			End:   loc[1],
			Value: text[loc[0]:loc[1]],
		})
	}
	return matches
}
