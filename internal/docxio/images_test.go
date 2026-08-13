package docxio

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	docx "github.com/mmonterroca/docxgo"
)

// writeTestPNG writes a solid-color square PNG to path, returning its
// encoded bytes for comparison.
func writeTestPNG(t *testing.T, path string, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	for y := range 20 {
		for x := range 20 {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return buf.Bytes()
}

func buildDocWithImage(t *testing.T, docPath string, imgBytes []byte) {
	t.Helper()
	doc := docx.NewDocument()
	para, err := doc.AddParagraph()
	if err != nil {
		t.Fatalf("AddParagraph: %v", err)
	}

	dir := t.TempDir()
	imgPath := filepath.Join(dir, "original.png")
	if err := os.WriteFile(imgPath, imgBytes, 0o644); err != nil {
		t.Fatalf("write source image: %v", err)
	}

	if _, err := para.AddImage(imgPath); err != nil {
		t.Fatalf("AddImage: %v", err)
	}
	if err := doc.SaveAs(docPath); err != nil {
		t.Fatalf("SaveAs: %v", err)
	}
}

func TestExtractImages(t *testing.T) {
	dir := t.TempDir()
	docPath := filepath.Join(dir, "input.docx")

	original := writeTestPNG(t, filepath.Join(dir, "src.png"), color.RGBA{R: 255, A: 255})
	buildDocWithImage(t, docPath, original)

	images, err := ExtractImages(docPath)
	if err != nil {
		t.Fatalf("ExtractImages: %v", err)
	}
	if len(images) != 1 {
		t.Fatalf("expected 1 embedded image, got %d", len(images))
	}
	img := images[0]
	if img.Format() != "png" {
		t.Errorf("expected format png, got %q", img.Format())
	}
	if !bytes.Equal(img.Data, original) {
		t.Errorf("extracted image bytes do not match the original")
	}
}

func TestExtractImagesNoImages(t *testing.T) {
	dir := t.TempDir()
	docPath := filepath.Join(dir, "input.docx")

	doc := docx.NewDocument()
	para, _ := doc.AddParagraph()
	run, _ := para.AddRun()
	if err := run.SetText("no images here"); err != nil {
		t.Fatalf("SetText: %v", err)
	}
	if err := doc.SaveAs(docPath); err != nil {
		t.Fatalf("SaveAs: %v", err)
	}

	images, err := ExtractImages(docPath)
	if err != nil {
		t.Fatalf("ExtractImages: %v", err)
	}
	if len(images) != 0 {
		t.Errorf("expected no images, got %d", len(images))
	}
}

func TestReplaceImages(t *testing.T) {
	dir := t.TempDir()
	inPath := filepath.Join(dir, "input.docx")
	outPath := filepath.Join(dir, "output.docx")

	original := writeTestPNG(t, filepath.Join(dir, "src.png"), color.RGBA{R: 255, A: 255})
	buildDocWithImage(t, inPath, original)

	before, err := ExtractImages(inPath)
	if err != nil {
		t.Fatalf("ExtractImages: %v", err)
	}
	if len(before) != 1 {
		t.Fatalf("expected 1 image, got %d", len(before))
	}

	replacement := writeTestPNG(t, filepath.Join(dir, "replacement.png"), color.RGBA{B: 255, A: 255})
	if err := ReplaceImages(inPath, outPath, map[string][]byte{before[0].Name: replacement}); err != nil {
		t.Fatalf("ReplaceImages: %v", err)
	}

	// The document must still be a valid, openable .docx after the ZIP
	// surgery — everything except the one media entry should have passed
	// through byte-for-byte.
	reopened, err := docx.OpenDocument(outPath)
	if err != nil {
		t.Fatalf("OpenDocument on output: %v", err)
	}
	if len(reopened.Paragraphs()) == 0 {
		t.Fatalf("expected the reopened document to still have its paragraph")
	}

	after, err := ExtractImages(outPath)
	if err != nil {
		t.Fatalf("ExtractImages on output: %v", err)
	}
	if len(after) != 1 {
		t.Fatalf("expected 1 image in output, got %d", len(after))
	}
	if after[0].Name != before[0].Name {
		t.Errorf("expected the same entry name %q, got %q", before[0].Name, after[0].Name)
	}
	if !bytes.Equal(after[0].Data, replacement) {
		t.Errorf("expected the replaced image bytes, got something else")
	}
	if bytes.Equal(after[0].Data, original) {
		t.Errorf("image was not actually replaced")
	}
}

func TestReplaceImagesNoReplacements(t *testing.T) {
	dir := t.TempDir()
	inPath := filepath.Join(dir, "input.docx")
	outPath := filepath.Join(dir, "output.docx")

	original := writeTestPNG(t, filepath.Join(dir, "src.png"), color.RGBA{G: 255, A: 255})
	buildDocWithImage(t, inPath, original)

	if err := ReplaceImages(inPath, outPath, map[string][]byte{}); err != nil {
		t.Fatalf("ReplaceImages: %v", err)
	}

	images, err := ExtractImages(outPath)
	if err != nil {
		t.Fatalf("ExtractImages: %v", err)
	}
	if len(images) != 1 || !bytes.Equal(images[0].Data, original) {
		t.Errorf("expected the original image untouched when no replacements given")
	}
}

func TestRedactImagesInFile(t *testing.T) {
	dir := t.TempDir()
	inPath := filepath.Join(dir, "input.docx")
	outPath := filepath.Join(dir, "output.docx")

	original := writeTestPNG(t, filepath.Join(dir, "src.png"), color.RGBA{R: 255, A: 255})
	buildDocWithImage(t, inPath, original)

	fakeRedact := func(data []byte, format string) ([]byte, int) {
		if format != "png" {
			t.Errorf("expected format png, got %q", format)
		}
		return append(append([]byte(nil), data...), '!'), 2
	}

	total, err := RedactImagesInFile(inPath, outPath, fakeRedact)
	if err != nil {
		t.Fatalf("RedactImagesInFile: %v", err)
	}
	if total != 2 {
		t.Errorf("expected 2 regions redacted, got %d", total)
	}

	images, err := ExtractImages(outPath)
	if err != nil {
		t.Fatalf("ExtractImages: %v", err)
	}
	if len(images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(images))
	}
	if !bytes.Equal(images[0].Data, append(append([]byte(nil), original...), '!')) {
		t.Errorf("expected the redacted image bytes to be applied")
	}
}

func TestRedactImagesInFileNoRedactions(t *testing.T) {
	dir := t.TempDir()
	inPath := filepath.Join(dir, "input.docx")
	outPath := filepath.Join(dir, "output.docx")

	original := writeTestPNG(t, filepath.Join(dir, "src.png"), color.RGBA{B: 255, A: 255})
	buildDocWithImage(t, inPath, original)

	noopRedact := func(data []byte, format string) ([]byte, int) { return data, 0 }

	total, err := RedactImagesInFile(inPath, outPath, noopRedact)
	if err != nil {
		t.Fatalf("RedactImagesInFile: %v", err)
	}
	if total != 0 {
		t.Errorf("expected 0 regions redacted, got %d", total)
	}

	images, err := ExtractImages(outPath)
	if err != nil {
		t.Fatalf("ExtractImages: %v", err)
	}
	if len(images) != 1 || !bytes.Equal(images[0].Data, original) {
		t.Errorf("expected the original image unchanged")
	}
}

func TestRedactImagesInFileNoImages(t *testing.T) {
	dir := t.TempDir()
	inPath := filepath.Join(dir, "input.docx")
	outPath := filepath.Join(dir, "output.docx")

	doc := docx.NewDocument()
	para, _ := doc.AddParagraph()
	run, _ := para.AddRun()
	if err := run.SetText("no images here"); err != nil {
		t.Fatalf("SetText: %v", err)
	}
	if err := doc.SaveAs(inPath); err != nil {
		t.Fatalf("SaveAs: %v", err)
	}

	called := false
	total, err := RedactImagesInFile(inPath, outPath, func(data []byte, format string) ([]byte, int) {
		called = true
		return data, 0
	})
	if err != nil {
		t.Fatalf("RedactImagesInFile: %v", err)
	}
	if total != 0 {
		t.Errorf("expected 0 regions redacted, got %d", total)
	}
	if called {
		t.Errorf("expected redact func not to be called when there are no images")
	}

	reopened, err := docx.OpenDocument(outPath)
	if err != nil {
		t.Fatalf("OpenDocument: %v", err)
	}
	if got := reopened.Paragraphs()[0].Text(); got != "no images here" {
		t.Errorf("unexpected paragraph text: %q", got)
	}
}
