"""
Persistence layer for the Language App.

This package provides database operations for:
- Text storage and retrieval
- Event logging
- Vocabulary management
- SRS (Spaced Repetition System) state

Re-exports maintain backward compatibility with existing code.
"""

from app.persistence.crud import (
    create_event,
    create_text,
    get_text,
    save_vocab_item,
    update_vocab_status,
)
from app.persistence.db import db_conn, get_db_path, init_db
from app.persistence.models import ReviewCard, SRSState, TextRecord, VocabSRSInfo
from app.persistence.srs import (
    compute_opacity,
    get_due_count,
    get_review_queue,
    get_vocab_srs_info,
    initialize_srs_state,
    is_struggling,
    record_lookup,
    record_review_grade,
)

__all__ = [
    # DB
    "init_db",
    "db_conn",
    "get_db_path",
    # Models
    "TextRecord",
    "SRSState",
    "VocabSRSInfo",
    "ReviewCard",
    # CRUD
    "create_text",
    "get_text",
    "create_event",
    "save_vocab_item",
    "update_vocab_status",
    # SRS
    "initialize_srs_state",
    "is_struggling",
    "compute_opacity",
    "record_lookup",
    "get_vocab_srs_info",
    "record_review_grade",
    "get_review_queue",
    "get_due_count",
]
