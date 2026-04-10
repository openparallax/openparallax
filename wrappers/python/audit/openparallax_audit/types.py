"""Type definitions for the OpenParallax audit wrapper."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any


@dataclass
class Entry:
    """An audit event to be logged."""

    event_type: int
    action_type: str = ""
    session_id: str = ""
    details: str = ""
    otr: bool = False
    source: str = ""

    def to_dict(self) -> dict[str, Any]:
        return {
            "EventType": self.event_type,
            "ActionType": self.action_type,
            "SessionID": self.session_id,
            "Details": self.details,
            "OTR": self.otr,
            "Source": self.source,
        }


@dataclass
class LogEntry:
    """A stored audit log entry."""

    id: int = 0
    timestamp: str = ""
    event_type: int = 0
    action_type: str = ""
    session_id: str = ""
    details: str = ""
    hash: str = ""
    prev_hash: str = ""

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> LogEntry:
        return cls(
            id=data.get("ID", 0),
            timestamp=data.get("Timestamp", ""),
            event_type=data.get("EventType", 0),
            action_type=data.get("ActionType", ""),
            session_id=data.get("SessionID", ""),
            details=data.get("Details", ""),
            hash=data.get("Hash", ""),
            prev_hash=data.get("PrevHash", ""),
        )


@dataclass
class Query:
    """Audit log query parameters."""

    session_id: str = ""
    event_type: int = 0
    action_type: str = ""
    limit: int = 100

    def to_dict(self) -> dict[str, Any]:
        d: dict[str, Any] = {"Limit": self.limit}
        if self.session_id:
            d["SessionID"] = self.session_id
        if self.event_type:
            d["EventType"] = self.event_type
        if self.action_type:
            d["ActionType"] = self.action_type
        return d
