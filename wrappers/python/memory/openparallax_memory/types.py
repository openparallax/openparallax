"""Type definitions for the OpenParallax memory wrapper."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any


@dataclass
class SearchResult:
    """A single memory search result."""

    path: str = ""
    section: str = ""
    snippet: str = ""
    score: float = 0.0

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> SearchResult:
        return cls(
            path=data.get("Path", data.get("path", "")),
            section=data.get("Section", data.get("section", "")),
            snippet=data.get("Snippet", data.get("snippet", "")),
            score=data.get("Score", data.get("score", 0.0)),
        )
