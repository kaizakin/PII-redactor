package processor

import (
	"strings"
	"testing"

	"github.com/kaizakin/PII-redactor/internal/detector"
	"github.com/kaizakin/PII-redactor/internal/faker"
)

// fakeDetector returns a fixed set of matches regardless of input, letting
// tests exercise the processor's merge/overlap/replace logic without
// depending on real detector behavior.
type fakeDetector struct {
	piiType detector.PIIType
	matches []detector.Match
}

func (f *fakeDetector) Type() detector.PIIType         { return f.piiType }
func (f *fakeDetector) Detect(string) []detector.Match { return f.matches }

func TestProcessorRedact(t *testing.T) {
	t.Run("replaces every detected match", func(t *testing.T) {
		text := "Email jane@example.com and phone 555-0100."
		detectors := []detector.Detector{detector.NewEmailDetector()}
		p := New(detectors, faker.NewCache(), map[detector.PIIType]faker.Generator{
			detector.TypeEmail: faker.Email,
		})

		redacted, replacements := p.Redact(text)
		if len(replacements) != 1 {
			t.Fatalf("expected 1 replacement, got %d: %+v", len(replacements), replacements)
		}
		if replacements[0].Match.Value != "jane@example.com" {
			t.Errorf("unexpected matched value: %q", replacements[0].Match.Value)
		}
		if redacted == text {
			t.Errorf("expected redacted text to differ from input")
		}
		if want := replacements[0].Replacement; !strings.Contains(redacted, want) {
			t.Errorf("expected redacted text %q to contain fake value %q", redacted, want)
		}
	})

	t.Run("same PII value redacts identically every time it appears", func(t *testing.T) {
		text := "jane@example.com sent a copy to jane@example.com."
		detectors := []detector.Detector{detector.NewEmailDetector()}
		p := New(detectors, faker.NewCache(), map[detector.PIIType]faker.Generator{
			detector.TypeEmail: faker.Email,
		})

		_, replacements := p.Redact(text)
		if len(replacements) != 2 {
			t.Fatalf("expected 2 replacements, got %d: %+v", len(replacements), replacements)
		}
		if replacements[0].Replacement != replacements[1].Replacement {
			t.Errorf("expected identical fake values, got %q and %q", replacements[0].Replacement, replacements[1].Replacement)
		}
	})

	t.Run("unregistered PII type falls back to a visible placeholder", func(t *testing.T) {
		detectors := []detector.Detector{
			&fakeDetector{piiType: detector.TypeName, matches: []detector.Match{
				{Type: detector.TypeName, Start: 0, End: 5, Value: "Rashi"},
			}},
		}
		p := New(detectors, faker.NewCache(), map[detector.PIIType]faker.Generator{})
		redacted, _ := p.Redact("Rashi works here")
		if redacted != "[REDACTED] works here" {
			t.Errorf("unexpected redacted text: %q", redacted)
		}
	})

	t.Run("overlapping matches keep only the earliest, longest span", func(t *testing.T) {
		// Two detectors disagree about a 9-char span starting at 0: one
		// reports the full span, the other a shorter prefix of it.
		detectors := []detector.Detector{
			&fakeDetector{piiType: detector.TypeSSN, matches: []detector.Match{
				{Type: detector.TypeSSN, Start: 0, End: 11, Value: "523-45-6789"},
			}},
			&fakeDetector{piiType: detector.TypePhone, matches: []detector.Match{
				{Type: detector.TypePhone, Start: 0, End: 7, Value: "523-45-"},
			}},
		}
		p := New(detectors, faker.NewCache(), map[detector.PIIType]faker.Generator{
			detector.TypeSSN:   faker.SSN,
			detector.TypePhone: faker.Phone,
		})
		replacements := p.Analyze("523-45-6789 on file")
		if len(replacements) != 1 {
			t.Fatalf("expected exactly 1 surviving match, got %d: %+v", len(replacements), replacements)
		}
		if replacements[0].Match.Type != detector.TypeSSN {
			t.Errorf("expected the longer SSN match to win, got %s", replacements[0].Match.Type)
		}
	})
}

func TestApply(t *testing.T) {
	text := "call 555-0100 now"
	replacements := []Replacement{
		{Match: detector.Match{Start: 5, End: 13}, Replacement: "XXX-XXXX"},
	}
	got := Apply(text, replacements)
	want := "call XXX-XXXX now"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDefaultGenerators(t *testing.T) {
	gens := DefaultGenerators()
	for _, typ := range []detector.PIIType{
		detector.TypeEmail, detector.TypePhone, detector.TypeSSN,
		detector.TypeCreditCard, detector.TypeIPAddress, detector.TypeDOB,
		detector.TypeName, detector.TypeCompany, detector.TypeAddress,
	} {
		if _, ok := gens[typ]; !ok {
			t.Errorf("expected a default generator registered for %s", typ)
		}
	}
}
