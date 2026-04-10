"""High-level Python API for the OpenParallax channels module."""

from __future__ import annotations

from typing import Any

from openparallax_channels.bridge import BridgeProcess
from openparallax_channels.types import ChannelMessage


class Channels:
    """Message formatting and splitting utilities for channel adapters.

    Example::

        from openparallax_channels import Channels

        with Channels() as ch:
            parts = ch.split_message(long_text, max_length=2000)
            msg = ch.format_message("Hello!", format=1)
    """

    def __init__(self) -> None:
        self._bridge = BridgeProcess("channels-bridge")

    def split_message(self, content: str, max_length: int = 4096) -> list[str]:
        """Split a long message into chunks respecting the character limit.

        Returns a list of string parts.
        """
        result = self._bridge.call("split_message", {
            "content": content,
            "max_length": max_length,
        })
        return result if result else []

    def format_message(self, text: str, format: int = 0) -> ChannelMessage:
        """Format a message for a specific channel format.

        Returns a ChannelMessage with the formatted text.
        """
        result = self._bridge.call("format_message", {
            "text": text,
            "format": format,
        })
        return ChannelMessage.from_dict(result) if result else ChannelMessage(text=text)

    def close(self) -> None:
        """Terminate the bridge process."""
        self._bridge.close()

    def __enter__(self) -> Channels:
        return self

    def __exit__(self, *_: Any) -> None:
        self.close()
