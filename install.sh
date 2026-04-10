#!/bin/sh
# OpenParallax installer for Linux and macOS.
# Usage: curl -sSL https://get.openparallax.dev | sh
#        curl -sSL https://get.openparallax.dev/shield | sh
#    or: sh install.sh [--component shield] [--version v0.1.0] [--dir ~/.local/bin]
set -e

REPO="openparallax/openparallax"
INSTALL_DIR="${HOME}/.local/bin"
VERSION=""
COMPONENT="openparallax"

# Parse flags.
while [ $# -gt 0 ]; do
  case "$1" in
    --version)   VERSION="$2"; shift 2 ;;
    --dir)       INSTALL_DIR="$2"; shift 2 ;;
    --component) COMPONENT="openparallax-$2"; shift 2 ;;
    *)           echo "Unknown flag: $1"; exit 1 ;;
  esac
done

# Detect OS.
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  linux)  OS="linux" ;;
  darwin) OS="darwin" ;;
  *)      echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Detect architecture.
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64)   ARCH="amd64" ;;
  aarch64|arm64)   ARCH="arm64" ;;
  *)               echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Fetch latest version if not pinned.
if [ -z "$VERSION" ]; then
  VERSION=$(curl -sSf "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' | head -1 | cut -d'"' -f4)
  if [ -z "$VERSION" ]; then
    echo "Failed to fetch latest release version."
    exit 1
  fi
fi

# Strip leading v for the archive name.
VERSION_NUM="${VERSION#v}"

# Build archive name. The main binary archive uses "openparallax_VERSION_OS_ARCH",
# Shield uses "openparallax-shield_VERSION_OS_ARCH".
ARCHIVE="${COMPONENT}_${VERSION_NUM}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"
CHECKSUM_URL="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"

DISPLAY_NAME=$(echo "$COMPONENT" | sed 's/openparallax-/OpenParallax /;s/openparallax/OpenParallax/')

echo "Installing ${DISPLAY_NAME} ${VERSION} (${OS}/${ARCH})"
echo "  Archive:  ${ARCHIVE}"
echo "  Install:  ${INSTALL_DIR}"

# Create temp directory.
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

# Download archive and checksums.
echo "Downloading..."
curl -#fL -o "${TMP}/${ARCHIVE}" "$URL"
curl -sSfL -o "${TMP}/checksums.txt" "$CHECKSUM_URL"

# Verify checksum.
echo "Verifying checksum..."
cd "$TMP"
EXPECTED=$(grep "${ARCHIVE}" checksums.txt | awk '{print $1}')
if [ -z "$EXPECTED" ]; then
  echo "Checksum not found for ${ARCHIVE}"
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL=$(sha256sum "${ARCHIVE}" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL=$(shasum -a 256 "${ARCHIVE}" | awk '{print $1}')
else
  echo "Warning: no sha256sum or shasum found, skipping verification"
  ACTUAL="$EXPECTED"
fi

if [ "$EXPECTED" != "$ACTUAL" ]; then
  echo "Checksum mismatch!"
  echo "  Expected: ${EXPECTED}"
  echo "  Got:      ${ACTUAL}"
  exit 1
fi
echo "Checksum OK."

# Extract.
tar xzf "${ARCHIVE}"

# Install.
mkdir -p "$INSTALL_DIR"
cp "$COMPONENT" "$INSTALL_DIR/$COMPONENT"
chmod +x "$INSTALL_DIR/$COMPONENT"

echo ""
echo "${DISPLAY_NAME} ${VERSION} installed to ${INSTALL_DIR}/${COMPONENT}"

# Check PATH.
case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    echo ""
    echo "Add ${INSTALL_DIR} to your PATH:"
    echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
    ;;
esac

echo ""
if [ "$COMPONENT" = "openparallax" ]; then
  echo "Get started:"
  echo "  openparallax init"
  echo "  openparallax start"
else
  echo "Get started:"
  echo "  ${COMPONENT} --help"
fi
