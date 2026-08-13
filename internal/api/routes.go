package api

import "net/http"

// NewRouter builds the HTTP routing table for the redaction engine.
func NewRouter(h *Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/redact/docx", h.RedactDocx)
	return mux
}
