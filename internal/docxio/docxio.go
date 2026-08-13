// Package docxio applies redaction to Microsoft Word (.docx) documents
// using github.com/mmonterroca/docxgo, so the engine can redact real
// office documents in place instead of only plain text.
package docxio

import (
	"fmt"

	docx "github.com/mmonterroca/docxgo"
	"github.com/mmonterroca/docxgo/domain"
	"golang.org/x/sync/errgroup"
)

// RedactFunc redacts a single string of text, returning the redacted text
// and how many PII matches were replaced. This package takes it as a
// dependency rather than importing internal/processor directly, so it has
// no compile-time coupling to the detection pipeline and can be unit
// tested with a trivial fake.
type RedactFunc func(text string) (redacted string, count int)

// maxConcurrentRedactions bounds how many runs are redacted at once. A
// redact call may reach out to the Python NLP worker over gRPC, so this
// also bounds how many requests one document generates concurrently.
const maxConcurrentRedactions = 16

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

	total := redactRuns(collectRuns(doc), redact)

	if err := doc.SaveAs(outPath); err != nil {
		return total, fmt.Errorf("docxio: save %q: %w", outPath, err)
	}
	return total, nil
}

// collectRuns flattens every run in the document — top-level paragraphs
// and every table cell's paragraphs — into one slice. Gathering them all
// up front, rather than redacting paragraph by paragraph, is what lets
// redactRuns fan detection out across the whole document concurrently
// instead of one run at a time: a real document can easily have hundreds
// of runs (Word splits sentences into many runs over edits, spell-check,
// and formatting changes), and each redact call may be a network round
// trip, so processing them one at a time does not scale.
func collectRuns(doc domain.Document) []domain.Run {
	var runs []domain.Run
	for _, para := range doc.Paragraphs() {
		runs = append(runs, para.Runs()...)
	}
	for _, table := range doc.Tables() {
		for _, row := range table.Rows() {
			for _, cell := range row.Cells() {
				for _, para := range cell.Paragraphs() {
					runs = append(runs, para.Runs()...)
				}
			}
		}
	}
	return runs
}

// redactRuns redacts every run's text concurrently (bounded by
// maxConcurrentRedactions) and only afterward applies the results via
// run.SetText, one at a time. The concurrency is what makes redacting a
// document with many runs fast; the sequential apply step is required
// because docxgo's Document is not safe for concurrent mutation.
func redactRuns(runs []domain.Run, redact RedactFunc) int {
	redacted := make([]string, len(runs))
	counts := make([]int, len(runs))

	var g errgroup.Group
	g.SetLimit(maxConcurrentRedactions)
	for i, run := range runs {
		text := run.Text()
		if text == "" {
			continue
		}
		g.Go(func() error {
			redacted[i], counts[i] = redact(text)
			return nil
		})
	}
	_ = g.Wait() // redact has no error to propagate; every goroutine above always returns nil

	total := 0
	for i, run := range runs {
		if counts[i] == 0 {
			continue
		}
		if err := run.SetText(redacted[i]); err != nil {
			// Best-effort: leave the original run text in place rather
			// than fail the whole document over one unwritable run.
			continue
		}
		total += counts[i]
	}
	return total
}
