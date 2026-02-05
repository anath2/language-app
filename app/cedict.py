"""
CC-CEDICT dictionary parser and lookup.

Parses the CC-CEDICT Chinese-English dictionary file and provides
lookup functionality for Chinese words.

File format: Traditional Simplified [pinyin] /def1/def2/.../
Example: 感覺 感觉 [gan3 jue2] /feeling; impression; sensation/to feel; to perceive/
"""

import re
from pathlib import Path

# Regex to parse CC-CEDICT entries
# Format: Traditional Simplified [pinyin] /definitions/
ENTRY_PATTERN = re.compile(r"^(\S+)\s+(\S+)\s+\[([^\]]+)\]\s+/(.+)/$")

# Type alias for the dictionary
CedictDict = dict[str, list[str]]


def _get_cedict_path() -> Path:
    """Get the path to the CC-CEDICT file."""
    return Path(__file__).parent / "data" / "cedict_ts.u8"


def load_cedict(path: Path | None = None) -> CedictDict:
    """
    Load and parse the CC-CEDICT dictionary file.

    Returns a dictionary mapping simplified Chinese words to their definitions.
    Words with multiple pronunciations will have all definitions combined.

    Args:
        path: Optional path to the dictionary file. Defaults to app/data/cedict_ts.u8

    Returns:
        Dictionary mapping simplified Chinese to list of definition strings.
    """
    if path is None:
        path = _get_cedict_path()

    cedict: CedictDict = {}

    with open(path, encoding="utf-8") as f:
        for line in f:
            # Skip comments and empty lines
            if line.startswith("#") or line.startswith("%") or not line.strip():
                continue

            match = ENTRY_PATTERN.match(line.strip())
            if not match:
                continue

            # Extract components
            _traditional, simplified, _pinyin, definitions = match.groups()

            # Split definitions by /
            defs = [d.strip() for d in definitions.split("/") if d.strip()]

            # Add to dictionary (combine if word already exists with different pinyin)
            if simplified in cedict:
                # Avoid duplicates
                for d in defs:
                    if d not in cedict[simplified]:
                        cedict[simplified].append(d)
            else:
                cedict[simplified] = defs

    return cedict


def lookup(cedict: CedictDict, word: str) -> str | None:
    """
    Look up a Chinese word in the dictionary.

    Args:
        cedict: The loaded dictionary from load_cedict()
        word: The simplified Chinese word to look up

    Returns:
        Definitions joined by " / ", or None if not found.
        Example: "feeling; impression; sensation / to feel; to perceive"
    """
    defs = cedict.get(word)
    if defs:
        return " / ".join(defs)
    return None
