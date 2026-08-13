// Package processor orchestrates PII detection and redaction. It is the
// one place that knows about both the Detector strategy pattern
// (internal/detector) and the deterministic replacement cache
// (internal/faker); neither of those packages knows about the other.
package processor

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/kaizakin/PII-redactor/internal/detector"
	"github.com/kaizakin/PII-redactor/internal/faker"
	"github.com/kaizakin/PII-redactor/internal/safe"
)

// redactedPlaceholder is used when a detector reports a PII type that has
// no registered faker.Generator. This should only happen if a new
// Detector is registered without a matching generator; failing safe to a
// visible placeholder is strictly better than leaving the original PII in
// the clear.
const redactedPlaceholder = "[REDACTED]"

// Replacement pairs a detected PII match with the fake value that will
// replace it in the output text.
type Replacement struct {
	Match       detector.Match
	Replacement string
}

// Processor runs every registered Detector over input text concurrently,
// resolves any overlapping matches, and replaces each surviving match with
// a deterministic fake value. Adding a new PII type requires no change
// here: register another Detector and, if it needs one, a generator.
type Processor struct {
	detectors  []detector.Detector
	cache      *faker.Cache
	generators map[detector.PIIType]faker.Generator
}

// New builds a Processor from a set of detectors, a shared replacement
// cache, and the PII-type-to-generator mapping used to fake each match.
func New(detectors []detector.Detector, cache *faker.Cache, generators map[detector.PIIType]faker.Generator) *Processor {
	return &Processor{detectors: detectors, cache: cache, generators: generators}
}

// Analyze runs detection and returns the resulting matches, each paired
// with the fake value it will be replaced by. Overlapping matches across
// detectors are resolved first, so the returned Replacements are sorted by
// Match.Start and guaranteed non-overlapping.
func (p *Processor) Analyze(text string) []Replacement {
	matches := resolveOverlaps(p.detectAll(text))
	replacements := make([]Replacement, 0, len(matches))
	for _, m := range matches {
		replacements = append(replacements, Replacement{
			Match:       m,
			Replacement: p.fake(m),
		})
	}
	return replacements
}

// Redact returns text with every detected PII match replaced by its
// deterministic fake value, along with the replacements that were applied.
func (p *Processor) Redact(text string) (string, []Replacement) {
	replacements := p.Analyze(text)
	return Apply(text, replacements), replacements
}

func (p *Processor) fake(m detector.Match) string {
	gen, ok := p.generators[m.Type]
	if !ok {
		return redactedPlaceholder
	}
	return p.cache.Get(m.Value, gen)
}

// detectAll runs every detector concurrently against text. Detectors are
// required to be safe for concurrent use (see detector.Detector), and each
// goroutine writes to its own slot, so no further synchronization is
// needed beyond the WaitGroup. A panicking detector is recovered and
// logged rather than left to crash the whole process — one bad input
// should fail that detector's contribution, not the entire server.
func (p *Processor) detectAll(text string) []detector.Match {
	results := make([][]detector.Match, len(p.detectors))
	var wg sync.WaitGroup
	for i, d := range p.detectors {
		wg.Add(1)
		go func(i int, d detector.Detector) {
			defer wg.Done()
			defer safe.Recover(fmt.Sprintf("processor.detectAll detector %s", d.Type()))
			results[i] = d.Detect(text)
		}(i, d)
	}
	wg.Wait()

	var all []detector.Match
	for _, r := range results {
		all = append(all, r...)
	}
	return all
}

// resolveOverlaps sorts matches by position and drops any match that
// overlaps one already accepted, preferring the earliest match at each
// position and, among ties, the longest. This lets detectors run
// independently without knowing about each other: if an SSN-shaped and a
// phone-shaped candidate both match the same digit run, exactly one wins
// instead of the output being corrupted by overlapping replacements.
func resolveOverlaps(matches []detector.Match) []detector.Match {
	if len(matches) == 0 {
		return matches
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Start != matches[j].Start {
			return matches[i].Start < matches[j].Start
		}
		return (matches[i].End - matches[i].Start) > (matches[j].End - matches[j].Start)
	})

	resolved := make([]detector.Match, 0, len(matches))
	lastEnd := -1
	for _, m := range matches {
		if m.Start < lastEnd {
			continue
		}
		resolved = append(resolved, m)
		lastEnd = m.End
	}
	return resolved
}

// Apply builds the redacted text by substituting each replacement's fake
// value for its match span in text. replacements must be sorted by
// Match.Start ascending and non-overlapping, which is guaranteed by
// Analyze's output.
func Apply(text string, replacements []Replacement) string {
	if len(replacements) == 0 {
		return text
	}
	var sb strings.Builder
	lastEnd := 0
	for _, r := range replacements {
		sb.WriteString(text[lastEnd:r.Match.Start])
		sb.WriteString(r.Replacement)
		lastEnd = r.Match.End
	}
	sb.WriteString(text[lastEnd:])
	return sb.String()
}
