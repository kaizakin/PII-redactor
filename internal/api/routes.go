package api

import (
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

// NewRouter builds the HTTP routing table for the redaction engine.
func NewRouter(h *Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/redact/docx", h.RedactDocx)
	return withCORS(withRequestLogging(withRecovery(mux)))
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, Authorization")
		if r.Method == "OPTIONS" {
			return
		}
		next.ServeHTTP(w, r)
	})
}

// withRecovery converts a panic reaching the request's main goroutine into
// a clean 500 response instead of an abruptly reset connection — net/http
// itself recovers such panics, but only by closing the connection with no
// HTTP response at all, which the client just sees as a broken connection.
// (Panics in goroutines *spawned by* a handler — concurrent detection,
// concurrent image/run redaction — are a separate problem this doesn't
// cover; see internal/safe.Recover, used at each of those call sites.)
func withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("http: recovered from panic handling %s %s: %v\n%s", r.Method, r.URL.Path, rec, debug.Stack())
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
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
