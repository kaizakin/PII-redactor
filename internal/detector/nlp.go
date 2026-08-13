package detector

import (
	"context"

	"github.com/kaizakin/PII-redactor/internal/grpcclient"
)

// NLPDetector adapts a grpcclient.NLPClient to the Detector interface, so
// unstructured PII (names, company names, physical addresses) slots into
// the exact same detection pipeline as the structured, Go-native
// detectors — the processor never knows or cares that this Detect call
// crosses a process boundary. entityLabel selects which entity type
// reported by the NLP worker this instance is responsible for (e.g.
// "PERSON"), since one worker call can return multiple entity types at
// once.
//
// Registered today with grpcclient.NoOpClient, this detector simply
// contributes zero matches. The moment a real NLPClient (backed by the
// Python Presidio worker) is substituted in cmd/server/main.go, it becomes
// live — no other part of the engine changes.
type NLPDetector struct {
	client      grpcclient.NLPClient
	piiType     PIIType
	entityLabel string
}

func NewNLPDetector(client grpcclient.NLPClient, piiType PIIType, entityLabel string) *NLPDetector {
	return &NLPDetector{client: client, piiType: piiType, entityLabel: entityLabel}
}

func (d *NLPDetector) Type() PIIType { return d.piiType }

func (d *NLPDetector) Detect(text string) []Match {
	entities, err := d.client.Analyze(context.Background(), text)
	if err != nil || len(entities) == 0 {
		return nil
	}
	matches := make([]Match, 0, len(entities))
	for _, e := range entities {
		if e.Type != d.entityLabel {
			continue
		}
		matches = append(matches, Match{
			Type:  d.piiType,
			Start: e.Start,
			End:   e.End,
			Value: e.Text,
		})
	}
	return matches
}
