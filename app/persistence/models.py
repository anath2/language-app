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

@dataclass
class ProgressBundle:
    """Container for progress data."""

    schema_version: int
    exported_at: str
    vocab_items: list[dict[str, Any]]
    srs_state: list[dict[str, Any]]
    vocab_lookups: list[dict[str, Any]]

