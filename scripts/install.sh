#!/bin/bash
set -e

REPO="rexadbapp/rexadb"
BIN_NAME="rexadb"

detect_os() {
    case "$(uname -s)" in
        Linux) echo "linux" ;;
        Darwin) echo "darwin" ;;
        *) echo "" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64) echo "amd64" ;;
        arm64|aarch64) echo "arm64" ;;
        *) echo "" ;;
    esac
}

OS=$(detect_os)
ARCH=$(detect_arch)

if [ -z "$OS" ] || [ -z "$ARCH" ]; then
    echo "Error: Unsupported platform"
    exit 1
fi

download_url() {
    local url
    url=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | \
        jq -r ".assets[] | select(.name | contains(\"${OS}\") and contains(\"${ARCH}\")) | .browser_download_url")
    echo "$url"
}

echo "Detected: ${OS}-${ARCH}"
echo "Fetching latest release..."
ASSET_URL=$(download_url)

if [ -z "$ASSET_URL" ] || [ "$ASSET_URL" = "null" ]; then
    echo "Error: Could not find release for your platform"
    exit 1
fi

echo "Downloading: $ASSET_URL"

INSTALL_DIR="${HOME}/.local/bin"
if [ ! -d "$INSTALL_DIR" ]; then
    mkdir -p "$INSTALL_DIR"
fi

TEMP_FILE=$(mktemp)
curl -fL "$ASSET_URL" -o "$TEMP_FILE"

mv "$TEMP_FILE" "${INSTALL_DIR}/${BIN_NAME}"
chmod +x "${INSTALL_DIR}/${BIN_NAME}"

if [ "$OS" = "darwin" ]; then
    xattr -cr "${INSTALL_DIR}/${BIN_NAME}" 2>/dev/null || true
fi

echo ""
echo "Installed to: ${INSTALL_DIR}/${BIN_NAME}"
echo ""
echo "Add to PATH if needed:"
echo "  export PATH=\"\$PATH:${INSTALL_DIR}\""
echo ""
echo "Run:"
echo "  rexadb --help"
