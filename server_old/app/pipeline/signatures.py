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
    """Select the best English definition for a single Chinese word based on context.

    You are defining ONE WORD, not translating the whole sentence.
    Use the dictionary entry to pick the most appropriate meaning for this context.
    """

    segment: str = dspy.InputField(description="Chinese word to define")
    sentence_context: str = dspy.InputField(
        description="Full sentence where this word appears (for disambiguation only)"
    )
    dictionary_entry: str = dspy.InputField(
        description="CC-CEDICT definitions separated by ' / '. Pick the best one for this context. May be 'Not in dictionary' if word not found."
    )
    english: str = dspy.OutputField(
        description="Best definition for this context (1-5 words). Pick from dictionary when available."
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
