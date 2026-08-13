// Package docxio applies redaction to Microsoft Word (.docx) documents
// using github.com/mmonterroca/docxgo, so the engine can redact real
// office documents in place instead of only plain text.
package docxio

import (
	"fmt"

	docx "github.com/mmonterroca/docxgo"
	"github.com/mmonterroca/docxgo/domain"
)

// RedactFunc redacts a single string of text, returning the redacted text
// and how many PII matches were replaced. This package takes it as a
// dependency rather than importing internal/processor directly, so it has
// no compile-time coupling to the detection pipeline and can be unit
// tested with a trivial fake.
type RedactFunc func(text string) (redacted string, count int)

// RedactFile reads the .docx file at inPath, redacts PII in every
// paragraph run — including runs inside table cells — and writes the
// result to outPath. It returns the total number of PII matches redacted
// across the whole document.
//
// Detection and replacement operate at the level of individual runs
// rather than whole paragraphs. A run is the smallest span of text with
// uniform formatting in OOXML, and in practice a single PII value (an
// email address, a phone number, ...) is written with consistent
// formatting and so lives entirely inside one run. A value split across a
// run boundary by a mid-token formatting change is a rare edge case this
// pass does not attempt to stitch back together.
func RedactFile(inPath, outPath string, redact RedactFunc) (int, error) {
	doc, err := docx.OpenDocument(inPath)
	if err != nil {
		return 0, fmt.Errorf("docxio: open %q: %w", inPath, err)
	}

	total := 0
	for _, para := range doc.Paragraphs() {
		total += redactRuns(para.Runs(), redact)
	}
	for _, table := range doc.Tables() {
		total += redactTable(table, redact)
	}

	if err := doc.SaveAs(outPath); err != nil {
		return total, fmt.Errorf("docxio: save %q: %w", outPath, err)
	}
	return total, nil
}

func redactTable(table domain.Table, redact RedactFunc) int {
	total := 0
	for _, row := range table.Rows() {
		for _, cell := range row.Cells() {
			for _, para := range cell.Paragraphs() {
				total += redactRuns(para.Runs(), redact)
			}
		}
	}
	return total
}

func redactRuns(runs []domain.Run, redact RedactFunc) int {
	total := 0
	for _, run := range runs {
		text := run.Text()
		if text == "" {
			continue
		}
		redacted, count := redact(text)
		if count == 0 {
			continue
		}
		if err := run.SetText(redacted); err != nil {
			// Best-effort: leave the original run text in place rather
			// than fail the whole document over one unwritable run.
			continue
		}
		total += count
	}
	return total
}
