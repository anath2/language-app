"""
DSPy Pipeline for Chinese text processing.

This module provides:
- Pipeline class for segmentation and translation
- Thread-safe lazy initialization of pipeline instances
"""

from threading import Lock

import dspy

from app.pipeline.signatures import FullTranslator, Segmenter, Translator
from app.utils import should_skip_segment

# Thread-safe lazy initialization
_pipeline_lock = Lock()
_pipeline = None
_full_translation_lock = Lock()
_full_translation_model = None


def get_pipeline() -> "Pipeline":
    """Thread-safe lazy initialization of pipeline"""
    global _pipeline
    if _pipeline is None:
        with _pipeline_lock:
            if _pipeline is None:
                _pipeline = Pipeline()
    return _pipeline


def get_full_translator():
    """Thread-safe lazy initialization for full-text translation"""
    global _full_translation_model
    if _full_translation_model is None:
        with _full_translation_lock:
            if _full_translation_model is None:
                _full_translation_model = dspy.ChainOfThought(FullTranslator)
    return _full_translation_model


class Pipeline(dspy.Module):
    """Pipeline for Chinese text processing"""

    def __init__(self):
        self.segment = dspy.ChainOfThought(Segmenter)
        self.translate = dspy.Predict(Translator)

    def forward(self, text: str) -> list[tuple[str, str, str]]:
        """Sync: Segment and translate Chinese text"""
        segmentation = self.segment(text=text)

        result = []
        for segment in segmentation.segments:
            # Skip translation for segments with only symbols, numbers, and punctuation
            if should_skip_segment(segment):
                result.append((segment, "", ""))
            else:
                translation = self.translate(segment=segment, context=text)
                result.append((segment, translation.pinyin, translation.english))
        return result

    async def aforward(self, text: str) -> list[tuple[str, str, str]]:
        """Async: Segment and translate Chinese text"""
        segmentation = await self.segment.acall(text=text)

        result = []
        for segment in segmentation.segments:
            # Skip translation for segments with only symbols, numbers, and punctuation
            if should_skip_segment(segment):
                result.append((segment, "", ""))
            else:
                translation = await self.translate.acall(segment=segment, context=text)
                result.append((segment, translation.pinyin, translation.english))
        return result
