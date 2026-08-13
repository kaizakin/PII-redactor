package detector

import (
	"context"
	"testing"

	"github.com/kaizakin/PII-redactor/internal/grpcclient"
)

type fakeNLPClient struct {
	entities []grpcclient.Entity
	err      error
}

func (f *fakeNLPClient) Analyze(ctx context.Context, text string) ([]grpcclient.Entity, error) {
	return f.entities, f.err
}

func (f *fakeNLPClient) Close() error { return nil }

func TestNLPDetector(t *testing.T) {
	t.Run("filters entities by label and maps to the configured PIIType", func(t *testing.T) {
		client := &fakeNLPClient{entities: []grpcclient.Entity{
			{Type: "PERSON", Start: 0, End: 11, Text: "Rashi Patil"},
			{Type: "ORG", Start: 20, End: 26, Text: "Acme Co"},
		}}
		d := NewNLPDetector(client, TypeName, "PERSON")
		matches := d.Detect("Rashi Patil works at Acme Co")
		if len(matches) != 1 {
			t.Fatalf("expected 1 match, got %d: %+v", len(matches), matches)
		}
		if matches[0].Value != "Rashi Patil" || matches[0].Type != TypeName {
			t.Errorf("unexpected match: %+v", matches[0])
		}
	})

	t.Run("NoOpClient produces no matches", func(t *testing.T) {
		d := NewNLPDetector(grpcclient.NoOpClient{}, TypeName, "PERSON")
		if matches := d.Detect("Rashi Patil works at Acme Co"); len(matches) != 0 {
			t.Errorf("expected no matches from NoOpClient, got %+v", matches)
		}
	})

	t.Run("client error yields no matches", func(t *testing.T) {
		client := &fakeNLPClient{err: context.DeadlineExceeded}
		d := NewNLPDetector(client, TypeName, "PERSON")
		if matches := d.Detect("irrelevant"); len(matches) != 0 {
			t.Errorf("expected no matches on error, got %+v", matches)
		}
	})
}
