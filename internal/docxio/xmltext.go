package docxio

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"path"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/kaizakin/PII-redactor/internal/safe"
)

// wordNS is the WordprocessingML namespace URI. Matching an element by
// namespace + local name — the way encoding/xml itself resolves a
// prefixed name — is what lets isWordText recognize <w:t> regardless of
// whatever prefix a particular document happens to declare for that
// namespace (almost always "w", but never guaranteed).
const wordNS = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"

// textSpan is one <w:t> element's content: its exact byte range in the
// ORIGINAL XML bytes, and its XML-decoded (entity-unescaped) text value.
// Redaction never rebuilds or reserializes any XML — it only ever splices
// a new escaped string into this exact byte range, leaving every
// surrounding byte (attributes, sibling elements, whitespace) untouched.
type textSpan struct {
	start, end int64
	text       string
}

// findTextSpans locates every <w:t> element in one OOXML part (document.xml,
// or a header/footer part) using encoding/xml purely as a tokenizer to find
// exact byte offsets and correctly XML-decode entities — never as a
// round-trip encoder. Go's xml.Encoder does not preserve original
// namespace prefixes when re-serializing tokens it decoded (it invents its
// own, differently for every element), so reconstructing XML by decoding
// into generic tokens and re-encoding them would scramble every namespaced
// tag in the document. Byte-splicing into the original input sidesteps
// that failure mode entirely: nothing is ever re-encoded.
func findTextSpans(data []byte) ([]textSpan, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))

	var spans []textSpan
	inText := false
	var spanStart int64
	var text strings.Builder

	for {
		beforeOffset := dec.InputOffset()
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if isWordText(t.Name) {
				inText = true
				spanStart = dec.InputOffset()
				text.Reset()
			}
		case xml.CharData:
			if inText {
				text.Write(t)
			}
		case xml.EndElement:
			if inText && isWordText(t.Name) {
				spans = append(spans, textSpan{start: spanStart, end: beforeOffset, text: text.String()})
				inText = false
			}
		}
	}
	return spans, nil
}

func isWordText(name xml.Name) bool {
	return name.Local == "t" && name.Space == wordNS
}

// redactXMLPart redacts every <w:t> text node in a single OOXML part,
// returning the modified bytes and how many PII matches were replaced.
// Spans are redacted concurrently (bounded by maxConcurrentRedactions),
// matching this package's existing pattern for runs and images, since
// each redact call may be a network round trip to the NLP worker.
func redactXMLPart(data []byte, redact RedactFunc) ([]byte, int, error) {
	spans, err := findTextSpans(data)
	if err != nil {
		return nil, 0, fmt.Errorf("parse xml: %w", err)
	}

	redacted := make([]string, len(spans))
	counts := make([]int, len(spans))

	var g errgroup.Group
	g.SetLimit(maxConcurrentRedactions)
	for i, span := range spans {
		if span.text == "" {
			continue
		}
		g.Go(func() error {
			defer safe.Recover(fmt.Sprintf("docxio.redactXMLPart span %d", i))
			redacted[i], counts[i] = redact(span.text)
			return nil
		})
	}
	_ = g.Wait() // redact has no error to propagate; every goroutine above always returns nil

	total := 0
	var buf bytes.Buffer
	var lastEnd int64
	for i, span := range spans {
		if counts[i] == 0 {
			continue
		}
		total += counts[i]
		buf.Write(data[lastEnd:span.start])
		xml.EscapeText(&buf, []byte(redacted[i])) //nolint:errcheck // EscapeText only errors on a broken io.Writer; bytes.Buffer never fails
		lastEnd = span.end
	}
	if total == 0 {
		return data, 0, nil
	}
	buf.Write(data[lastEnd:])
	return buf.Bytes(), total, nil
}

// redactablePartNames returns the names of every XML part in the .docx
// package that can contain visible <w:t> text: the main document body,
// and every header/footer. Only scanning word/document.xml — which an
// earlier, docxgo-based implementation did — misses PII placed in a
// header or footer entirely.
func redactablePartNames(r *zip.ReadCloser) []string {
	var names []string
	for _, f := range r.File {
		if isRedactableTextPart(f.Name) {
			names = append(names, f.Name)
		}
	}
	return names
}

func isRedactableTextPart(name string) bool {
	if name == "word/document.xml" {
		return true
	}
	if path.Dir(name) != "word" {
		return false
	}
	base := path.Base(name)
	return strings.HasSuffix(base, ".xml") && (strings.HasPrefix(base, "header") || strings.HasPrefix(base, "footer"))
}

// RedactFile redacts PII in every <w:t> text node across the document
// body and all headers/footers of the .docx file at inPath, writing the
// result to outPath. It returns the total number of PII matches redacted.
//
// This operates directly on the OOXML package's raw XML rather than
// through docxgo's typed object model. That's deliberate, not a style
// choice: docxgo's domain model represents bold, italic, color,
// underline, and highlight, and text redacted through it preserves all of
// those correctly — but paragraph/run shading (<w:shd>) and any other
// property outside that model has no field to round-trip through at all,
// and is silently dropped on ANY read+write pass through docxgo, even for
// a paragraph redaction never touches. Splicing directly into the
// original bytes at exactly the <w:t> content it changes means every
// other byte in the file — known formatting or not — passes through
// unchanged, by construction.
//
// Detection and replacement operate at the level of individual <w:t>
// nodes, which in virtually every real document is exactly one run's
// text. A PII value split across a run boundary by a mid-token formatting
// change is a rare edge case this pass does not attempt to stitch back
// together.
func RedactFile(inPath, outPath string, redact RedactFunc) (int, error) {
	r, err := zip.OpenReader(inPath)
	if err != nil {
		return 0, fmt.Errorf("docxio: open %q as zip: %w", inPath, err)
	}
	defer r.Close()

	partNames := redactablePartNames(r)
	log.Printf("docxio: %q has %d text parts to scan (document + headers/footers)", inPath, len(partNames))

	byName := make(map[string]*zip.File, len(r.File))
	for _, f := range r.File {
		byName[f.Name] = f
	}

	total := 0
	replacements := make(map[string][]byte)
	for _, name := range partNames {
		data, err := readZipEntry(byName[name])
		if err != nil {
			return 0, fmt.Errorf("docxio: read %q: %w", name, err)
		}
		redactedData, count, err := redactXMLPart(data, redact)
		if err != nil {
			return 0, fmt.Errorf("docxio: redact %q: %w", name, err)
		}
		if count == 0 {
			continue
		}
		replacements[name] = redactedData
		total += count
	}

	if len(replacements) == 0 {
		return 0, copyFile(inPath, outPath)
	}
	return total, ReplaceImages(inPath, outPath, replacements)
}
