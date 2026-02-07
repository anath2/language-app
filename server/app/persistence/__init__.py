"""
Persistence layer for the Language App.

This package provides database operations for:
- Text storage and retrieval
- Event logging
- Vocabulary management
- SRS (Spaced Repetition System) state
- User profile management

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
from app.persistence.models import (
    ReviewCard,
    SRSState,
    TextRecord,
    TranslationParagraphRecord,
    TranslationRecord,
    TranslationSegmentRecord,
    TranslationWithResults,
    VocabSRSInfo,
)
from app.persistence.translations import (
    complete_translation,
    create_translation,
    delete_translation,
    fail_translation,
    get_translation,
    get_translation_segment_count,
    get_translation_segments,
    get_translation_with_results,
    list_translations,
    save_translation_paragraph,
    save_translation_segment,
    update_translation_segments,
    update_translation_status,
)
from app.persistence.profile import (
    UserProfile,
    count_known_vocab,
    count_learning_vocab,
    count_total_vocab,
    get_user_profile,
    upsert_user_profile,
)
from app.persistence.progress_sync import (
    ImportError as ProgressImportError,
    ProgressBundle,
    export_progress,
    export_progress_json,
    import_progress,
    import_progress_json,
    validate_progress_bundle,
)
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
    "UserProfile",
    "TranslationRecord",
    "TranslationSegmentRecord",
    "TranslationParagraphRecord",
    "TranslationWithResults",
    # CRUD
    "create_text",
    "get_text",
    "create_event",
    "save_vocab_item",
    "update_vocab_status",
    # Translations
    "create_translation",
    "get_translation",
    "get_translation_with_results",
    "get_translation_segments",
    "get_translation_segment_count",
    "update_translation_status",
    "complete_translation",
    "fail_translation",
    "save_translation_paragraph",
    "save_translation_segment",
    "update_translation_segments",
    "list_translations",
    "delete_translation",
    # Profile
    "get_user_profile",
    "upsert_user_profile",
    "count_known_vocab",
    "count_learning_vocab",
    "count_total_vocab",
    # SRS
    "initialize_srs_state",
    "is_struggling",
    "compute_opacity",
    "record_lookup",
    "get_vocab_srs_info",
    "record_review_grade",
    "get_review_queue",
    "get_due_count",
    # Progress Sync
    "ProgressBundle",
    "ProgressImportError",
    "export_progress",
    "export_progress_json",
    "import_progress",
    "import_progress_json",
    "validate_progress_bundle",
]
