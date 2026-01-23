"""
DSPy Signature definitions for the translation pipeline.

Signatures define the typed inputs and outputs for LLM operations.
"""

import dspy
from PIL import Image as PILImage  # noqa: F401 - used for type hints


class Segmenter(dspy.Signature):
    """Segment Chinese text into words"""

    text: str = dspy.InputField(description="Chinese text to segment")
    segments: list[str] = dspy.OutputField(description="List of words")


class Translator(dspy.Signature):
    """Translate Chinese words to Pinyin and English"""

    segment: str = dspy.InputField(description="A segment of chinese text to translate")
    context: str = dspy.InputField(description="Context of the translation task")
    pinyin: str = dspy.OutputField(
        description="Pinyin transliteration of the segment"
    )
    english: str = dspy.OutputField(
        description="English translation of the segment"
    )


class FullTranslator(dspy.Signature):
    """Translate full Chinese text into English for reference"""

    text: str = dspy.InputField(description="Full Chinese text to translate")
    english: str = dspy.OutputField(description="English translation of the full text")


class OCRExtractor(dspy.Signature):
    """Extract Chinese text from an image"""

    image: dspy.Image = dspy.InputField(description="Image containing Chinese text")
    chinese_text: str = dspy.OutputField(
        description="Extracted Chinese text from the image"
    )
