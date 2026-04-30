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
    local url=""
    
    # Try using gh CLI first (avoids rate limits)
    if command -v gh >/dev/null 2>&1; then
        url=$(gh release view --repo "${REPO}" --json assets --jq ".assets[] | select(.name | contains(\"${OS}_\") and contains(\"${ARCH}\")) | .url" 2>/dev/null || true)
        if [ -z "$url" ] || [ "$url" = "null" ]; then
            url=$(gh release view --repo "${REPO}" --json assets --jq ".assets[] | select(.name | contains(\"${OS}-\") and contains(\"${ARCH}\")) | .url" 2>/dev/null || true)
        fi
        if [ -n "$url" ] && [ "$url" != "null" ]; then
            echo "$url"
            return
        fi
    fi
    
    # Fallback to API with auth if available
    local auth_header=""
    if [ -n "$GITHUB_TOKEN" ]; then
        auth_header="-H \"Authorization: token $GITHUB_TOKEN\""
    fi
    
    local release_info
    release_info=$(curl -sL $auth_header "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null || echo "{}")
    
    # Check for error
    if echo "$release_info" | jq -e '.message' >/dev/null 2>&1; then
        echo "Warning: API issue: $(echo "$release_info" | jq -r '.message')" >&2
        # Try listing releases instead
        release_info=$(curl -sL $auth_header "https://api.github.com/repos/${REPO}/releases" | jq '.[0]' 2>/dev/null || echo "{}")
    fi
    
    # Extract the asset URL
    url=$(echo "$release_info" | jq -r ".assets[] | select(.name | contains(\"${OS}_\") and contains(\"${ARCH}\")) | .browser_download_url" 2>/dev/null || true)
    
    # If not found with underscore, try hyphen
    if [ -z "$url" ] || [ "$url" = "null" ]; then
        url=$(echo "$release_info" | jq -r ".assets[] | select(.name | contains(\"${OS}-\") and contains(\"${ARCH}\")) | .browser_download_url" 2>/dev/null || true)
    fi
    
    echo "$url"
}

echo "Detected: ${OS}-${ARCH}"
echo "Fetching latest release..."
ASSET_URL=$(download_url)

if [ -z "$ASSET_URL" ] || [ "$ASSET_URL" = "null" ] || [ "$ASSET_URL" = "null" ]; then
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

if [ "$OS" = "darwin" ]; then
    xattr -dr com.apple.quarantine "$TEMP_FILE" 2>/dev/null || true
    cp "$TEMP_FILE" "${INSTALL_DIR}/${BIN_NAME}"
    rm -f "$TEMP_FILE"
else
    mv "$TEMP_FILE" "${INSTALL_DIR}/${BIN_NAME}"
fi
chmod +x "${INSTALL_DIR}/${BIN_NAME}"

echo ""
echo "Installed to: ${INSTALL_DIR}/${BIN_NAME}"
echo ""
echo "Add to PATH if needed:"
echo "  export PATH=\"\$PATH:${INSTALL_DIR}\""
echo ""
echo "Run:"
echo "  rexadb --help"
