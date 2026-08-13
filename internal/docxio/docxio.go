// Package docxio applies redaction to Microsoft Word (.docx) documents.
//
// Text (word/document.xml and every header/footer part) is redacted by
// splicing directly into the raw XML at exact <w:t> byte ranges — see
// xmltext.go for why. Embedded images (word/media/*) are redacted by
// swapping ZIP entries directly — see images.go for why docxgo can't do
// that either. Neither pass uses docxgo for anything beyond what test
// fixtures in this package construct documents with.
package docxio

import (
	"fmt"
	"os"
	"path/filepath"
)

// RedactFunc redacts a single string of text, returning the redacted text
// and how many PII matches were replaced. This package takes it as a
// dependency rather than importing internal/processor directly, so it has
// no compile-time coupling to the detection pipeline and can be unit
// tested with a trivial fake.
type RedactFunc func(text string) (redacted string, count int)

// maxConcurrentRedactions bounds how many <w:t> nodes or images are
// redacted at once. A redact call may reach out to the Python NLP worker
// over gRPC, so this also bounds how many requests one document generates
// concurrently.
const maxConcurrentRedactions = 16

// RedactDocument runs the full redaction pipeline against the .docx file
// at inPath: text first (textRedact, across the document body and every
// header/footer — see RedactFile), then embedded images (imageRedact,
// against every entry under word/media/ — see RedactImagesInFile). It
// returns the number of text matches and image regions redacted.
func RedactDocument(inPath, outPath string, textRedact RedactFunc, imageRedact ImageRedactFunc) (textMatches, imageRegions int, err error) {
	intermediate, err := os.CreateTemp(filepath.Dir(outPath), "docxio-text-*.docx")
	if err != nil {
		return 0, 0, fmt.Errorf("docxio: create intermediate file: %w", err)
	}
	intermediatePath := intermediate.Name()
	intermediate.Close()
	defer os.Remove(intermediatePath)

	textMatches, err = RedactFile(inPath, intermediatePath, textRedact)
	if err != nil {
		return 0, 0, err
	}

	imageRegions, err = RedactImagesInFile(intermediatePath, outPath, imageRedact)
	if err != nil {
		return textMatches, 0, err
	}
	return textMatches, imageRegions, nil
}
