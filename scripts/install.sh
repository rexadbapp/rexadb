#!/bin/bash
set -e

REPO="rexadb/rexadb"
VERSION=$(git tag --sort=-version:refname 2>/dev/null | head -1 || echo "")

if [ -z "$VERSION" ]; then
    echo "Downloading latest release..."
    ASSET_URL=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | \
        jq -r '.assets[] | select(.name | contains("'"$(uname -s | tr '[:upper:]' '[:lower:]')"'"'"'"'"'"'"'" and contains("'"$(uname -m)"'")) | .browser_download_url')
else
    echo "Downloading ${VERSION}..."
    ASSET_URL=$(curl -sL "https://api.github.com/repos/${REPO}/releases/tags/${VERSION}" | \
        jq -r '.assets[] | select(.name | contains("'"$(uname -s | tr '[:upper:]' '[:lower:]')"'"'"'"'"'"'"'" and contains("'"$(uname -m)"'")) | .browser_download_url')
fi

if [ -z "$ASSET_URL" ]; then
    echo "Error: Could not find release for your platform"
    exit 1
fi

echo "Downloading: $ASSET_URL"

# Download to temp
TEMP_FILE=$(mktemp)
curl -fL "$ASSET_URL" -o "$TEMP_FILE"

# Install
INSTALL_DIR="${HOME}/.local/bin"
if [ ! -d "$INSTALL_DIR" ]; then
    mkdir -p "$INSTALL_DIR"
fi

mv "$TEMP_FILE" "${INSTALL_DIR}/rexadb"
chmod +x "${INSTALL_DIR}/rexadb"

echo ""
echo "Installed to: ${INSTALL_DIR}/rexadb"
echo ""
echo "Add to PATH if needed:"
echo "  export PATH=\"\$PATH:${INSTALL_DIR}\""
echo ""
echo "Run:"
echo "  rexadb --help"
