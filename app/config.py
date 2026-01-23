"""
Application configuration and LLM setup.

This module handles:
- DSPy LM configuration with OpenRouter
- Application constants (file size limits, allowed extensions)
- Chinese punctuation definitions
"""

import os

import dspy
from dotenv import load_dotenv

# Load environment variables
load_dotenv()

# Configure DSPy with OpenRouter
openrouter_model = os.getenv("OPENROUTER_MODEL")
openrouter_api_key = os.getenv("OPENROUTER_API_KEY")

if not openrouter_api_key:
    raise ValueError("OPENROUTER_API_KEY environment variable is required")

lm = dspy.LM(
    model=f"openrouter/{openrouter_model}",
    api_key=openrouter_api_key,
)
dspy.configure(lm=lm)

# Image validation constants
MAX_FILE_SIZE = 5 * 1024 * 1024  # 5MB
ALLOWED_EXTENSIONS = {".png", ".jpg", ".jpeg", ".webp", ".gif"}

# Chinese punctuation marks
CHINESE_PUNCTUATION = "。，、；：？！\"\"''（）【】《》…—·「」『』〈〉〔〕"
