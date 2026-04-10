"""Type definitions for the OpenParallax channels wrapper."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any


@dataclass
class ChannelMessage:
    """A formatted channel message."""

    text: str = ""
    format: int = 0

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> ChannelMessage:
        return cls(
            text=data.get("Text", data.get("text", "")),
            format=data.get("Format", data.get("format", 0)),
        )
