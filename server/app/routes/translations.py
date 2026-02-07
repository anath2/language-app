"""
Translation queue API routes.

Endpoints:
- POST /api/translations - Submit new translation
- GET /api/translations - List translations (for Translations page)
- GET /api/translations/{translation_id} - Get translation details and results
- GET /api/translations/{translation_id}/status - Quick status check
- DELETE /api/translations/{translation_id} - Delete a translation
- GET /api/translations/{translation_id}/stream - SSE stream for translation progress
"""

import asyncio
import json
from typing import Any

from fastapi import APIRouter, HTTPException
from fastapi.responses import StreamingResponse

from app.models import (
    CreateTranslationRequest,
    CreateTranslationResponse,
    OkResponse,
    ParagraphResult,
    TranslationDetailResponse,
    TranslationStatusResponse,
    TranslationSummary,
    ListTranslationsResponse,
    TranslationResult,
)
from app.persistence import (
    delete_translation,
    get_translation,
    get_translation_segment_count,
    get_translation_with_results,
    list_translations,
)
from app.queue import get_queue_manager

router = APIRouter(tags=["translations"])


def _translation_to_summary(translation: Any) -> TranslationSummary:
    """Convert a TranslationRecord to TranslationSummary."""
    # Get segment counts
    completed, total = (
        get_translation_segment_count(translation.id) if translation.status != "pending" else (None, None)
    )

    return TranslationSummary(
        id=translation.id,
        created_at=translation.created_at,
        status=translation.status,
        source_type=translation.source_type,
        input_preview=translation.input_text[:100] + "..."
        if len(translation.input_text) > 100
        else translation.input_text,
        full_translation_preview=(
            translation.full_translation[:100] + "..."
            if translation.full_translation and len(translation.full_translation) > 100
            else translation.full_translation
        ),
        segment_count=completed,
        total_segments=total,
    )


# --- JSON API Endpoints (prefix: /api) ---


@router.post("/api/translations", response_model=CreateTranslationResponse)
async def api_create_translation(request: CreateTranslationRequest):
    """
    Submit a new translation.

    The translation is created immediately with 'pending' status.
    Use GET /api/translations/{translation_id}/status to check progress.
    Use GET /api/translations/{translation_id}/stream to stream progress via SSE.
    """
    if not request.input_text.strip():
        raise HTTPException(status_code=400, detail="input_text is required")

    manager = get_queue_manager()
    translation_id = manager.submit_translation(
        input_text=request.input_text,
        source_type=request.source_type,
    )

    return CreateTranslationResponse(translation_id=translation_id, status="pending")


@router.get("/api/translations", response_model=ListTranslationsResponse)
async def api_list_translations(
    limit: int = 20,
    offset: int = 0,
    status: str | None = None,
):
    """
    List translations for the Translations page.

    Supports pagination and optional status filtering.
    """
    if status and status not in {"pending", "processing", "completed", "failed"}:
        raise HTTPException(status_code=400, detail="Invalid status filter")

    translations, total = list_translations(limit=limit, offset=offset, status=status)

    return ListTranslationsResponse(
        translations=[_translation_to_summary(t) for t in translations],
        total=total,
    )


@router.get("/api/translations/{translation_id}", response_model=TranslationDetailResponse)
async def api_get_translation(translation_id: str):
    """
    Get translation with full results.

    Returns the translation details and all translated segments organized by paragraph.
    """
    result = get_translation_with_results(translation_id)
    if result is None:
        raise HTTPException(status_code=404, detail="Translation not found")

    # Convert to response format
    paragraphs = None
    if result.paragraphs:
        paragraphs = [
            ParagraphResult(
                translations=[
                    TranslationResult(
                        segment=t["segment"],
                        pinyin=t["pinyin"],
                        english=t["english"],
                    )
                    for t in p["translations"]
                ],
                indent=p["indent"],
                separator=p["separator"],
            )
            for p in result.paragraphs
        ]

    return TranslationDetailResponse(
        id=result.translation.id,
        created_at=result.translation.created_at,
        status=result.translation.status,
        source_type=result.translation.source_type,
        input_text=result.translation.input_text,
        full_translation=result.translation.full_translation,
        error_message=result.translation.error_message,
        paragraphs=paragraphs,
    )


@router.get("/api/translations/{translation_id}/status", response_model=TranslationStatusResponse)
async def api_get_translation_status(translation_id: str):
    """
    Quick status check for a translation.

    Returns current status and progress without full results.
    """
    translation = get_translation(translation_id)
    if translation is None:
        raise HTTPException(status_code=404, detail="Translation not found")

    progress, total = (
        get_translation_segment_count(translation_id) if translation.status != "pending" else (None, None)
    )

    return TranslationStatusResponse(
        translation_id=translation_id,
        status=translation.status,
        progress=progress,
        total=total,
    )


@router.delete("/api/translations/{translation_id}", response_model=OkResponse)
async def api_delete_translation(translation_id: str):
    """Delete a translation and its results."""
    if not delete_translation(translation_id):
        raise HTTPException(status_code=404, detail="Translation not found")

    return OkResponse()


# --- SSE Streaming Endpoint ---


@router.get("/api/translations/{translation_id}/stream")
async def translation_stream(translation_id: str):
    """
    SSE stream for translation progress.

    Events:
    - start: { type: "start", translation_id, total, paragraphs }
    - progress: { type: "progress", current, total, result }
    - complete: { type: "complete", paragraphs, full_translation }
    - error: { type: "error", message }

    This endpoint starts processing the translation if it's pending.
    """

    async def generate():
        translation = get_translation(translation_id)
        if translation is None:
            yield f"data: {json.dumps({'type': 'error', 'message': 'Translation not found'})}\n\n"
            return

        manager = get_queue_manager()

        # If translation is already completed, send results immediately
        if translation.status == "completed":
            result = get_translation_with_results(translation_id)
            if result:
                # Calculate total segments
                total_segments = (
                    sum(len(p["translations"]) for p in result.paragraphs)
                    if result.paragraphs
                    else 0
                )

                # Send start event
                paragraph_info = (
                    [
                        {
                            "segment_count": len(p["translations"]),
                            "indent": p["indent"],
                            "separator": p["separator"],
                        }
                        for p in result.paragraphs
                    ]
                    if result.paragraphs
                    else []
                )

                yield f"data: {json.dumps({'type': 'start', 'translation_id': translation_id, 'total': total_segments, 'paragraphs': paragraph_info, 'fullTranslation': result.translation.full_translation})}\n\n"

                # Send all progress events at once
                global_idx = 0
                for para_idx, para in enumerate(result.paragraphs or []):
                    for seg_idx, t in enumerate(para["translations"]):
                        result_data = {
                            "segment": t["segment"],
                            "pinyin": t["pinyin"],
                            "english": t["english"],
                            "index": global_idx,
                            "paragraph_index": para_idx,
                        }
                        global_idx += 1
                        yield f"data: {json.dumps({'type': 'progress', 'current': global_idx, 'total': total_segments, 'result': result_data})}\n\n"

                # Send complete event
                yield f"data: {json.dumps({'type': 'complete', 'paragraphs': result.paragraphs, 'fullTranslation': result.translation.full_translation})}\n\n"
            return

        # If translation failed, send error
        if translation.status == "failed":
            yield f"data: {json.dumps({'type': 'error', 'message': translation.error_message or 'Translation failed'})}\n\n"
            return

        # Start processing if pending
        if translation.status == "pending":
            # Use a queue to collect progress updates
            progress_queue: asyncio.Queue = asyncio.Queue()

            def progress_callback(tid: str, seg_result):
                # Put progress update in queue
                try:
                    loop = asyncio.get_event_loop()
                    loop.call_soon_threadsafe(
                        progress_queue.put_nowait,
                        {
                            "type": "progress",
                            "translation_id": tid,
                            "result": seg_result,
                        },
                    )
                except Exception:
                    pass

            # Start processing in background
            manager.start_processing(translation_id, progress_callback)

            # Wait for processing to initialize
            await asyncio.sleep(0.5)

        # Poll for progress updates
        last_progress = 0
        sent_start = False

        while True:
            # Get current progress from manager
            progress = manager.get_progress(translation_id)
            if progress is None:
                # Translation might be done, check DB
                translation = get_translation(translation_id)
                if translation is None:
                    yield f"data: {json.dumps({'type': 'error', 'message': 'Translation not found'})}\n\n"
                    return
                if translation.status == "completed":
                    break
                if translation.status == "failed":
                    yield f"data: {json.dumps({'type': 'error', 'message': translation.error_message or 'Translation failed'})}\n\n"
                    return
                await asyncio.sleep(0.2)
                continue

            # Send start event once we have total
            if not sent_start and progress.get("total", 0) > 0:
                total = progress["total"]
                # Build paragraph info from results so far
                yield f"data: {json.dumps({'type': 'start', 'translation_id': translation_id, 'total': total, 'paragraphs': []})}\n\n"
                sent_start = True

            # Send progress events for new results
            current = progress.get("current", 0)
            results = progress.get("results", [])

            for i in range(last_progress, current):
                if i < len(results):
                    seg = results[i]
                    result_data = {
                        "segment": seg.segment,
                        "pinyin": seg.pinyin,
                        "english": seg.english,
                        "index": seg.global_idx,
                        "paragraph_index": seg.paragraph_idx,
                    }
                    yield f"data: {json.dumps({'type': 'progress', 'current': i + 1, 'total': progress.get('total', 0), 'result': result_data})}\n\n"

            last_progress = current

            # Check if completed
            if progress.get("status") == "completed":
                break

            if progress.get("status") == "failed":
                yield f"data: {json.dumps({'type': 'error', 'message': progress.get('error', 'Translation failed')})}\n\n"
                return

            await asyncio.sleep(0.1)

        # Send complete event
        result = get_translation_with_results(translation_id)
        if result:
            yield f"data: {json.dumps({'type': 'complete', 'paragraphs': result.paragraphs, 'fullTranslation': result.translation.full_translation})}\n\n"

        # Cleanup progress tracking
        manager.cleanup_progress(translation_id)

    return StreamingResponse(generate(), media_type="text/event-stream")
