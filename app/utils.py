"""
Utility functions for text and image processing.

This module provides:
- Text processing (paragraph splitting, translation skip logic)
- Deterministic pinyin generation
- Image validation and OCR extraction
"""

import io
import unicodedata
from pathlib import Path

import dspy
from PIL import Image as PILImage
from pypinyin import Style, pinyin

from app.config import ALLOWED_EXTENSIONS, CHINESE_PUNCTUATION, MAX_FILE_SIZE
from app.pipeline.signatures import OCRExtractor


def _is_cjk_ideograph(char: str) -> bool:
    """Check if a character is a CJK Unified Ideograph (Chinese character)."""
    code_point = ord(char)
    # CJK Unified Ideographs: U+4E00-U+9FFF
    # CJK Unified Ideographs Extension A: U+3400-U+4DBF
    # CJK Unified Ideographs Extension B-F and beyond
    return (
        0x4E00 <= code_point <= 0x9FFF  # Main CJK block
        or 0x3400 <= code_point <= 0x4DBF  # Extension A
        or 0x20000 <= code_point <= 0x2A6DF  # Extension B
        or 0x2A700 <= code_point <= 0x2CEAF  # Extensions C-E
        or 0x2CEB0 <= code_point <= 0x2EBEF  # Extensions F-I
        or 0x30000 <= code_point <= 0x323AF  # Extensions G-H
    )


def should_skip_segment(segment: str) -> bool:
    """
    Check if a segment should be skipped (for translation and SRS).
    Returns True if segment:
    - Is empty or whitespace only
    - Contains no CJK ideographs (Chinese characters)
    - Contains only punctuation/symbols/numbers
    """
    if not segment or not segment.strip():
        return True

    has_cjk = False

    for char in segment:
        # Check if it's a CJK ideograph (Chinese character)
        if _is_cjk_ideograph(char):
            has_cjk = True
            continue

        # Skip whitespace
        if char.isspace():
            continue

        # Check if it's ASCII punctuation, symbol, or digit
        if char.isascii() and not char.isalpha():
            continue

        # Check if it's Chinese punctuation
        if char in CHINESE_PUNCTUATION:
            continue

        # Check if it's a full-width number or symbol (Unicode category)
        category = unicodedata.category(char)
        if category in ("Nd", "No", "Po", "Ps", "Pe", "Pd", "Pc", "Sk", "Sm", "So"):
            # Nd: Decimal number, No: Other number
            # Po: Other punctuation, Ps: Open punctuation, Pe: Close punctuation
            # Pd: Dash punctuation, Pc: Connector punctuation
            # Sk: Modifier symbol, Sm: Math symbol, So: Other symbol
            continue

        # Non-CJK alphabetic characters (e.g., English letters) - not skippable punctuation,
        # but we only care if there's at least one CJK character in the segment

    # Skip if no CJK characters were found
    return not has_cjk


def to_pinyin(segment: str) -> str:
    """
    Convert a Chinese segment to pinyin with tone marks.

    Uses pypinyin for deterministic conversion. Syllables are joined with spaces
    to match the expected API output format (e.g., "nǐ hǎo").

    Non-hanzi characters are preserved as-is via the errors handler.
    """
    if should_skip_segment(segment):
        return ""

    # Get pinyin for each character/word
    # Each element in the result is a list of possible readings; we take the first
    result = pinyin(segment, style=Style.TONE, heteronym=False, errors=lambda x: x)

    # Flatten: pinyin() returns list of lists, take first reading from each
    syllables = [readings[0] for readings in result]

    # Join with spaces and normalize whitespace
    return " ".join(syllables).strip()


def split_into_paragraphs(text: str) -> list[dict[str, str]]:
    """
    Split text into paragraphs while preserving whitespace information.
    Returns a list of dicts with 'content', 'indent', and 'separator' keys.
    - content: the text content with leading/trailing whitespace stripped
    - indent: the leading whitespace (spaces/tabs) preserved for formatting
    - separator: the whitespace (newlines) that follows this paragraph
    """
    if not text:
        return []

    # Split by newlines while keeping track of the separators
    lines = text.split("\n")
    paragraphs = []

    for i, line in enumerate(lines):
        # Skip completely empty lines at the start
        if not paragraphs and not line.strip():
            continue

        # For non-empty lines, add them as paragraphs
        if line.strip():
            # Determine the separator by looking ahead
            # Count consecutive newlines after this line
            separator = "\n"
            j = i + 1
            while j < len(lines) and not lines[j].strip():
                separator += "\n"
                j += 1

            # Extract leading whitespace (indent) before stripping
            indent = line[: len(line) - len(line.lstrip())]

            paragraphs.append(
                {
                    "content": line.strip(),
                    "indent": indent,
                    "separator": separator if i < len(lines) - 1 else "",
                }
            )

    return paragraphs


def validate_image_file(file_bytes: bytes, filename: str) -> tuple[bool, str | None]:
    """Validate uploaded image file"""
    if len(file_bytes) > MAX_FILE_SIZE:
        return False, "File too large. Maximum size is 5MB"
    ext = Path(filename).suffix.lower()
    if ext not in ALLOWED_EXTENSIONS:
        return False, f"Unsupported file type. Allowed: {', '.join(ALLOWED_EXTENSIONS)}"
    return True, None


async def extract_text_from_image(image_bytes: bytes) -> str:
    """Extract Chinese text from image using the configured LM"""
    pil_image = PILImage.open(io.BytesIO(image_bytes))
    original_format = pil_image.format

    # Formats supported by vision APIs - no conversion needed
    supported_formats = {"JPEG", "PNG", "WEBP", "GIF"}

    if original_format in supported_formats:
        # Use original bytes directly if format is supported
        # Only need to handle RGBA/P mode conversion for JPEG
        if original_format == "JPEG" and pil_image.mode in ("RGBA", "P"):
            pil_image = pil_image.convert("RGB")
            buffer = io.BytesIO()
            pil_image.save(buffer, format="JPEG", quality=85)
            normalized_bytes = buffer.getvalue()
        else:
            normalized_bytes = image_bytes
    else:
        # Convert unsupported formats (MPO, BMP, TIFF, etc.) to JPEG
        if pil_image.mode in ("RGBA", "P"):
            pil_image = pil_image.convert("RGB")
        buffer = io.BytesIO()
        pil_image.save(buffer, format="JPEG", quality=85)
        normalized_bytes = buffer.getvalue()

    image = dspy.Image(normalized_bytes)
    extractor = dspy.ChainOfThought(OCRExtractor)
    result = await extractor.acall(image=image)
    return result.chinese_text
