"""
REST API routes for persistence and SRS.

Endpoints:
- POST /api/texts - Create text
- GET /api/texts/{text_id} - Get text
- POST /api/events - Create event
- POST /api/vocab/save - Save vocabulary item
- POST /api/vocab/status - Update vocabulary status
- POST /api/vocab/lookup - Record lookup event
- GET /api/vocab/srs-info - Get SRS info for headwords
- GET /api/review/queue - Get review queue
- POST /api/review/answer - Record review answer
- GET /api/review/count - Get due count
"""

from fastapi import APIRouter, HTTPException

from app.models import (
    CreateEventRequest,
    CreateEventResponse,
    CreateTextRequest,
    CreateTextResponse,
    DueCountResponse,
    OkResponse,
    RecordLookupRequest,
    RecordLookupResponse,
    ReviewAnswerRequest,
    ReviewAnswerResponse,
    ReviewCardResponse,
    ReviewQueueResponse,
    SaveVocabRequest,
    SaveVocabResponse,
    TextResponse,
    TranslateBatchRequest,
    TranslateBatchResponse,
    TranslationResult,
    UpdateVocabStatusRequest,
    VocabSRSInfoListResponse,
    VocabSRSInfoResponse,
)
from app.persistence import (
    create_event,
    create_text,
    get_due_count,
    get_review_queue,
    get_text,
    get_vocab_srs_info,
    record_lookup,
    record_review_grade,
    save_vocab_item,
    update_vocab_status,
)

router = APIRouter(prefix="/api", tags=["api"])


# --- Text Persistence ---


@router.post("/texts", response_model=CreateTextResponse)
async def api_create_text(request: CreateTextRequest):
    if not request.raw_text.strip():
        raise HTTPException(status_code=400, detail="raw_text is required")
    record = create_text(
        raw_text=request.raw_text, source_type=request.source_type, metadata=request.metadata
    )
    return CreateTextResponse(id=record.id)


@router.get("/texts/{text_id}", response_model=TextResponse)
async def api_get_text(text_id: str):
    record = get_text(text_id)
    if record is None:
        raise HTTPException(status_code=404, detail="Not found")
    return TextResponse(
        id=record.id,
        created_at=record.created_at,
        source_type=record.source_type,
        raw_text=record.raw_text,
        normalized_text=record.normalized_text,
        metadata=record.metadata,
    )


# --- Events ---


@router.post("/events", response_model=CreateEventResponse)
async def api_create_event(request: CreateEventRequest):
    if not request.event_type.strip():
        raise HTTPException(status_code=400, detail="event_type is required")
    event_id = create_event(
        event_type=request.event_type,
        text_id=request.text_id,
        segment_id=request.segment_id,
        payload=request.payload,
    )
    return CreateEventResponse(id=event_id)


# --- Vocabulary ---


@router.post("/vocab/save", response_model=SaveVocabResponse)
async def api_save_vocab(request: SaveVocabRequest):
    if not request.headword.strip():
        raise HTTPException(status_code=400, detail="headword is required")
    if request.status not in {"unknown", "learning", "known"}:
        raise HTTPException(status_code=400, detail="Invalid status")
    vocab_item_id = save_vocab_item(
        headword=request.headword.strip(),
        pinyin=request.pinyin.strip(),
        english=request.english.strip(),
        text_id=request.text_id,
        segment_id=request.segment_id,
        snippet=request.snippet,
        status=request.status,
    )
    return SaveVocabResponse(vocab_item_id=vocab_item_id)


@router.post("/vocab/status", response_model=OkResponse)
async def api_update_vocab_status(request: UpdateVocabStatusRequest):
    try:
        update_vocab_status(vocab_item_id=request.vocab_item_id, status=request.status)
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e)) from e
    return OkResponse()


# --- SRS API ---


@router.post("/vocab/lookup", response_model=RecordLookupResponse)
async def api_record_lookup(request: RecordLookupRequest):
    """Record a passive lookup event for a vocab item."""
    result = record_lookup(request.vocab_item_id)
    if result is None:
        raise HTTPException(status_code=404, detail="Vocab item not found")
    return RecordLookupResponse(
        vocab_item_id=result.vocab_item_id,
        opacity=result.opacity,
        is_struggling=result.is_struggling,
    )


@router.get("/vocab/srs-info", response_model=VocabSRSInfoListResponse)
async def api_get_vocab_srs_info(headwords: str):
    """Get SRS info for a comma-separated list of headwords."""
    if not headwords.strip():
        return VocabSRSInfoListResponse(items=[])

    headword_list = [h.strip() for h in headwords.split(",") if h.strip()]
    results = get_vocab_srs_info(headword_list)

    return VocabSRSInfoListResponse(
        items=[
            VocabSRSInfoResponse(
                vocab_item_id=r.vocab_item_id,
                headword=r.headword,
                pinyin=r.pinyin,
                english=r.english,
                opacity=r.opacity,
                is_struggling=r.is_struggling,
                status=r.status,
            )
            for r in results
        ]
    )


@router.get("/review/queue", response_model=ReviewQueueResponse)
async def api_get_review_queue(limit: int = 10):
    """Get vocab items due for active review."""
    cards = get_review_queue(limit=limit)
    due_count = get_due_count()

    return ReviewQueueResponse(
        cards=[
            ReviewCardResponse(
                vocab_item_id=c.vocab_item_id,
                headword=c.headword,
                pinyin=c.pinyin,
                english=c.english,
                snippets=c.snippets,
            )
            for c in cards
        ],
        due_count=due_count,
    )


@router.post("/review/answer", response_model=ReviewAnswerResponse)
async def api_record_review_answer(request: ReviewAnswerRequest):
    """Record a review grade for a vocab item (active review)."""
    try:
        state = record_review_grade(request.vocab_item_id, grade=int(request.grade))
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e)) from e

    if state is None:
        raise HTTPException(status_code=404, detail="Vocab item not found")

    remaining = get_due_count()

    return ReviewAnswerResponse(
        vocab_item_id=state.vocab_item_id,
        next_due_at=state.due_at,
        interval_days=state.interval_days,
        remaining_due=remaining,
    )


@router.get("/review/count", response_model=DueCountResponse)
async def api_get_due_count():
    """Get count of vocab items due for review."""
    return DueCountResponse(due_count=get_due_count())


# --- Segment Editing ---


@router.post("/segments/translate-batch", response_model=TranslateBatchResponse)
async def api_translate_batch(request: TranslateBatchRequest):
    """
    Translate a batch of segment texts.

    Used after split/join operations to get proper translations for modified segments.
    Uses pypinyin for pinyin generation, CEDICT for dictionary lookup,
    and falls back to LLM for words not in CEDICT.
    """
    from app.cedict import lookup
    from app.pipeline import get_pipeline
    from app.utils import should_skip_segment, to_pinyin

    pipe = get_pipeline()
    translations = []

    for segment_text in request.segments:
        if should_skip_segment(segment_text):
            translations.append(
                TranslationResult(segment=segment_text, pinyin="", english="")
            )
            continue

        pinyin = to_pinyin(segment_text)
        english = lookup(pipe.cedict, segment_text)

        if not english:
            # LLM fallback for CEDICT miss
            result = await pipe.translate.acall(  # type: ignore[attr-defined]
                segment=segment_text,
                sentence_context=request.context or segment_text,
                dictionary_entry="Not in dictionary",
            )
            english = result.english

        translations.append(
            TranslationResult(segment=segment_text, pinyin=pinyin, english=english)
        )

    return TranslateBatchResponse(translations=translations)
