"""JSON-RPC bridge to the Go memory-bridge binary."""

from __future__ import annotations

import json
import platform
import subprocess
import sys
import threading
from pathlib import Path
from typing import Any


class BridgeProcess:
    """Manages a Go bridge binary subprocess communicating via JSON-RPC over stdio."""

    def __init__(self, binary_name: str) -> None:
        self._binary = self._find_binary(binary_name)
        self._proc: subprocess.Popen[bytes] | None = None
        self._lock = threading.Lock()
        self._id = 0
        self._start()

    def call(self, method: str, params: Any = None) -> Any:
        """Send a JSON-RPC request and return the result."""
        with self._lock:
            if self._proc is None or self._proc.poll() is not None:
                self._start()

            self._id += 1
            request: dict[str, Any] = {
                "jsonrpc": "2.0",
                "id": self._id,
                "method": method,
            }
            if params is not None:
                request["params"] = params

            line = json.dumps(request) + "\n"
            assert self._proc is not None
            assert self._proc.stdin is not None
            assert self._proc.stdout is not None

            self._proc.stdin.write(line.encode())
            self._proc.stdin.flush()

            response_line = self._proc.stdout.readline()
            if not response_line:
                raise ConnectionError("bridge process closed unexpectedly")

            response = json.loads(response_line)
            if "error" in response and response["error"] is not None:
                raise BridgeError(
                    response["error"].get("message", "unknown error"),
                    response["error"].get("code", -1),
                )
            return response.get("result")

    def close(self) -> None:
        """Terminate the bridge process."""
        with self._lock:
            if self._proc is not None:
                self._proc.stdin.close()  # type: ignore[union-attr]
                self._proc.wait(timeout=5)
                self._proc = None

    def _start(self) -> None:
        self._proc = subprocess.Popen(
            [self._binary],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )

    @staticmethod
    def _find_binary(name: str) -> str:
        """Locate the bridge binary."""
        pkg_dir = Path(__file__).parent / "bin"
        suffix = ".exe" if sys.platform == "win32" else ""
        pkg_binary = pkg_dir / f"{name}{suffix}"
        if pkg_binary.exists():
            return str(pkg_binary)

        from shutil import which

        system_binary = which(name)
        if system_binary:
            return system_binary

        goos = {"linux": "linux", "darwin": "darwin", "win32": "windows"}.get(
            sys.platform, sys.platform
        )
        goarch = {"x86_64": "amd64", "AMD64": "amd64", "aarch64": "arm64", "arm64": "arm64"}.get(
            platform.machine(), "amd64"
        )
        platform_binary = pkg_dir / f"{name}-{goos}-{goarch}{suffix}"
        if platform_binary.exists():
            return str(platform_binary)

        msg = (
            f"Bridge binary '{name}' not found. "
            f"Install it with: pip install openparallax-memory[binary] "
            f"or place '{name}' in your PATH."
        )
        raise FileNotFoundError(msg)

    def __enter__(self) -> BridgeProcess:
        return self

    def __exit__(self, *_: Any) -> None:
        self.close()

    def __del__(self) -> None:
        try:
            self.close()
        except Exception:
            pass


class BridgeError(Exception):
    """Error returned by the bridge binary."""

    def __init__(self, message: str, code: int = -1) -> None:
        super().__init__(message)
        self.code = code
