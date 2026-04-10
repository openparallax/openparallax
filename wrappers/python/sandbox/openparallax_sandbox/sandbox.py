"""High-level Python API for the OpenParallax sandbox module."""

from __future__ import annotations

from typing import Any

from openparallax_sandbox.bridge import BridgeProcess


class Sandbox:
    """Kernel-level process isolation verification.

    Example::

        from openparallax_sandbox import Sandbox

        with Sandbox() as sb:
            result = sb.verify_canary()
            print(f"Sandbox active: {result}")
    """

    def __init__(self) -> None:
        self._bridge = BridgeProcess("sandbox-bridge")

    def verify_canary(self) -> Any:
        """Verify the sandbox canary.

        Returns the canary verification result from the Go sandbox module.
        """
        return self._bridge.call("verify_canary")

    def close(self) -> None:
        """Terminate the bridge process."""
        self._bridge.close()

    def __enter__(self) -> Sandbox:
        return self

    def __exit__(self, *_: Any) -> None:
        self.close()
