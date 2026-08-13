package docxio

import (
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
