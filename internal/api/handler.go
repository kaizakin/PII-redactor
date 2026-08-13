// Package api exposes the redaction engine over HTTP.
package api

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kaizakin/PII-redactor/internal/detector"
	"github.com/kaizakin/PII-redactor/internal/docxio"
	"github.com/kaizakin/PII-redactor/internal/faker"
	"github.com/kaizakin/PII-redactor/internal/grpcclient"
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
	nlpClient  grpcclient.NLPClient
}

// NewHandler builds a Handler from the active set of detectors, the
// PII-type-to-generator mapping used to fake text matches, and the NLP
// client used to redact PII embedded in images (word/media/* entries).
func NewHandler(detectors []detector.Detector, generators map[detector.PIIType]faker.Generator, nlpClient grpcclient.NLPClient) *Handler {
	return &Handler{detectors: detectors, generators: generators, nlpClient: nlpClient}
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
		httpError(w, http.StatusMethodNotAllowed, "method not allowed", "got %s, want POST", r.Method)
		return
	}

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		httpError(w, http.StatusBadRequest, "invalid multipart form", "ParseMultipartForm: %v", err)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpError(w, http.StatusBadRequest, "form field \"file\" is required", "FormFile: %v", err)
		return
	}
	defer file.Close()

	log.Printf("redact/docx: received %q (%d bytes)", header.Filename, header.Size)

	if !strings.EqualFold(filepath.Ext(header.Filename), ".docx") {
		httpError(w, http.StatusBadRequest, "only .docx files are supported", "rejected non-.docx upload %q", header.Filename)
		return
	}

	tmpDir, err := os.MkdirTemp("", "pii-redactor-*")
	if err != nil {
		httpError(w, http.StatusInternalServerError, "internal error", "MkdirTemp: %v", err)
		return
	}
	defer os.RemoveAll(tmpDir)

	inPath := filepath.Join(tmpDir, "input.docx")
	outPath := filepath.Join(tmpDir, "output.docx")

	if err := writeUploadToFile(file, inPath); err != nil {
		httpError(w, http.StatusInternalServerError, "failed to read uploaded file", "writeUploadToFile: %v", err)
		return
	}

	log.Printf("redact/docx: %q saved, starting redaction", header.Filename)
	start := time.Now()

	proc := h.newProcessor()
	textRedact := func(text string) (string, int) {
		redacted, replacements := proc.Redact(text)
		return redacted, len(replacements)
	}
	imageRedact := func(data []byte, format string) ([]byte, int) {
		redacted, count, err := h.nlpClient.RedactImage(r.Context(), data, format)
		if err != nil {
			log.Printf("redact/docx: %q: image redaction failed, leaving image unchanged: %v", header.Filename, err)
			return data, 0
		}
		return redacted, count
	}

	textMatches, imageRegions, err := docxio.RedactDocument(inPath, outPath, textRedact, imageRedact)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "failed to redact document", "docxio.RedactDocument(%q): %v", header.Filename, err)
		return
	}

	log.Printf("redact/docx: %q done — %d text matches, %d image regions redacted in %s",
		header.Filename, textMatches, imageRegions, time.Since(start))

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	w.Header().Set("Content-Disposition", `attachment; filename="redacted.docx"`)
	http.ServeFile(w, r, outPath)
}

// httpError logs the real, detailed error server-side (format+args) and
// sends the client a generic message — so nothing that goes wrong is
// silently invisible in the logs, without leaking internals in the
// response body.
func httpError(w http.ResponseWriter, status int, clientMsg, logFormat string, logArgs ...any) {
	log.Printf("redact/docx: "+logFormat, logArgs...)
	http.Error(w, clientMsg, status)
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
