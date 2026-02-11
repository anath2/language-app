"""
Persistence layer dataclasses.

These are internal data models for the persistence layer,
distinct from the Pydantic API models in app/models.py.
"""

from dataclasses import dataclass
from typing import Any


@dataclass(frozen=True)
class TextRecord:
    id: str
    created_at: str
    source_type: str
    raw_text: str
    normalized_text: str
    metadata: dict[str, Any]


@dataclass(frozen=True)
class SRSState:
    """SRS state for a vocab item."""

    vocab_item_id: str
    due_at: str | None
    interval_days: float
    ease: float
    reps: int
    lapses: int
    last_reviewed_at: str | None


@dataclass(frozen=True)
class VocabSRSInfo:
    """SRS info for rendering opacity in the UI."""

    vocab_item_id: str
    headword: str
    pinyin: str
    english: str
    opacity: float  # 0.0 (transparent/familiar) to 1.0 (solid/new)
    is_struggling: bool
    status: str  # unknown|learning|known


@dataclass(frozen=True)
class ReviewCard:
    """A card for the active review queue."""

    vocab_item_id: str
    headword: str
    pinyin: str
    english: str
    snippets: list[str]


@dataclass(frozen=True)
class UserProfile:
    """User profile data."""

    name: str
    email: str
    language: str
    created_at: str
    updated_at: str


@dataclass(frozen=True)
class TranslationRecord:
    """Translation queue record."""

    id: str
    created_at: str
    updated_at: str
    status: str  # pending, processing, completed, failed
    translation_type: str
    source_type: str
    input_text: str
    full_translation: str | None
    error_message: str | None
    metadata: dict[str, Any]
    text_id: str | None


@dataclass(frozen=True)
class TranslationSegmentRecord:
    """Individual segment translation result within a translation."""

    id: str
    translation_id: str
    paragraph_idx: int
    seg_idx: int
    segment_text: str
    pinyin: str
    english: str
    created_at: str


@dataclass(frozen=True)
class TranslationParagraphRecord:
    """Paragraph metadata for a translation."""

    id: str
    translation_id: str
    paragraph_idx: int
    indent: str
    separator: str


@dataclass(frozen=True)
class TranslationWithResults:
    """Translation with full translation results for API responses."""

    translation: TranslationRecord
    paragraphs: list[dict[str, Any]]  # [{translations: [...], indent, separator}]


@dataclass
class ProgressBundle:
    """Container for progress data."""

    schema_version: int
    exported_at: str
    vocab_items: list[dict[str, Any]]
    srs_state: list[dict[str, Any]]
    vocab_lookups: list[dict[str, Any]]
    # Translation queue data for translation history
    translations: list[dict[str, Any]] | None = None
    translation_segments: list[dict[str, Any]] | None = None
    translation_paragraphs: list[dict[str, Any]] | None = None
