"""
Pydantic request/response models for the API.

This module contains all the Pydantic models used for:
- Translation endpoints
- Text persistence API
- Vocabulary management API
- SRS (Spaced Repetition System) API
"""

from enum import IntEnum

from pydantic import BaseModel


# =============================================================================
# Translation Models
# =============================================================================


class TranslateRequest(BaseModel):
    text: str


class TranslationResult(BaseModel):
    segment: str
    pinyin: str
    english: str


class ParagraphResult(BaseModel):
    translations: list[TranslationResult]
    indent: str = ""
    separator: str


class TranslateResponse(BaseModel):
    paragraphs: list[ParagraphResult]


# =============================================================================
# Text Persistence Models
# =============================================================================


class CreateTextRequest(BaseModel):
    raw_text: str
    source_type: str = "text"  # 'text' | 'ocr'
    metadata: dict = {}


class CreateTextResponse(BaseModel):
    id: str


class TextResponse(BaseModel):
    id: str
    created_at: str
    source_type: str
    raw_text: str
    normalized_text: str
    metadata: dict


class CreateEventRequest(BaseModel):
    event_type: str
    text_id: str | None = None
    segment_id: str | None = None
    payload: dict = {}


class CreateEventResponse(BaseModel):
    id: str


# =============================================================================
# Vocabulary Models
# =============================================================================


class SaveVocabRequest(BaseModel):
    headword: str
    pinyin: str = ""
    english: str = ""
    text_id: str | None = None
    segment_id: str | None = None
    snippet: str | None = None
    status: str = "learning"  # unknown|learning|known


class SaveVocabResponse(BaseModel):
    vocab_item_id: str


class UpdateVocabStatusRequest(BaseModel):
    vocab_item_id: str
    status: str  # unknown|learning|known


class OkResponse(BaseModel):
    ok: bool = True


# =============================================================================
# SRS (Spaced Repetition System) Models
# =============================================================================


class ReviewGrade(IntEnum):
    AGAIN = 0
    HARD = 1
    GOOD = 2


class RecordLookupRequest(BaseModel):
    vocab_item_id: str


class RecordLookupResponse(BaseModel):
    vocab_item_id: str
    opacity: float
    is_struggling: bool


class VocabSRSInfoResponse(BaseModel):
    vocab_item_id: str
    headword: str
    pinyin: str
    english: str
    opacity: float
    is_struggling: bool
    status: str  # unknown|learning|known


class VocabSRSInfoListResponse(BaseModel):
    items: list[VocabSRSInfoResponse]


class ReviewCardResponse(BaseModel):
    vocab_item_id: str
    headword: str
    pinyin: str
    english: str
    snippets: list[str]


class ReviewQueueResponse(BaseModel):
    cards: list[ReviewCardResponse]
    due_count: int


class ReviewAnswerRequest(BaseModel):
    vocab_item_id: str
    grade: ReviewGrade


class ReviewAnswerResponse(BaseModel):
    vocab_item_id: str
    next_due_at: str | None
    interval_days: float
    remaining_due: int


class DueCountResponse(BaseModel):
    due_count: int
