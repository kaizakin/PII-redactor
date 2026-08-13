package docxio

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	docx "github.com/mmonterroca/docxgo"
	"github.com/mmonterroca/docxgo/domain"
)

// injectParagraphShading rewrites a .docx's document.xml to add a red
// paragraph-background shading (<w:shd>) to its first paragraph,
// simulating a real Word feature that docxgo's domain model has no field
// for at all (there is no Paragraph.SetShading, and no corresponding
// property is parsed back out on read either). This is the concrete,
// confirmed case where an earlier docxgo-object-model-based
// implementation of RedactFile silently dropped formatting on ANY
// read+write round trip, independent of whether redaction touched that
// paragraph — the bug this file's tests guard against regressing.
func injectParagraphShading(t *testing.T, path string) {
	t.Helper()

	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open %q as zip: %v", path, err)
	}
	var docXML []byte
	entries := map[string][]byte{}
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open entry %q: %v", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("read entry %q: %v", f.Name, err)
		}
		if f.Name == "word/document.xml" {
			docXML = data
		}
		entries[f.Name] = data
	}
	r.Close()

	if docXML == nil {
		t.Fatal("word/document.xml not found in fixture")
	}
	before := string(docXML)
	after := strings.Replace(before, "<w:pPr></w:pPr>", `<w:pPr><w:shd w:val="clear" w:color="auto" w:fill="FF0000"/></w:pPr>`, 1)
	if after == before {
		t.Fatal("expected an empty <w:pPr></w:pPr> to inject shading into — fixture shape changed?")
	}
	entries["word/document.xml"] = []byte(after)

	out, err := os.Create(path)
	if err != nil {
		t.Fatalf("recreate %q: %v", path, err)
	}
	defer out.Close()
	w := zip.NewWriter(out)
	for _, f := range r.File {
		fw, err := w.Create(f.Name)
		if err != nil {
			t.Fatalf("create entry %q: %v", f.Name, err)
		}
		if _, err := fw.Write(entries[f.Name]); err != nil {
			t.Fatalf("write entry %q: %v", f.Name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
}

func nameReplace(text string) (string, int) {
	if strings.Contains(text, "Rashi Patil") {
		return strings.ReplaceAll(text, "Rashi Patil", "Harold Baumbach"), 1
	}
	return text, 0
}

func TestRedactFilePreservesFormattingDocxgoDoesNotModel(t *testing.T) {
	dir := t.TempDir()
	inPath := filepath.Join(dir, "input.docx")
	outPath := filepath.Join(dir, "output.docx")

	doc := docx.NewDocument()
	p, _ := doc.AddParagraph()
	r, _ := p.AddRun()
	if err := r.SetText("Contact Rashi Patil for details."); err != nil {
		t.Fatalf("SetText: %v", err)
	}
	if err := doc.SaveAs(inPath); err != nil {
		t.Fatalf("SaveAs: %v", err)
	}

	injectParagraphShading(t, inPath)

	total, err := RedactFile(inPath, outPath, nameReplace)
	if err != nil {
		t.Fatalf("RedactFile: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected 1 redaction, got %d", total)
	}

	outZip, err := zip.OpenReader(outPath)
	if err != nil {
		t.Fatalf("open output as zip: %v", err)
	}
	defer outZip.Close()
	var outDocXML []byte
	for _, f := range outZip.File {
		if f.Name == "word/document.xml" {
			rc, _ := f.Open()
			outDocXML, _ = io.ReadAll(rc)
			rc.Close()
		}
	}

	if !bytes.Contains(outDocXML, []byte(`<w:shd w:val="clear" w:color="auto" w:fill="FF0000"/>`)) {
		t.Errorf("paragraph shading was dropped; output document.xml:\n%s", outDocXML)
	}
	if bytes.Contains(outDocXML, []byte("Rashi Patil")) {
		t.Errorf("original PII still present in output")
	}
	if !bytes.Contains(outDocXML, []byte("Harold Baumbach")) {
		t.Errorf("expected the fake replacement text in output")
	}
}

func TestRedactFilePreservesRunFormattingAndXMLSpacePreserve(t *testing.T) {
	dir := t.TempDir()
	inPath := filepath.Join(dir, "input.docx")
	outPath := filepath.Join(dir, "output.docx")

	doc := docx.NewDocument()
	p, _ := doc.AddParagraph()

	r1, _ := p.AddRun()
	r1.SetText("Confidential: ")
	r1.SetBold(true)
	r1.SetColor(docx.Red)

	r2, _ := p.AddRun()
	r2.SetText("Rashi Patil")
	r2.SetItalic(true)
	r2.SetHighlight(domain.HighlightYellow)

	r3, _ := p.AddRun()
	r3.SetText(" is the contact.")
	r3.SetUnderline(domain.UnderlineSingle)
	r3.SetColor(docx.Blue)

	if err := doc.SaveAs(inPath); err != nil {
		t.Fatalf("SaveAs: %v", err)
	}

	total, err := RedactFile(inPath, outPath, nameReplace)
	if err != nil {
		t.Fatalf("RedactFile: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected 1 redaction, got %d", total)
	}

	reopened, err := docx.OpenDocument(outPath)
	if err != nil {
		t.Fatalf("OpenDocument: %v", err)
	}
	runs := reopened.Paragraphs()[0].Runs()
	if len(runs) != 3 {
		t.Fatalf("expected 3 runs to survive, got %d", len(runs))
	}
	if !runs[0].Bold() || runs[0].Color() != docx.Red {
		t.Errorf("run 0 lost bold/color: bold=%v color=%+v", runs[0].Bold(), runs[0].Color())
	}
	if runs[1].Text() != "Harold Baumbach" {
		t.Errorf("expected run 1 redacted, got %q", runs[1].Text())
	}
	if !runs[1].Italic() || runs[1].Highlight() != domain.HighlightYellow {
		t.Errorf("run 1 lost italic/highlight: italic=%v highlight=%v", runs[1].Italic(), runs[1].Highlight())
	}
	if runs[2].Color() != docx.Blue || runs[2].Underline() != domain.UnderlineSingle {
		t.Errorf("run 2 lost color/underline: color=%+v underline=%v", runs[2].Color(), runs[2].Underline())
	}

	// xml:space="preserve" must survive on runs with significant leading/
	// trailing whitespace, or Word collapses it and words run together.
	outZip, err := zip.OpenReader(outPath)
	if err != nil {
		t.Fatalf("open output as zip: %v", err)
	}
	defer outZip.Close()
	for _, f := range outZip.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, _ := f.Open()
		data, _ := io.ReadAll(rc)
		rc.Close()
		if !bytes.Contains(data, []byte(`xml:space="preserve"`)) {
			t.Errorf(`expected xml:space="preserve" to survive in output, got:\n%s`, data)
		}
	}
}

func TestRedactFileRedactsHeadersAndFooters(t *testing.T) {
	dir := t.TempDir()
	inPath := filepath.Join(dir, "input.docx")
	outPath := filepath.Join(dir, "output.docx")

	doc := docx.NewDocument()
	sec, err := doc.DefaultSection()
	if err != nil {
		t.Fatalf("DefaultSection: %v", err)
	}

	hdr, err := sec.Header(domain.HeaderDefault)
	if err != nil {
		t.Fatalf("Header: %v", err)
	}
	hp, _ := hdr.AddParagraph()
	hr, _ := hp.AddRun()
	if err := hr.SetText("Prepared for Rashi Patil"); err != nil {
		t.Fatalf("SetText: %v", err)
	}

	ftr, err := sec.Footer(domain.FooterDefault)
	if err != nil {
		t.Fatalf("Footer: %v", err)
	}
	fp, _ := ftr.AddParagraph()
	fr, _ := fp.AddRun()
	if err := fr.SetText("Page prepared by Rashi Patil"); err != nil {
		t.Fatalf("SetText: %v", err)
	}

	body, _ := doc.AddParagraph()
	br, _ := body.AddRun()
	if err := br.SetText("Body text mentions Rashi Patil too."); err != nil {
		t.Fatalf("SetText: %v", err)
	}

	if err := doc.SaveAs(inPath); err != nil {
		t.Fatalf("SaveAs: %v", err)
	}

	total, err := RedactFile(inPath, outPath, nameReplace)
	if err != nil {
		t.Fatalf("RedactFile: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected 3 redactions (header + footer + body), got %d", total)
	}

	reopened, err := docx.OpenDocument(outPath)
	if err != nil {
		t.Fatalf("OpenDocument: %v", err)
	}
	if got := reopened.Paragraphs()[0].Text(); got != "Body text mentions Harold Baumbach too." {
		t.Errorf("body not redacted: %q", got)
	}

	reopenedSec, err := reopened.DefaultSection()
	if err != nil {
		t.Fatalf("DefaultSection: %v", err)
	}
	reopenedHdr, err := reopenedSec.Header(domain.HeaderDefault)
	if err != nil {
		t.Fatalf("Header: %v", err)
	}
	if got := reopenedHdr.Paragraphs()[0].Text(); got != "Prepared for Harold Baumbach" {
		t.Errorf("header not redacted: %q", got)
	}
	reopenedFtr, err := reopenedSec.Footer(domain.FooterDefault)
	if err != nil {
		t.Fatalf("Footer: %v", err)
	}
	if got := reopenedFtr.Paragraphs()[0].Text(); got != "Page prepared by Harold Baumbach" {
		t.Errorf("footer not redacted: %q", got)
	}
}

func TestRedactXMLPartEscapesSpecialCharactersInReplacement(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>SECRET</w:t></w:r></w:p>
  </w:body>
</w:document>`)

	redact := func(text string) (string, int) {
		if text == "SECRET" {
			return `A & B <C> "D"`, 1
		}
		return text, 0
	}

	out, count, err := redactXMLPart(data, redact)
	if err != nil {
		t.Fatalf("redactXMLPart: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 redaction, got %d", count)
	}

	spans, err := findTextSpans(out)
	if err != nil {
		t.Fatalf("findTextSpans on output: %v", err)
	}
	if len(spans) != 1 || spans[0].text != `A & B <C> "D"` {
		t.Errorf("expected the replacement to decode back correctly, got %+v", spans)
	}
}

func TestFindTextSpansDecodesEntitiesAndTracksOffsets(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t xml:space="preserve">Rashi &amp; Patil</w:t></w:r></w:p>
    <w:p><w:r><w:t/></w:r></w:p>
  </w:body>
</w:document>`)

	spans, err := findTextSpans(data)
	if err != nil {
		t.Fatalf("findTextSpans: %v", err)
	}
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans (including the empty one), got %d", len(spans))
	}
	if spans[0].text != "Rashi & Patil" {
		t.Errorf("expected decoded entity, got %q", spans[0].text)
	}
	if string(data[spans[0].start:spans[0].end]) != "Rashi &amp; Patil" {
		t.Errorf("expected raw (still-escaped) bytes at the recorded offsets, got %q", data[spans[0].start:spans[0].end])
	}
	if spans[1].text != "" {
		t.Errorf("expected the self-closing <w:t/> to have empty text, got %q", spans[1].text)
	}
}
