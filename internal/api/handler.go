// Package api exposes the redaction engine over HTTP.
package api

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/kaizakin/PII-redactor/internal/detector"
	"github.com/kaizakin/PII-redactor/internal/docxio"
	"github.com/kaizakin/PII-redactor/internal/faker"
	"github.com/kaizakin/PII-redactor/internal/processor"
)

// maxUploadSize bounds how much of a multipart upload is buffered in
// memory before the rest spills to a temp file (handled internally by
// net/http); it is not a hard cap on file size.
const maxUploadSize = 32 << 20 // 32 MiB

// Handler wires HTTP requests to the redaction pipeline.
type Handler struct {
	detectors  []detector.Detector
	generators map[detector.PIIType]faker.Generator
}

// NewHandler builds a Handler from the active set of detectors and the
// PII-type-to-generator mapping used to fake matches.
func NewHandler(detectors []detector.Detector, generators map[detector.PIIType]faker.Generator) *Handler {
	return &Handler{detectors: detectors, generators: generators}
}

// newProcessor builds a fresh Processor with its own faker.Cache. A new
// cache per request keeps fake-value consistency scoped to a single
// document — "Rashi Patil" maps to the same fake name every time it
// appears in one request — without leaking that mapping into unrelated
// requests from other callers.
func (h *Handler) newProcessor() *processor.Processor {
	return processor.New(h.detectors, faker.NewCache(), h.generators)
}

// RedactDocx handles POST /api/v1/redact/docx: it accepts a multipart
// upload under the "file" field containing a .docx document, redacts PII
// in every paragraph and table cell, and streams the redacted document
// back as the response body.
func (h *Handler) RedactDocx(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		http.Error(w, "invalid multipart form", http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "form field \"file\" is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	if !strings.EqualFold(filepath.Ext(header.Filename), ".docx") {
		http.Error(w, "only .docx files are supported", http.StatusBadRequest)
		return
	}

	tmpDir, err := os.MkdirTemp("", "pii-redactor-*")
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tmpDir)

	inPath := filepath.Join(tmpDir, "input.docx")
	outPath := filepath.Join(tmpDir, "output.docx")

	if err := writeUploadToFile(file, inPath); err != nil {
		http.Error(w, "failed to read uploaded file", http.StatusInternalServerError)
		return
	}

	proc := h.newProcessor()
	redact := func(text string) (string, int) {
		redacted, replacements := proc.Redact(text)
		return redacted, len(replacements)
	}

	if _, err := docxio.RedactFile(inPath, outPath, redact); err != nil {
		http.Error(w, "failed to redact document", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	w.Header().Set("Content-Disposition", `attachment; filename="redacted.docx"`)
	http.ServeFile(w, r, outPath)
}

func writeUploadToFile(src io.Reader, dstPath string) error {
	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}
