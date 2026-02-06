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


class TranslateBatchRequest(BaseModel):
    """Request to translate a batch of segment texts."""

    segments: list[str]  # Chinese text strings to translate
    context: str | None = None  # Optional full text for context
    job_id: str | None = None  # Optional job ID for persistence
    paragraph_idx: int | None = None  # Required if job_id is provided


class TranslateBatchResponse(BaseModel):
    """Response with translated segments."""

    translations: list[TranslationResult]


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


# =============================================================================
# Job Queue Models
# =============================================================================


class CreateJobRequest(BaseModel):
    """Request to create a new translation job."""

    input_text: str
    source_type: str = "text"  # text, ocr


class CreateJobResponse(BaseModel):
    """Response after creating a job."""

    job_id: str
    status: str


class JobSummary(BaseModel):
    """Summary of a job for list views."""

    id: str
    created_at: str
    status: str  # pending, processing, completed, failed
    source_type: str
    input_preview: str  # First 100 chars of input_text
    full_translation_preview: str | None  # First 100 chars of full translation
    segment_count: int | None  # Completed segments
    total_segments: int | None  # Total segments


class ListJobsResponse(BaseModel):
    """Response for listing jobs."""

    jobs: list[JobSummary]
    total: int


class JobDetailResponse(BaseModel):
    """Full job details with translation results."""

    id: str
    created_at: str
    status: str
    source_type: str
    input_text: str
    full_translation: str | None
    error_message: str | None
    paragraphs: list[ParagraphResult] | None


class JobStatusResponse(BaseModel):
    """Quick job status check response."""

    job_id: str
    status: str
    progress: int | None  # Completed segment count
    total: int | None  # Total segment count
