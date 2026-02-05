"""
Job queue package for background translation processing.

This package provides:
- JobQueueManager: Thread pool manager with rate limiting
- RateLimiter: Coordinated rate limiting for LLM API calls
"""

from threading import Lock

from app.queue.manager import JobQueueManager, RateLimiter

# Thread-safe singleton
_queue_manager_lock = Lock()
_queue_manager: JobQueueManager | None = None


def get_queue_manager() -> JobQueueManager:
    """Thread-safe lazy initialization of job queue manager."""
    global _queue_manager
    if _queue_manager is None:
        with _queue_manager_lock:
            if _queue_manager is None:
                _queue_manager = JobQueueManager()
    return _queue_manager


__all__ = [
    "JobQueueManager",
    "RateLimiter",
    "get_queue_manager",
]
