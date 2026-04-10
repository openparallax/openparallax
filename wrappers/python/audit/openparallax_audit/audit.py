"""High-level Python API for the OpenParallax audit module."""

from __future__ import annotations

from typing import Any

from openparallax_audit.bridge import BridgeProcess
from openparallax_audit.types import Entry, LogEntry, Query


class Audit:
    """Append-only JSONL audit log with SHA-256 hash chain.

    Example::

        from openparallax_audit import Audit, Entry

        with Audit("audit.jsonl") as audit:
            audit.log(Entry(event_type=1, action_type="read_file"))
            valid = audit.verify()
    """

    def __init__(self, path: str | None = None) -> None:
        self._bridge = BridgeProcess("audit-bridge")
        if path:
            self.configure(path)

    def configure(self, path: str) -> None:
        """Initialize the audit logger with the given file path."""
        self._bridge.call("configure", {"path": path})

    def log(self, entry: Entry) -> None:
        """Append an audit entry to the log."""
        self._bridge.call("log", entry.to_dict())

    def verify(self, path: str | None = None) -> dict[str, Any]:
        """Verify the integrity of an audit log file.

        Returns a dict with 'valid' (bool) and optionally 'error' (str).
        """
        params: dict[str, Any] = {}
        if path:
            params["path"] = path
        return self._bridge.call("verify", params)

    def query(self, path: str, query: Query | None = None) -> list[LogEntry]:
        """Query audit log entries.

        Returns a list of LogEntry objects matching the query.
        """
        params: dict[str, Any] = {"path": path}
        if query:
            params["query"] = query.to_dict()
        result = self._bridge.call("query", params)
        if not result:
            return []
        return [LogEntry.from_dict(e) for e in result]

    def close(self) -> None:
        """Terminate the bridge process."""
        self._bridge.close()

    def __enter__(self) -> Audit:
        return self

    def __exit__(self, *_: Any) -> None:
        self.close()
