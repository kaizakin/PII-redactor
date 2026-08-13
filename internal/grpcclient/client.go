// Package grpcclient is the contract between the Go engine and the Python
// NLP worker (python-worker/): unstructured text PII (Analyze) and PII
// embedded in images such as scanned IDs or screenshots (RedactImage).
package grpcclient

import "context"

// Entity is a single unstructured PII entity reported by the NLP worker.
// Start and End are byte offsets into the text that was analyzed.
type Entity struct {
	Type       string
	Start      int
	End        int
	Text       string
	Confidence float64
}

// NLPClient analyzes text for unstructured PII entities and redacts PII
// found inside images. Implementations must be safe for concurrent use,
// since the processor may invoke detectors — and docxio may redact
// multiple images — backed by the same client from multiple goroutines.
type NLPClient interface {
	Analyze(ctx context.Context, text string) ([]Entity, error)

	// RedactImage returns data re-encoded in format with every detected
	// PII region blacked out, plus how many regions were redacted. If no
	// PII is found, data is returned unchanged (redactions is 0).
	RedactImage(ctx context.Context, data []byte, format string) (redacted []byte, redactions int, err error)

	Close() error
}

// NoOpClient is the default NLPClient: it reports no text entities and
// passes images through unchanged. It exists so the engine runs
// correctly, end to end, without the Python worker running — structured
// PII (email, phone, SSN, credit card, IP, DOB) is still fully detected
// and redacted; only the unstructured and image-embedded categories are
// inactive until a real NLPClient is substituted.
type NoOpClient struct{}

func (NoOpClient) Analyze(ctx context.Context, text string) ([]Entity, error) { return nil, nil }

func (NoOpClient) RedactImage(ctx context.Context, data []byte, format string) ([]byte, int, error) {
	return data, 0, nil
}

func (NoOpClient) Close() error { return nil }
