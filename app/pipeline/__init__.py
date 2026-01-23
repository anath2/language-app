"""
DSPy Pipeline for Chinese text translation.

This package provides:
- DSPy Signatures for segmentation, translation, and OCR
- Pipeline class for orchestrating the translation flow
- Thread-safe lazy initialization of pipeline instances
"""

from app.pipeline.processor import Pipeline, get_full_translator, get_pipeline
from app.pipeline.signatures import FullTranslator, OCRExtractor, Segmenter, Translator

__all__ = [
    # Signatures
    "Segmenter",
    "Translator",
    "FullTranslator",
    "OCRExtractor",
    # Pipeline
    "Pipeline",
    "get_pipeline",
    "get_full_translator",
]
