package api

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	docx "github.com/mmonterroca/docxgo"

	"github.com/kaizakin/PII-redactor/internal/detector"
	"github.com/kaizakin/PII-redactor/internal/docxio"
	"github.com/kaizakin/PII-redactor/internal/grpcclient"
	"github.com/kaizakin/PII-redactor/internal/processor"
)

func testHandler() *Handler {
	detectors := []detector.Detector{detector.NewEmailDetector(), detector.NewSSNDetector()}
	return NewHandler(detectors, processor.DefaultGenerators(), grpcclient.NoOpClient{})
}

// fakeImageRedactingClient marks every image as containing one PII region
// by appending a byte, without doing any real OCR — enough to prove the
// handler wires image bytes through to the NLP client and applies the
// result.
type fakeImageRedactingClient struct{ grpcclient.NoOpClient }

func (fakeImageRedactingClient) RedactImage(ctx context.Context, data []byte, format string) ([]byte, int, error) {
	return append(append([]byte(nil), data...), '!'), 1, nil
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

func TestRedactDocxRedactsEmbeddedImages(t *testing.T) {
	h := NewHandler(
		[]detector.Detector{detector.NewEmailDetector()},
		processor.DefaultGenerators(),
		fakeImageRedactingClient{},
	)
	dir := t.TempDir()
	docPath := filepath.Join(dir, "input.docx")

	imgPath := filepath.Join(dir, "src.png")
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := range 10 {
		for x := range 10 {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	var imgBuf bytes.Buffer
	if err := png.Encode(&imgBuf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	if err := os.WriteFile(imgPath, imgBuf.Bytes(), 0o644); err != nil {
		t.Fatalf("write source image: %v", err)
	}

	doc := docx.NewDocument()
	para, _ := doc.AddParagraph()
	if _, err := para.AddImage(imgPath); err != nil {
		t.Fatalf("AddImage: %v", err)
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

	req := httptest.NewRequest(http.MethodPost, "/redact/docx", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	h.RedactDocx(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	outPath := filepath.Join(dir, "output.docx")
	if err := os.WriteFile(outPath, rec.Body.Bytes(), 0o644); err != nil {
		t.Fatalf("write response body: %v", err)
	}

	images, err := docxio.ExtractImages(outPath)
	if err != nil {
		t.Fatalf("ExtractImages: %v", err)
	}
	if len(images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(images))
	}
	if !bytes.Equal(images[0].Data, append(append([]byte(nil), imgBuf.Bytes()...), '!')) {
		t.Errorf("expected the image to carry the fake client's redacted bytes")
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
