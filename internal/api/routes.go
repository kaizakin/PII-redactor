package api

import (
	"log"
	"net/http"
	"time"
)

// NewRouter builds the HTTP routing table for the redaction engine.
func NewRouter(h *Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/redact/docx", h.RedactDocx)
	return withRequestLogging(mux)
}

// statusRecorder captures the status code a handler writes, since
// http.ResponseWriter doesn't expose it after the fact.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// withRequestLogging logs every request that hits the server: method,
// path, remote address, the status code the handler responded with, and
// how long it took. This is the first thing to check when it's unclear
// whether a request even arrived.
func withRequestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		log.Printf("http: <- %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
		next.ServeHTTP(rec, r)
		log.Printf("http: -> %s %s status=%d took=%s", r.Method, r.URL.Path, rec.status, time.Since(start))
	})
}
