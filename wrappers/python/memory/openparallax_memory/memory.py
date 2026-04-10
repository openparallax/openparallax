"""High-level Python API for the OpenParallax memory module."""

from __future__ import annotations

from typing import Any

from openparallax_memory.bridge import BridgeProcess
from openparallax_memory.types import SearchResult


class Memory:
    """Workspace memory: file reading, FTS5 search, and daily logs.

    Example::

        from openparallax_memory import Memory

        with Memory("/path/to/workspace", "/path/to/db") as mem:
            results = mem.search("deployment process")
            content = mem.read("MEMORY.md")
    """

    def __init__(self, workspace: str | None = None, db_path: str | None = None) -> None:
        self._bridge = BridgeProcess("memory-bridge")
        if workspace and db_path:
            self.configure(workspace, db_path)

    def configure(self, workspace: str, db_path: str) -> None:
        """Initialize the memory manager with workspace and database paths."""
        self._bridge.call("configure", {"workspace": workspace, "db_path": db_path})

    def search(self, query: str, limit: int = 10) -> list[SearchResult]:
        """Search memory files using FTS5 full-text search.

        Returns a list of SearchResult objects ranked by relevance.
        """
        result = self._bridge.call("search", {"query": query, "limit": limit})
        if not result:
            return []
        return [SearchResult.from_dict(r) for r in result]

    def read(self, file_type: str) -> str:
        """Read a workspace memory file by type (e.g. 'MEMORY.md', 'SOUL.md')."""
        result = self._bridge.call("read", {"file_type": file_type})
        return result.get("content", "") if result else ""

    def close(self) -> None:
        """Terminate the bridge process."""
        self._bridge.close()

    def __enter__(self) -> Memory:
        return self

    def __exit__(self, *_: Any) -> None:
        self.close()
