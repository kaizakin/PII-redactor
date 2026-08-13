package detector

import (
	"net"
	"regexp"
)

// ipv4Candidate matches four dot-separated groups of 1-3 digits. It is
// deliberately loose (e.g. it would match "999.999.999.999") because the
// real validation is delegated to net.ParseIP below — this keeps precision
// high without hand-rolling range checks in the regex itself.
var ipv4Candidate = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)

// ipv6Candidate greedily grabs runs of hex digits and colons. It has no
// word-boundary anchors because "::1" legitimately starts with a non-word
// character; the character class itself defines where a candidate begins
// and ends. Actual validity (colon count, hex grouping, "::" compression
// rules) is left entirely to net.ParseIP.
var ipv6Candidate = regexp.MustCompile(`[0-9A-Fa-f:]{2,45}`)

// IPDetector finds IPv4 and IPv6 addresses. Candidates are located with a
// cheap regex and then validated with the standard library's net.ParseIP,
// which is the authoritative parser for what actually constitutes a valid
// IP address — far more reliable than encoding octet-range rules in regex.
type IPDetector struct{}

func NewIPDetector() *IPDetector { return &IPDetector{} }

func (d *IPDetector) Type() PIIType { return TypeIPAddress }

func (d *IPDetector) Detect(text string) []Match {
	var matches []Match
	matches = append(matches, findValidIPs(text, ipv4Candidate)...)
	matches = append(matches, findValidIPs(text, ipv6Candidate)...)
	return matches
}

func findValidIPs(text string, pattern *regexp.Regexp) []Match {
	idx := pattern.FindAllStringIndex(text, -1)
	if idx == nil {
		return nil
	}
	matches := make([]Match, 0, len(idx))
	for _, loc := range idx {
		candidate := text[loc[0]:loc[1]]
		if net.ParseIP(candidate) == nil {
			continue
		}
		matches = append(matches, Match{
			Type:  TypeIPAddress,
			Start: loc[0],
			End:   loc[1],
			Value: candidate,
		})
	}
	return matches
}
