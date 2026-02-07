"""
Job queue API routes.

Endpoints:
- POST /api/jobs - Submit new translation job
- GET /api/jobs - List jobs (for Translations page)
- GET /api/jobs/{job_id} - Get job details and results
- GET /api/jobs/{job_id}/status - Quick status check
- DELETE /api/jobs/{job_id} - Delete a job
- GET /api/translations/{translation_id}/stream - SSE stream for translation progress
"""

import asyncio
import json
from typing import Any

from fastapi import APIRouter, HTTPException
from fastapi.responses import StreamingResponse

from app.models import (
    CreateJobRequest,
    CreateJobResponse,
    JobDetailResponse,
    JobStatusResponse,
    JobSummary,
    ListJobsResponse,
    OkResponse,
    ParagraphResult,
    TranslationResult,
)
from app.persistence import (
    delete_job,
    get_job,
    get_job_segment_count,
    get_job_with_results,
    list_jobs,
)
from app.queue import get_queue_manager

router = APIRouter(tags=["jobs"])


def _job_to_summary(job: Any) -> JobSummary:
    """Convert a JobRecord to JobSummary."""
    # Get segment counts
    completed, total = (
        get_job_segment_count(job.id) if job.status != "pending" else (None, None)
    )

    return JobSummary(
        id=job.id,
        created_at=job.created_at,
        status=job.status,
        source_type=job.source_type,
        input_preview=job.input_text[:100] + "..."
        if len(job.input_text) > 100
        else job.input_text,
        full_translation_preview=(
            job.full_translation[:100] + "..."
            if job.full_translation and len(job.full_translation) > 100
            else job.full_translation
        ),
        segment_count=completed,
        total_segments=total,
    )


# --- JSON API Endpoints (prefix: /api) ---


@router.post("/api/jobs", response_model=CreateJobResponse)
async def api_create_job(request: CreateJobRequest):
    """
    Submit a new translation job.

    The job is created immediately with 'pending' status.
    Use GET /api/jobs/{job_id}/status to check progress.
    Use GET /api/translations/{translation_id}/stream to stream progress via SSE.
    """
    if not request.input_text.strip():
        raise HTTPException(status_code=400, detail="input_text is required")

    manager = get_queue_manager()
    job_id = manager.submit_job(
        input_text=request.input_text,
        source_type=request.source_type,
    )

    return CreateJobResponse(job_id=job_id, status="pending")


@router.get("/api/jobs", response_model=ListJobsResponse)
async def api_list_jobs(
    limit: int = 20,
    offset: int = 0,
    status: str | None = None,
):
    """
    List translation jobs for the Translations page.

    Supports pagination and optional status filtering.
    """
    if status and status not in {"pending", "processing", "completed", "failed"}:
        raise HTTPException(status_code=400, detail="Invalid status filter")

    jobs, total = list_jobs(limit=limit, offset=offset, status=status)

    return ListJobsResponse(
        jobs=[_job_to_summary(job) for job in jobs],
        total=total,
    )


@router.get("/api/jobs/{job_id}", response_model=JobDetailResponse)
async def api_get_job(job_id: str):
    """
    Get job with full results.

    Returns the job details and all translated segments organized by paragraph.
    """
    result = get_job_with_results(job_id)
    if result is None:
        raise HTTPException(status_code=404, detail="Job not found")

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

    return JobDetailResponse(
        id=result.job.id,
        created_at=result.job.created_at,
        status=result.job.status,
        source_type=result.job.source_type,
        input_text=result.job.input_text,
        full_translation=result.job.full_translation,
        error_message=result.job.error_message,
        paragraphs=paragraphs,
    )


@router.get("/api/jobs/{job_id}/status", response_model=JobStatusResponse)
async def api_get_job_status(job_id: str):
    """
    Quick status check for a job.

    Returns current status and progress without full results.
    """
    job = get_job(job_id)
    if job is None:
        raise HTTPException(status_code=404, detail="Job not found")

    progress, total = (
        get_job_segment_count(job_id) if job.status != "pending" else (None, None)
    )

    return JobStatusResponse(
        job_id=job_id,
        status=job.status,
        progress=progress,
        total=total,
    )


@router.delete("/api/jobs/{job_id}", response_model=OkResponse)
async def api_delete_job(job_id: str):
    """Delete a job and its results."""
    if not delete_job(job_id):
        raise HTTPException(status_code=404, detail="Job not found")

    return OkResponse()


# --- SSE Streaming Endpoint ---


@router.get("/api/translations/{job_id}/stream")
async def translation_stream(job_id: str):
    """
    SSE stream for job progress.

    Events:
    - start: { type: "start", job_id, total, paragraphs }
    - progress: { type: "progress", current, total, result }
    - complete: { type: "complete", paragraphs, full_translation }
    - error: { type: "error", message }

    This endpoint starts processing the job if it's pending.
    """

    async def generate():
        job = get_job(job_id)
        if job is None:
            yield f"data: {json.dumps({'type': 'error', 'message': 'Job not found'})}\n\n"
            return

        manager = get_queue_manager()

        # If job is already completed, send results immediately
        if job.status == "completed":
            result = get_job_with_results(job_id)
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

                yield f"data: {json.dumps({'type': 'start', 'job_id': job_id, 'total': total_segments, 'paragraphs': paragraph_info, 'fullTranslation': result.job.full_translation})}\n\n"

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
                yield f"data: {json.dumps({'type': 'complete', 'paragraphs': result.paragraphs, 'fullTranslation': result.job.full_translation})}\n\n"
            return

        # If job failed, send error
        if job.status == "failed":
            yield f"data: {json.dumps({'type': 'error', 'message': job.error_message or 'Job failed'})}\n\n"
            return

        # Start processing if pending
        if job.status == "pending":
            # Use a queue to collect progress updates
            progress_queue: asyncio.Queue = asyncio.Queue()

            def progress_callback(jid: str, seg_result):
                # Put progress update in queue
                try:
                    loop = asyncio.get_event_loop()
                    loop.call_soon_threadsafe(
                        progress_queue.put_nowait,
                        {
                            "type": "progress",
                            "job_id": jid,
                            "result": seg_result,
                        },
                    )
                except Exception:
                    pass

            # Start processing in background
            manager.start_processing(job_id, progress_callback)

            # Wait for processing to initialize
            await asyncio.sleep(0.5)

        # Poll for progress updates
        last_progress = 0
        sent_start = False

        while True:
            # Get current progress from manager
            progress = manager.get_progress(job_id)
            if progress is None:
                # Job might be done, check DB
                job = get_job(job_id)
                if job is None:
                    yield f"data: {json.dumps({'type': 'error', 'message': 'Job not found'})}\n\n"
                    return
                if job.status == "completed":
                    break
                if job.status == "failed":
                    yield f"data: {json.dumps({'type': 'error', 'message': job.error_message or 'Job failed'})}\n\n"
                    return
                await asyncio.sleep(0.2)
                continue

            # Send start event once we have total
            if not sent_start and progress.get("total", 0) > 0:
                total = progress["total"]
                # Build paragraph info from results so far
                yield f"data: {json.dumps({'type': 'start', 'job_id': job_id, 'total': total, 'paragraphs': []})}\n\n"
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
                yield f"data: {json.dumps({'type': 'error', 'message': progress.get('error', 'Job failed')})}\n\n"
                return

            await asyncio.sleep(0.1)

        # Send complete event
        result = get_job_with_results(job_id)
        if result:
            yield f"data: {json.dumps({'type': 'complete', 'paragraphs': result.paragraphs, 'fullTranslation': result.job.full_translation})}\n\n"

        # Cleanup progress tracking
        manager.cleanup_progress(job_id)

    return StreamingResponse(generate(), media_type="text/event-stream")
