package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithRecoveryConvertsPanicToServerError(t *testing.T) {
	panicky := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("simulated handler failure")
	})

	req := httptest.NewRequest(http.MethodGet, "/redact/docx", nil)
	rec := httptest.NewRecorder()

	// The call itself must return normally, not crash the test process.
	withRecovery(panicky).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestWithRecoveryLetsHealthyRequestsThrough(t *testing.T) {
	healthy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/redact/docx", nil)
	rec := httptest.NewRecorder()

	withRecovery(healthy).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestNewRouterSurvivesAPanickingRequest(t *testing.T) {
	h := testHandler()
	router := NewRouter(h)

	// A GET to /redact/docx (which only accepts POST) exercises the real
	// router through withRequestLogging(withRecovery(mux)) end to end,
	// confirming the composition itself doesn't crash on an ordinary
	// error path; the panic-specific behavior is covered directly above
	// since RedactDocx itself has no intentional panic to trigger here.
	req := httptest.NewRequest(http.MethodGet, "/redact/docx", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}
