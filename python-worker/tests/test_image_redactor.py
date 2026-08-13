import io

import pytest
import pytesseract
from PIL import Image, ImageDraw, ImageFont

from redactor.analyzer import PresidioAnalyzer
from redactor.image_redactor import Box, ImageRedactor, _group_words_into_lines, _union

_FONT_PATH = "/usr/share/fonts/TTF/DejaVuSans.ttf"


def _font(size: int = 24) -> ImageFont.FreeTypeFont:
    try:
        return ImageFont.truetype(_FONT_PATH, size)
    except OSError:
        pytest.skip(f"test font not available at {_FONT_PATH}")


def _render_png(lines: list[str]) -> bytes:
    img = Image.new("RGB", (500, 40 * len(lines) + 20), "white")
    draw = ImageDraw.Draw(img)
    font = _font()
    for i, line in enumerate(lines):
        draw.text((10, 10 + i * 40), line, fill="black", font=font)
    buf = io.BytesIO()
    img.save(buf, format="PNG")
    return buf.getvalue()


@pytest.fixture(scope="module")
def redactor():
    return ImageRedactor(PresidioAnalyzer())


def test_redact_blacks_out_pii_and_leaves_other_text(redactor):
    original = _render_png(["Name: Rashi Patil", "Email: rashi.patil@gmail.com", "Invoice #4521"])

    redacted, count = redactor.redact(original, "png")
    assert count == 2
    assert redacted != original

    text_after = pytesseract.image_to_string(Image.open(io.BytesIO(redacted)))
    assert "Rashi Patil" not in text_after
    assert "rashi.patil@gmail.com" not in text_after
    assert "Invoice #4521" in text_after


def test_redact_no_pii_returns_original_bytes_unchanged(redactor):
    original = _render_png(["This image has no sensitive information."])
    redacted, count = redactor.redact(original, "png")
    assert count == 0
    assert redacted == original


def test_redact_jpeg_round_trip(redactor):
    img = Image.new("RGB", (300, 60), "white")
    draw = ImageDraw.Draw(img)
    draw.text((10, 10), "Nothing sensitive here", fill="black", font=_font())
    buf = io.BytesIO()
    img.save(buf, format="JPEG")

    redacted, count = redactor.redact(buf.getvalue(), "jpeg")
    assert count == 0
    # Must still be a valid, openable JPEG.
    Image.open(io.BytesIO(redacted)).load()


def test_union_computes_bounding_box_of_multiple_boxes():
    boxes = [Box(10, 10, 20, 20), Box(50, 5, 10, 10)]
    box = _union(boxes)
    assert box.left == 10
    assert box.top == 5
    assert box.right == 60
    assert box.bottom == 30


def test_group_words_into_lines_reconstructs_line_text_and_spans():
    # A minimal fake pytesseract Output.DICT covering two lines: one word
    # per line here for simplicity, plus a low-confidence word that must
    # be dropped.
    ocr = {
        "text": ["Hello", "world", "junk"],
        "conf": [95, 92, 5],
        "left": [0, 60, 200],
        "top": [0, 0, 40],
        "width": [50, 50, 20],
        "height": [20, 20, 20],
        "block_num": [1, 1, 1],
        "par_num": [1, 1, 1],
        "line_num": [1, 1, 2],
    }
    lines = _group_words_into_lines(ocr)
    assert len(lines) == 1  # the junk word's line is dropped entirely

    text, boxes, spans = lines[0]
    assert text == "Hello world"
    assert spans == [(0, 5), (6, 11)]
    assert boxes[0] == Box(0, 0, 50, 20)
    assert boxes[1] == Box(60, 0, 50, 20)


def test_group_words_into_lines_skips_empty_and_low_confidence_words():
    ocr = {
        "text": ["", "  ", "ok"],
        "conf": [-1, -1, 88],
        "left": [0, 0, 0],
        "top": [0, 0, 0],
        "width": [0, 0, 10],
        "height": [0, 0, 10],
        "block_num": [1, 1, 1],
        "par_num": [1, 1, 1],
        "line_num": [1, 1, 1],
    }
    lines = _group_words_into_lines(ocr)
    assert len(lines) == 1
    assert lines[0][0] == "ok"
