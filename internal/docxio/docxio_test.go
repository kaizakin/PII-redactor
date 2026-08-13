package docxio

import (
	"bytes"
	"image/color"
	"path/filepath"
	"strings"
	"testing"

	docx "github.com/mmonterroca/docxgo"
)

// fakeRedact stands in for the real processor: it replaces every
// occurrence of "SECRET" with "REDACTED" and reports the count, without
// depending on the actual detection pipeline.
func fakeRedact(text string) (string, int) {
	count := strings.Count(text, "SECRET")
	if count == 0 {
		return text, 0
	}
	return strings.ReplaceAll(text, "SECRET", "REDACTED"), count
}

func TestRedactFile(t *testing.T) {
	dir := t.TempDir()
	inPath := filepath.Join(dir, "input.docx")
	outPath := filepath.Join(dir, "output.docx")

	doc := docx.NewDocument()

	para, err := doc.AddParagraph()
	if err != nil {
		t.Fatalf("AddParagraph: %v", err)
	}
	run, err := para.AddRun()
	if err != nil {
		t.Fatalf("AddRun: %v", err)
	}
	if err := run.SetText("Contact SECRET for details."); err != nil {
		t.Fatalf("SetText: %v", err)
	}

	clean, err := doc.AddParagraph()
	if err != nil {
		t.Fatalf("AddParagraph: %v", err)
	}
	cleanRun, err := clean.AddRun()
	if err != nil {
		t.Fatalf("AddRun: %v", err)
	}
	if err := cleanRun.SetText("This paragraph has no PII."); err != nil {
		t.Fatalf("SetText: %v", err)
	}

	table, err := doc.AddTable(1, 1)
	if err != nil {
		t.Fatalf("AddTable: %v", err)
	}
	row, err := table.Row(0)
	if err != nil {
		t.Fatalf("Row: %v", err)
	}
	cell, err := row.Cell(0)
	if err != nil {
		t.Fatalf("Cell: %v", err)
	}
	cellPara, err := cell.AddParagraph()
	if err != nil {
		t.Fatalf("cell AddParagraph: %v", err)
	}
	cellRun, err := cellPara.AddRun()
	if err != nil {
		t.Fatalf("cell AddRun: %v", err)
	}
	if err := cellRun.SetText("Table cell holds SECRET too."); err != nil {
		t.Fatalf("SetText: %v", err)
	}

	if err := doc.SaveAs(inPath); err != nil {
		t.Fatalf("SaveAs: %v", err)
	}

	total, err := RedactFile(inPath, outPath, fakeRedact)
	if err != nil {
		t.Fatalf("RedactFile: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected 2 redactions across paragraph + table, got %d", total)
	}

	reopened, err := docx.OpenDocument(outPath)
	if err != nil {
		t.Fatalf("OpenDocument: %v", err)
	}

	paras := reopened.Paragraphs()
	if len(paras) < 2 {
		t.Fatalf("expected at least 2 paragraphs, got %d", len(paras))
	}
	if got := paras[0].Text(); got != "Contact REDACTED for details." {
		t.Errorf("unexpected first paragraph text: %q", got)
	}
	if got := paras[1].Text(); got != "This paragraph has no PII." {
		t.Errorf("expected untouched paragraph to survive unchanged, got %q", got)
	}

	tables := reopened.Tables()
	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}
	reRow, err := tables[0].Row(0)
	if err != nil {
		t.Fatalf("Row: %v", err)
	}
	reCell, err := reRow.Cell(0)
	if err != nil {
		t.Fatalf("Cell: %v", err)
	}
	cellParas := reCell.Paragraphs()
	if len(cellParas) != 1 {
		t.Fatalf("expected 1 paragraph in table cell, got %d", len(cellParas))
	}
	if got := cellParas[0].Text(); got != "Table cell holds REDACTED too." {
		t.Errorf("unexpected table cell text: %q", got)
	}
}

func TestRedactDocument(t *testing.T) {
	dir := t.TempDir()
	inPath := filepath.Join(dir, "input.docx")
	outPath := filepath.Join(dir, "output.docx")

	doc := docx.NewDocument()
	para, err := doc.AddParagraph()
	if err != nil {
		t.Fatalf("AddParagraph: %v", err)
	}
	run, err := para.AddRun()
	if err != nil {
		t.Fatalf("AddRun: %v", err)
	}
	if err := run.SetText("Contact SECRET for details."); err != nil {
		t.Fatalf("SetText: %v", err)
	}

	imgPara, err := doc.AddParagraph()
	if err != nil {
		t.Fatalf("AddParagraph: %v", err)
	}
	original := writeTestPNG(t, filepath.Join(dir, "src.png"), color.RGBA{R: 255, A: 255})
	if _, err := imgPara.AddImage(filepath.Join(dir, "src.png")); err != nil {
		t.Fatalf("AddImage: %v", err)
	}

	if err := doc.SaveAs(inPath); err != nil {
		t.Fatalf("SaveAs: %v", err)
	}

	imageRedact := func(data []byte, format string) ([]byte, int) {
		return append(append([]byte(nil), data...), '!'), 5
	}

	textMatches, imageRegions, err := RedactDocument(inPath, outPath, fakeRedact, imageRedact)
	if err != nil {
		t.Fatalf("RedactDocument: %v", err)
	}
	if textMatches != 1 {
		t.Errorf("expected 1 text match, got %d", textMatches)
	}
	if imageRegions != 5 {
		t.Errorf("expected 5 image regions, got %d", imageRegions)
	}

	reopened, err := docx.OpenDocument(outPath)
	if err != nil {
		t.Fatalf("OpenDocument: %v", err)
	}
	if got := reopened.Paragraphs()[0].Text(); got != "Contact REDACTED for details." {
		t.Errorf("unexpected paragraph text: %q", got)
	}

	images, err := ExtractImages(outPath)
	if err != nil {
		t.Fatalf("ExtractImages: %v", err)
	}
	if len(images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(images))
	}
	if !bytes.Equal(images[0].Data, append(append([]byte(nil), original...), '!')) {
		t.Errorf("expected the image to carry the redacted bytes")
	}
}

func TestRedactFileSurvivesAPanickingRedactFunc(t *testing.T) {
	dir := t.TempDir()
	inPath := filepath.Join(dir, "input.docx")
	outPath := filepath.Join(dir, "output.docx")

	doc := docx.NewDocument()
	p1, _ := doc.AddParagraph()
	r1, _ := p1.AddRun()
	if err := r1.SetText("BOOM"); err != nil {
		t.Fatalf("SetText: %v", err)
	}
	p2, _ := doc.AddParagraph()
	r2, _ := p2.AddRun()
	if err := r2.SetText("Contact SECRET for details."); err != nil {
		t.Fatalf("SetText: %v", err)
	}
	if err := doc.SaveAs(inPath); err != nil {
		t.Fatalf("SaveAs: %v", err)
	}

	panickyRedact := func(text string) (string, int) {
		if text == "BOOM" {
			panic("simulated redact failure")
		}
		return fakeRedact(text)
	}

	// The call itself must return normally, not crash the test process,
	// and the non-panicking run must still get redacted correctly.
	total, err := RedactFile(inPath, outPath, panickyRedact)
	if err != nil {
		t.Fatalf("RedactFile: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected 1 redaction from the surviving run, got %d", total)
	}

	reopened, err := docx.OpenDocument(outPath)
	if err != nil {
		t.Fatalf("OpenDocument: %v", err)
	}
	paras := reopened.Paragraphs()
	if paras[0].Text() != "BOOM" {
		t.Errorf("expected the panicking run's original text preserved, got %q", paras[0].Text())
	}
	if paras[1].Text() != "Contact REDACTED for details." {
		t.Errorf("expected the surviving run redacted, got %q", paras[1].Text())
	}
}

func TestRedactFileNoMatches(t *testing.T) {
	dir := t.TempDir()
	inPath := filepath.Join(dir, "input.docx")
	outPath := filepath.Join(dir, "output.docx")

	doc := docx.NewDocument()
	para, _ := doc.AddParagraph()
	run, _ := para.AddRun()
	if err := run.SetText("Nothing sensitive here."); err != nil {
		t.Fatalf("SetText: %v", err)
	}
	if err := doc.SaveAs(inPath); err != nil {
		t.Fatalf("SaveAs: %v", err)
	}

	total, err := RedactFile(inPath, outPath, fakeRedact)
	if err != nil {
		t.Fatalf("RedactFile: %v", err)
	}
	if total != 0 {
		t.Errorf("expected 0 redactions, got %d", total)
	}
}
