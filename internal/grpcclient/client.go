// Package grpcclient defines the contract between the Go engine and the
// Python NLP worker responsible for unstructured PII (person names,
// company names, physical addresses). The worker itself and its generated
// protobuf stubs are a later phase of this project (see proto/redactor.proto);
// this package lets the rest of the engine — NLPDetector, and the wiring in
// cmd/server/main.go — be built, registered, and tested today against
// NoOpClient, then pointed at a real gRPC-backed implementation later
// without any other file changing.
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

// NLPClient analyzes text for unstructured PII entities. Implementations
// must be safe for concurrent use, since the processor may invoke
// detectors backed by the same client from multiple goroutines.
type NLPClient interface {
	Analyze(ctx context.Context, text string) ([]Entity, error)
	Close() error
}

// NoOpClient is the default NLPClient: it reports no entities. It exists so
// the engine runs correctly, end to end, before the Python worker exists —
// structured PII (email, phone, SSN, credit card, IP, DOB) is still fully
// detected and redacted; only the unstructured categories are inactive
// until a real NLPClient is substituted.
type NoOpClient struct{}

func (NoOpClient) Analyze(ctx context.Context, text string) ([]Entity, error) { return nil, nil }

func (NoOpClient) Close() error { return nil }
