package docxio

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/kaizakin/PII-redactor/internal/safe"
)

// ImageRedactFunc redacts a single embedded image, returning the (possibly
// re-encoded) image bytes and how many PII regions were blacked out. Like
// RedactFunc, this package takes it as a dependency rather than importing
// internal/grpcclient directly, keeping this package's only real-world
// dependency on OCR/PII detection at the call site, not compiled in.
type ImageRedactFunc func(data []byte, format string) (redacted []byte, redactions int)

// mediaPrefix is where a .docx package stores embedded images. docxgo's
// domain.Image exposes Data() to read these bytes but has no way to
// replace them (there is no SetData), so image redaction works one layer
// below docxgo: directly on the .docx ZIP archive, swapping out media
// entries while leaving every other part of the package — document.xml,
// relationships, content types, and any image not being replaced — passed
// through byte-for-byte. An image's relationship ID and target path never
// change, only its pixel content, so this never needs to understand or
// touch OOXML relationships.
const mediaPrefix = "word/media/"

// MediaFile is a single embedded image extracted from a .docx package.
type MediaFile struct {
	// Name is the full zip entry path, e.g. "word/media/image1.png".
	Name string
	Data []byte
}

// Format returns a lowercase format hint derived from the file extension
// (e.g. "png", "jpeg"), matching what the NLP worker's RedactImage RPC
// expects and what docxgo's domain.ImageFormat values use.
func (m MediaFile) Format() string {
	ext := strings.ToLower(strings.TrimPrefix(fileExt(m.Name), "."))
	if ext == "jpg" {
		return "jpeg"
	}
	return ext
}

func fileExt(name string) string {
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		return name[i:]
	}
	return ""
}

// ExtractImages returns every embedded image in the .docx file at path.
func ExtractImages(path string) ([]MediaFile, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("docxio: open %q as zip: %w", path, err)
	}
	defer r.Close()

	var images []MediaFile
	for _, f := range r.File {
		if !strings.HasPrefix(f.Name, mediaPrefix) {
			continue
		}
		data, err := readZipEntry(f)
		if err != nil {
			return nil, fmt.Errorf("docxio: read %q: %w", f.Name, err)
		}
		images = append(images, MediaFile{Name: f.Name, Data: data})
	}
	return images, nil
}

// RedactImagesInFile redacts every embedded image in the .docx file at
// inPath and writes the result to outPath. It returns the total number of
// PII regions redacted across every image. If the document has no images,
// or none of them contain PII, outPath is an unmodified copy of inPath.
//
// Images are redacted concurrently (bounded by maxConcurrentRedactions),
// same as text runs in RedactFile — each redact call may be a network
// round trip to the NLP worker's OCR pipeline, and a document can embed
// several images (scanned pages, screenshots, ID photos).
func RedactImagesInFile(inPath, outPath string, redact ImageRedactFunc) (int, error) {
	images, err := ExtractImages(inPath)
	if err != nil {
		return 0, err
	}
	if len(images) == 0 {
		return 0, copyFile(inPath, outPath)
	}
	log.Printf("docxio: %q has %d embedded images to scan", inPath, len(images))

	redactedData := make([][]byte, len(images))
	counts := make([]int, len(images))

	var g errgroup.Group
	g.SetLimit(maxConcurrentRedactions)
	for i, img := range images {
		g.Go(func() error {
			defer safe.Recover(fmt.Sprintf("docxio.RedactImagesInFile image %d (%s)", i, img.Name))
			redactedData[i], counts[i] = redact(img.Data, img.Format())
			return nil
		})
	}
	_ = g.Wait() // redact has no error to propagate; every goroutine above always returns nil

	total := 0
	replacements := make(map[string][]byte, len(images))
	for i, img := range images {
		if counts[i] == 0 {
			continue
		}
		replacements[img.Name] = redactedData[i]
		total += counts[i]
	}

	if len(replacements) == 0 {
		return 0, copyFile(inPath, outPath)
	}
	return total, ReplaceImages(inPath, outPath, replacements)
}

func copyFile(inPath, outPath string) error {
	src, err := os.Open(inPath)
	if err != nil {
		return fmt.Errorf("docxio: open %q: %w", inPath, err)
	}
	defer src.Close()

	dst, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("docxio: create %q: %w", outPath, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("docxio: copy %q to %q: %w", inPath, outPath, err)
	}
	return nil
}

// ReplaceImages copies every entry from the .docx file at inPath into a
// new .docx file at outPath, substituting the content of media entries
// named as keys in replacements. Every other entry passes through
// unchanged.
func ReplaceImages(inPath, outPath string, replacements map[string][]byte) error {
	r, err := zip.OpenReader(inPath)
	if err != nil {
		return fmt.Errorf("docxio: open %q as zip: %w", inPath, err)
	}
	defer r.Close()

	out, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("docxio: create %q: %w", outPath, err)
	}
	defer out.Close()

	w := zip.NewWriter(out)
	for _, f := range r.File {
		if data, ok := replacements[f.Name]; ok {
			if err := writeZipEntry(w, f, data); err != nil {
				return fmt.Errorf("docxio: write replaced %q: %w", f.Name, err)
			}
			continue
		}
		if err := copyZipEntry(w, f); err != nil {
			return fmt.Errorf("docxio: copy %q: %w", f.Name, err)
		}
	}
	return w.Close()
}

func readZipEntry(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func copyZipEntry(w *zip.Writer, f *zip.File) error {
	dst, err := w.CreateHeader(&f.FileHeader)
	if err != nil {
		return err
	}
	src, err := f.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	_, err = io.Copy(dst, src)
	return err
}

// writeZipEntry writes replacement content under the same name and
// compression method as the original entry. zip.Writer computes the
// correct CRC-32 and size for the new content as it's written; the stale
// values on FileHeader (from the original image) are not used for that.
func writeZipEntry(w *zip.Writer, f *zip.File, data []byte) error {
	dst, err := w.CreateHeader(&f.FileHeader)
	if err != nil {
		return err
	}
	_, err = io.Copy(dst, bytes.NewReader(data))
	return err
}
