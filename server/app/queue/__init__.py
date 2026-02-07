"""
Translation queue package for background translation processing.

This package provides:
- TranslationQueueManager: Thread pool manager with rate limiting
- RateLimiter: Coordinated rate limiting for LLM API calls
"""

from threading import Lock

from app.queue.manager import RateLimiter, TranslationQueueManager

# Thread-safe singleton
_queue_manager_lock = Lock()
_queue_manager: TranslationQueueManager | None = None


def get_queue_manager() -> TranslationQueueManager:
    """Thread-safe lazy initialization of translation queue manager."""
    global _queue_manager
    if _queue_manager is None:
        with _queue_manager_lock:
            if _queue_manager is None:
                _queue_manager = TranslationQueueManager()
    return _queue_manager


__all__ = [
    "TranslationQueueManager",
    "RateLimiter",
    "get_queue_manager",
]
