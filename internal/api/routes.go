package api

import "net/http"

// NewRouter builds the HTTP routing table for the redaction engine.
func NewRouter(h *Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", h.Health)
	mux.HandleFunc("/api/v1/redact", h.RedactText)
	mux.HandleFunc("/api/v1/redact/docx", h.RedactDocx)
	return mux
}
