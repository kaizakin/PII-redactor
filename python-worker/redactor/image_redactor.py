"""OCR-based PII redaction for images embedding text — scanned IDs,
screenshots, photographed documents. Unlike text runs in a .docx, there is
no XML structure to edit here: PII lives in a pixel array, so this module
finds it via OCR and permanently blacks out the pixels it occupies.

Blurring or pixelating is deliberately not used for redaction: both are
often reconstructible with the right tooling (deblurring models, brute
force over a small pixelation block). A solid fill is not.
"""

import io
from dataclasses import dataclass
from typing import List, Tuple

import pytesseract
from PIL import Image, ImageDraw
from pytesseract import Output

from .analyzer import PresidioAnalyzer

# Below this OCR confidence (0-100), a "word" is more likely noise or a
# misread than real text; skip it rather than feed garbage into the PII
# analyzer, which would otherwise waste cycles and occasionally produce
# spurious matches on gibberish.
MIN_WORD_CONFIDENCE = 30

_FORMAT_TO_PILLOW = {
    "jpg": "JPEG",
    "jpeg": "JPEG",
    "png": "PNG",
    "bmp": "BMP",
    "gif": "GIF",
    "tif": "TIFF",
    "tiff": "TIFF",
    "webp": "WEBP",
}


@dataclass(frozen=True)
class Box:
    left: int
    top: int
    width: int
    height: int

    @property
    def right(self) -> int:
        return self.left + self.width

    @property
    def bottom(self) -> int:
        return self.top + self.height


class ImageRedactor:
    """Finds PII in an image via OCR and blacks out the matched regions."""

    def __init__(self, analyzer: PresidioAnalyzer):
        self._analyzer = analyzer

    def redact(self, image_bytes: bytes, fmt: str) -> Tuple[bytes, int]:
        """Returns (possibly modified) image bytes re-encoded in fmt, and
        how many regions were redacted. Returns the original bytes
        unchanged if no PII is found.
        """
        image = Image.open(io.BytesIO(image_bytes))
        pillow_format = _FORMAT_TO_PILLOW.get(fmt.lower(), fmt.upper())
        image = _normalize_mode(image, pillow_format)

        ocr = pytesseract.image_to_data(image, output_type=Output.DICT)
        lines = _group_words_into_lines(ocr)

        redaction_boxes: List[Box] = []
        for line_text, word_boxes, word_spans in lines:
            for entity in self._analyzer.analyze_ocr(line_text):
                overlapping = [
                    word_boxes[i]
                    for i, (start, end) in enumerate(word_spans)
                    if start < entity.end and end > entity.start
                ]
                if overlapping:
                    redaction_boxes.append(_union(overlapping))

        if not redaction_boxes:
            return image_bytes, 0

        draw = ImageDraw.Draw(image)
        for box in redaction_boxes:
            draw.rectangle([box.left, box.top, box.right, box.bottom], fill="black")

        out = io.BytesIO()
        image.save(out, format=pillow_format)
        return out.getvalue(), len(redaction_boxes)


def _normalize_mode(image: Image.Image, pillow_format: str) -> Image.Image:
    """Puts image into a color mode ImageDraw and the target format can
    both handle. Palette ("P") images don't reliably support arbitrary
    draw colors, and JPEG/BMP have no alpha channel at all.
    """
    if image.mode == "P":
        image = image.convert("RGBA" if "transparency" in image.info else "RGB")
    if pillow_format in ("JPEG", "BMP") and image.mode in ("RGBA", "LA"):
        image = image.convert("RGB")
    return image


def _group_words_into_lines(ocr: dict) -> List[Tuple[str, List[Box], List[Tuple[int, int]]]]:
    """Groups pytesseract's flat word-level output into lines, using its
    own (block_num, par_num, line_num) grouping, and reconstructs each
    line's text with a per-word character span — so a PII match spanning
    multiple words (e.g. a full name) maps back to every word box it
    covers, not just one.
    """
    groups: dict = {}
    order: List[tuple] = []
    n = len(ocr["text"])
    for i in range(n):
        text = ocr["text"][i].strip()
        if not text:
            continue
        try:
            confidence = float(ocr["conf"][i])
        except (TypeError, ValueError):
            confidence = -1
        if confidence < MIN_WORD_CONFIDENCE:
            continue

        key = (ocr["block_num"][i], ocr["par_num"][i], ocr["line_num"][i])
        if key not in groups:
            groups[key] = []
            order.append(key)
        box = Box(ocr["left"][i], ocr["top"][i], ocr["width"][i], ocr["height"][i])
        groups[key].append((text, box))

    lines = []
    for key in order:
        words = groups[key]
        parts: List[str] = []
        word_boxes: List[Box] = []
        word_spans: List[Tuple[int, int]] = []
        pos = 0
        for i, (text, box) in enumerate(words):
            if i > 0:
                parts.append(" ")
                pos += 1
            start = pos
            parts.append(text)
            pos += len(text)
            word_boxes.append(box)
            word_spans.append((start, pos))
        lines.append(("".join(parts), word_boxes, word_spans))
    return lines


def _union(boxes: List[Box]) -> Box:
    left = min(b.left for b in boxes)
    top = min(b.top for b in boxes)
    right = max(b.right for b in boxes)
    bottom = max(b.bottom for b in boxes)
    return Box(left, top, right - left, bottom - top)
