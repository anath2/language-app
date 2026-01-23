"""
Utility functions for text and image processing.

This module provides:
- Text processing (paragraph splitting, translation skip logic)
- Image validation and OCR extraction
"""

import io
import unicodedata
from pathlib import Path

import dspy
from PIL import Image as PILImage

from app.config import ALLOWED_EXTENSIONS, CHINESE_PUNCTUATION, MAX_FILE_SIZE
from app.pipeline.signatures import OCRExtractor


def should_skip_translation(segment: str) -> bool:
    """
    Check if a segment should skip translation.
    Returns True if segment contains only:
    - Whitespace
    - ASCII punctuation/symbols
    - ASCII digits
    - Chinese punctuation
    - Full-width numbers and symbols
    """
    if not segment or not segment.strip():
        return True

    for char in segment:
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
        if category in ('Nd', 'No', 'Po', 'Ps', 'Pe', 'Pd', 'Pc', 'Sk', 'Sm', 'So'):
            # Nd: Decimal number, No: Other number
            # Po: Other punctuation, Ps: Open punctuation, Pe: Close punctuation
            # Pd: Dash punctuation, Pc: Connector punctuation
            # Sk: Modifier symbol, Sm: Math symbol, So: Other symbol
            continue

        # If we found a character that's not punctuation/number/symbol, don't skip
        return False

    # All characters are punctuation/numbers/symbols
    return True


def split_into_paragraphs(text: str) -> list[dict[str, str]]:
    """
    Split text into paragraphs while preserving whitespace information.
    Returns a list of dicts with 'content' and 'separator' keys.
    The separator indicates what whitespace follows this paragraph.
    """
    if not text:
        return []

    # Split by newlines while keeping track of the separators
    lines = text.split('\n')
    paragraphs = []

    for i, line in enumerate(lines):
        # Skip completely empty lines at the start
        if not paragraphs and not line.strip():
            continue

        # For non-empty lines, add them as paragraphs
        if line.strip():
            # Determine the separator by looking ahead
            # Count consecutive newlines after this line
            separator = '\n'
            j = i + 1
            while j < len(lines) and not lines[j].strip():
                separator += '\n'
                j += 1

            paragraphs.append({
                'content': line.strip(),
                'separator': separator if i < len(lines) - 1 else ''
            })

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
