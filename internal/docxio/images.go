package docxio

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
)

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
