"""JSON-RPC bridge to the Go shield-bridge binary."""

from __future__ import annotations

import hashlib
import json
import os
import platform
import subprocess
import sys
import tarfile
import tempfile
import threading
import urllib.request
import zipfile
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
            request = {
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
        # Check package bin/ directory first.
        pkg_dir = Path(__file__).parent / "bin"
        suffix = ".exe" if sys.platform == "win32" else ""
        pkg_binary = pkg_dir / f"{name}{suffix}"
        if pkg_binary.exists():
            return str(pkg_binary)

        # Check system PATH.
        from shutil import which

        system_binary = which(name)
        if system_binary:
            return system_binary

        # Check platform-specific name.
        goos = {"linux": "linux", "darwin": "darwin", "win32": "windows"}.get(
            sys.platform, sys.platform
        )
        goarch = {"x86_64": "amd64", "AMD64": "amd64", "aarch64": "arm64", "arm64": "arm64"}.get(
            platform.machine(), "amd64"
        )
        platform_binary = pkg_dir / f"{name}-{goos}-{goarch}{suffix}"
        if platform_binary.exists():
            return str(platform_binary)

        # Auto-download the bridge binary from the GitHub release.
        downloaded = _download_bridge(name, pkg_dir, goos, goarch, suffix)
        if downloaded:
            return downloaded

        msg = (
            f"Bridge binary '{name}' not found and auto-download failed. "
            f"Install OpenParallax (curl -sSL https://get.openparallax.dev | sh) "
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


REPO = "openparallax/openparallax"


def _download_bridge(name: str, dest_dir: Path, goos: str, goarch: str, suffix: str) -> str | None:
    """Download the bridge binary from the GitHub release matching this package version.

    Follows the same pattern as install.sh: detect platform, download archive,
    verify checksum, extract the specific binary.
    """
    version = _resolve_version()
    if not version:
        return None

    version_num = version.lstrip("v")
    archive_ext = "zip" if goos == "windows" else "tar.gz"
    archive_name = f"openparallax-bridges_{version_num}_{goos}_{goarch}.{archive_ext}"
    base_url = f"https://github.com/{REPO}/releases/download/{version}"
    archive_url = f"{base_url}/{archive_name}"
    checksum_url = f"{base_url}/checksums.txt"

    try:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            archive_path = tmp_path / archive_name
            checksum_path = tmp_path / "checksums.txt"

            # Download archive.
            print(f"Downloading {name} bridge binary ({goos}/{goarch})...")
            urllib.request.urlretrieve(archive_url, archive_path)
            urllib.request.urlretrieve(checksum_url, checksum_path)

            # Verify checksum.
            expected = _find_checksum(checksum_path, archive_name)
            if expected:
                actual = hashlib.sha256(archive_path.read_bytes()).hexdigest()
                if actual != expected:
                    print(f"Checksum mismatch for {archive_name}", file=sys.stderr)
                    return None
                print("Checksum verified.")

            # Extract the specific binary.
            binary_name = f"{name}{suffix}"
            if archive_ext == "zip":
                with zipfile.ZipFile(archive_path) as zf:
                    for member in zf.namelist():
                        if member.endswith(binary_name):
                            extracted = tmp_path / binary_name
                            extracted.write_bytes(zf.read(member))
                            break
            else:
                with tarfile.open(archive_path, "r:gz") as tf:
                    for member in tf.getmembers():
                        if member.name.endswith(binary_name):
                            member.name = binary_name
                            tf.extract(member, tmp_path)
                            break

            extracted = tmp_path / binary_name
            if not extracted.exists():
                print(f"Binary {binary_name} not found in archive", file=sys.stderr)
                return None

            # Install to package bin/ directory.
            dest_dir.mkdir(parents=True, exist_ok=True)
            dest = dest_dir / binary_name
            dest.write_bytes(extracted.read_bytes())
            dest.chmod(0o755)
            print(f"Installed {name} to {dest}")
            return str(dest)

    except Exception as e:
        print(f"Auto-download failed: {e}", file=sys.stderr)
        return None


def _resolve_version() -> str | None:
    """Get the package version. Falls back to latest GitHub release if 0.0.0.

    Tries every openparallax-* package to find a real version — any wrapper
    from the same release will have the same version number.
    """
    try:
        from importlib.metadata import version as pkg_version
        for pkg in ("openparallax-shield", "openparallax-audit", "openparallax-memory",
                     "openparallax-sandbox", "openparallax-channels"):
            try:
                v = pkg_version(pkg)
                if v and v != "0.0.0":
                    return f"v{v}"
            except Exception:
                continue
    except Exception:
        pass

    # Fall back to latest release.
    try:
        url = f"https://api.github.com/repos/{REPO}/releases/latest"
        req = urllib.request.Request(url, headers={"Accept": "application/vnd.github.v3+json"})
        with urllib.request.urlopen(req, timeout=10) as resp:
            data = json.loads(resp.read())
            return data.get("tag_name")
    except Exception:
        return None


def _find_checksum(checksum_path: Path, archive_name: str) -> str | None:
    """Find the SHA-256 checksum for the given archive in checksums.txt."""
    for line in checksum_path.read_text().splitlines():
        if archive_name in line:
            return line.split()[0]
    return None


class BridgeError(Exception):
    """Error returned by the bridge binary."""

    def __init__(self, message: str, code: int = -1) -> None:
        super().__init__(message)
        self.code = code
