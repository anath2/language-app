"""
Job queue manager with thread pool for rate-limited LLM processing.

This module provides:
- RateLimiter: Coordinated rate limiting across threads
- JobQueueManager: Thread pool for processing translation jobs
"""

import asyncio
import os
import time
from concurrent.futures import ThreadPoolExecutor
from dataclasses import dataclass
from threading import Lock
from typing import Any, Callable

from app.cedict import lookup
from app.persistence.jobs import (
    complete_job,
    create_job,
    fail_job,
    get_job,
    save_job_paragraph,
    save_job_segment,
    update_job_status,
)
from app.pipeline import get_full_translator, get_pipeline
from app.utils import should_skip_segment, split_into_paragraphs, to_pinyin


# Configuration from environment
JOB_QUEUE_WORKERS = int(os.getenv("JOB_QUEUE_WORKERS", "2"))
JOB_QUEUE_RATE_LIMIT_MS = int(os.getenv("JOB_QUEUE_RATE_LIMIT_MS", "500"))


@dataclass
class SegmentResult:
    """Result of translating a single segment."""

    paragraph_idx: int
    seg_idx: int
    global_idx: int
    segment: str
    pinyin: str
    english: str


class RateLimiter:
    """
    Round-robin rate limiter for LLM API calls.

    Ensures minimum delay between requests across all workers.
    Uses a global lock to coordinate between threads.
    """

    def __init__(self, min_delay_ms: int = 500):
        self._min_delay = min_delay_ms / 1000.0
        self._last_request = 0.0
        self._lock = Lock()

    def wait(self) -> None:
        """Block until rate limit allows next request."""
        with self._lock:
            now = time.time()
            elapsed = now - self._last_request
            if elapsed < self._min_delay:
                time.sleep(self._min_delay - elapsed)
            self._last_request = time.time()


class JobQueueManager:
    """
    Manages a thread pool for processing translation jobs.

    Features:
    - Thread pool with configurable worker count
    - Round-robin rate limit management
    - Job persistence to SQLite
    - Progress callbacks for SSE streaming
    """

    def __init__(
        self,
        max_workers: int = JOB_QUEUE_WORKERS,
        rate_limit_ms: int = JOB_QUEUE_RATE_LIMIT_MS,
    ):
        self._executor = ThreadPoolExecutor(max_workers=max_workers)
        self._rate_limiter = RateLimiter(rate_limit_ms)
        self._active_jobs: dict[str, asyncio.Event] = {}
        self._job_progress: dict[str, dict[str, Any]] = {}
        self._lock = Lock()

    def submit_job(
        self,
        input_text: str,
        source_type: str = "text",
    ) -> str:
        """
        Create and queue a new translation job.

        Returns job_id immediately. Processing happens asynchronously.
        """
        job_id = create_job(input_text=input_text, source_type=source_type)

        # Initialize progress tracking
        with self._lock:
            self._job_progress[job_id] = {
                "status": "pending",
                "current": 0,
                "total": 0,
                "results": [],
            }

        return job_id

    def start_processing(
        self,
        job_id: str,
        progress_callback: Callable[[str, SegmentResult], None] | None = None,
    ) -> None:
        """
        Start processing a job in the thread pool.

        progress_callback is called with (job_id, segment_result) for each segment.
        """
        self._executor.submit(self._process_job, job_id, progress_callback)

    def get_progress(self, job_id: str) -> dict[str, Any] | None:
        """Get current progress for a job."""
        with self._lock:
            return self._job_progress.get(job_id)

    def _process_job(
        self,
        job_id: str,
        progress_callback: Callable[[str, SegmentResult], None] | None = None,
    ) -> None:
        """
        Worker function that processes a single job.

        1. Load job from database
        2. Mark as processing
        3. Run segmentation for all paragraphs
        4. Run full-text translation
        5. For each segment: rate_limit() then translate
        6. Save results to job_segments
        7. Mark as completed (or failed on error)
        """
        try:
            # Mark as processing
            update_job_status(job_id, "processing")
            with self._lock:
                if job_id in self._job_progress:
                    self._job_progress[job_id]["status"] = "processing"

            job = get_job(job_id)
            if job is None:
                raise ValueError(f"Job {job_id} not found")

            pipe = get_pipeline()
            full_translator = get_full_translator()

            # Split text into paragraphs
            paragraphs = split_into_paragraphs(job.input_text)

            # Rate limit before full translation
            self._rate_limiter.wait()

            # Get full translation (run in thread-safe manner)
            loop = asyncio.new_event_loop()
            asyncio.set_event_loop(loop)
            try:
                full_result = loop.run_until_complete(
                    full_translator.acall(text=job.input_text)
                )
                full_translation = full_result.english
            finally:
                loop.close()

            # Segment all paragraphs first
            all_paragraph_segments: list[dict[str, Any]] = []
            for para in paragraphs:
                self._rate_limiter.wait()

                loop = asyncio.new_event_loop()
                asyncio.set_event_loop(loop)
                try:
                    segmentation = loop.run_until_complete(
                        pipe.segment.acall(text=para["content"])
                    )
                finally:
                    loop.close()

                all_paragraph_segments.append(
                    {
                        "segments": segmentation.segments,
                        "indent": para.get("indent", ""),
                        "separator": para["separator"],
                        "content": para["content"],
                    }
                )

            # Calculate total segments
            total_segments = sum(len(p["segments"]) for p in all_paragraph_segments)

            with self._lock:
                if job_id in self._job_progress:
                    self._job_progress[job_id]["total"] = total_segments

            # Save paragraph metadata
            for para_idx, para_data in enumerate(all_paragraph_segments):
                save_job_paragraph(
                    job_id=job_id,
                    paragraph_idx=para_idx,
                    indent=para_data["indent"],
                    separator=para_data["separator"],
                )

            # Translate each segment
            global_idx = 0
            for para_idx, para_data in enumerate(all_paragraph_segments):
                for seg_idx, segment in enumerate(para_data["segments"]):
                    if should_skip_segment(segment):
                        pinyin = ""
                        english = ""
                    else:
                        # Rate limit before LLM call
                        self._rate_limiter.wait()

                        pinyin = to_pinyin(segment)
                        dict_entry = lookup(pipe.cedict, segment) or "Not in dictionary"

                        loop = asyncio.new_event_loop()
                        asyncio.set_event_loop(loop)
                        try:
                            translation = loop.run_until_complete(
                                pipe.translate.acall(
                                    segment=segment,
                                    sentence_context=para_data["content"],
                                    dictionary_entry=dict_entry,
                                )
                            )
                            english = translation.english
                        finally:
                            loop.close()

                    # Save segment result
                    save_job_segment(
                        job_id=job_id,
                        paragraph_idx=para_idx,
                        seg_idx=seg_idx,
                        segment_text=segment,
                        pinyin=pinyin,
                        english=english,
                    )

                    # Create result object
                    result = SegmentResult(
                        paragraph_idx=para_idx,
                        seg_idx=seg_idx,
                        global_idx=global_idx,
                        segment=segment,
                        pinyin=pinyin,
                        english=english,
                    )

                    # Update progress
                    global_idx += 1
                    with self._lock:
                        if job_id in self._job_progress:
                            self._job_progress[job_id]["current"] = global_idx
                            self._job_progress[job_id]["results"].append(result)

                    # Call progress callback if provided
                    if progress_callback:
                        progress_callback(job_id, result)

            # Mark as completed
            complete_job(job_id, full_translation)
            with self._lock:
                if job_id in self._job_progress:
                    self._job_progress[job_id]["status"] = "completed"
                    self._job_progress[job_id]["full_translation"] = full_translation

        except Exception as e:
            # Mark as failed
            fail_job(job_id, str(e))
            with self._lock:
                if job_id in self._job_progress:
                    self._job_progress[job_id]["status"] = "failed"
                    self._job_progress[job_id]["error"] = str(e)
            raise

    def cleanup_progress(self, job_id: str) -> None:
        """Remove progress tracking for a completed job."""
        with self._lock:
            self._job_progress.pop(job_id, None)

    def shutdown(self, wait: bool = True) -> None:
        """Shutdown the thread pool."""
        self._executor.shutdown(wait=wait)
