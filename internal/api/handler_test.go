package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	docx "github.com/mmonterroca/docxgo"

	"github.com/kaizakin/PII-redactor/internal/detector"
	"github.com/kaizakin/PII-redactor/internal/processor"
)

func testHandler() *Handler {
	detectors := []detector.Detector{detector.NewEmailDetector(), detector.NewSSNDetector()}
	return NewHandler(detectors, processor.DefaultGenerators())
}

func TestHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	testHandler().Health(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRedactText(t *testing.T) {
	h := testHandler()

	t.Run("redacts detected PII and never echoes the original value", func(t *testing.T) {
		body, _ := json.Marshal(redactTextRequest{Text: "Contact jane@example.com about SSN 523-45-6789."})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/redact", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		h.RedactText(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var resp redactTextResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("invalid JSON response: %v", err)
		}
		if len(resp.Matches) != 2 {
			t.Fatalf("expected 2 matches, got %d: %+v", len(resp.Matches), resp.Matches)
		}
		if bytes.Contains([]byte(resp.RedactedText), []byte("jane@example.com")) {
			t.Errorf("redacted text still contains the original email: %q", resp.RedactedText)
		}
		if bytes.Contains([]byte(resp.RedactedText), []byte("523-45-6789")) {
			t.Errorf("redacted text still contains the original SSN: %q", resp.RedactedText)
		}
	})

	t.Run("rejects a missing text field", func(t *testing.T) {
		body, _ := json.Marshal(redactTextRequest{})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/redact", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		h.RedactText(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("rejects non-POST methods", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/redact", nil)
		rec := httptest.NewRecorder()
		h.RedactText(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", rec.Code)
		}
	})
}

func TestRedactDocx(t *testing.T) {
	h := testHandler()
	dir := t.TempDir()
	docPath := filepath.Join(dir, "input.docx")

	doc := docx.NewDocument()
	para, _ := doc.AddParagraph()
	run, _ := para.AddRun()
	if err := run.SetText("Reach jane@example.com for the offer letter."); err != nil {
		t.Fatalf("SetText: %v", err)
	}
	if err := doc.SaveAs(docPath); err != nil {
		t.Fatalf("SaveAs: %v", err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "input.docx")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	docBytes, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read docx: %v", err)
	}
	if _, err := part.Write(docBytes); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/redact/docx", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	h.RedactDocx(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
		t.Errorf("unexpected content type: %q", ct)
	}

	outPath := filepath.Join(dir, "output.docx")
	if err := os.WriteFile(outPath, rec.Body.Bytes(), 0o644); err != nil {
		t.Fatalf("write response body: %v", err)
	}
	reopened, err := docx.OpenDocument(outPath)
	if err != nil {
		t.Fatalf("OpenDocument: %v", err)
	}
	paras := reopened.Paragraphs()
	if len(paras) == 0 {
		t.Fatalf("expected at least 1 paragraph in redacted document")
	}
	if bytes.Contains([]byte(paras[0].Text()), []byte("jane@example.com")) {
		t.Errorf("redacted document still contains the original email: %q", paras[0].Text())
	}
}

func TestRedactDocxRejectsWrongExtension(t *testing.T) {
	h := testHandler()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "input.txt")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write([]byte("plain text")); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/redact/docx", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	h.RedactDocx(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}
