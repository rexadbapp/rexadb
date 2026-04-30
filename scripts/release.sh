#!/bin/bash
set -e

REPO="rexadbapp/rexadb"
VERSION="${VERSION:-$(git describe --tags 2>/dev/null | sed 's/^v//' || echo "0.1.0")}"
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(date -u +%Y-%m-%d)

echo "Building release v$VERSION..."

rm -rf release
mkdir -p release

build() {
    local os=$1
    local arch=$2
    local ext=""
    local ldflags="-s -w -X github.com/rexadb/rexadb/cmd.Version=$VERSION -X github.com/rexadb/rexadb/cmd.GitCommit=$COMMIT -X github.com/rexadb/rexadb/cmd.BuildTime=$DATE"
    
    if [ "$os" = "windows" ]; then
        ext=".exe"
    fi
    
    local name="rexadb_${os}_${arch}${ext}"
    echo "Building $name..."
    GOOS=$os GOARCH=$arch go build -ldflags="$ldflags" -o "release/$name" .
}

build darwin amd64
build darwin arm64
build linux amd64
build windows amd64

sha256sum release/rexadb_darwin_amd64 release/rexadb_darwin_arm64 release/rexadb_linux_amd64 release/rexadb_windows_amd64.exe > release/checksums.txt

if ! gh auth status &>/dev/null; then
    echo ""
    echo "Not logged into gh. Run 'gh auth login' first, then re-run this script."
    echo ""
    ls -lh release/
    exit 0
fi

echo "Creating GitHub release..."

body=$(cat <<'ENDOFNOTES'
## Changes
- Update to version VERSION

## Downloads
- macOS (Intel): rexadb_darwin_amd64
- macOS (Apple Silicon): rexadb_darwin_arm64  
- Linux: rexadb_linux_amd64
- Windows: rexadb_windows_amd64.exe

## Verification
Run `sha256sum -c checksums.txt` to verify downloads.
ENDOFNOTES
)

body="${body//VERSION/$VERSION}"

gh release create "v$VERSION" --title "Version $VERSION" --notes "$body" --draft=false 2>/dev/null || {
    echo "Release exists, adding assets..."
}

echo "Uploading assets..."
for f in release/rexadb_darwin_amd64 release/rexadb_darwin_arm64 release/rexadb_linux_amd64 release/rexadb_windows_amd64.exe release/checksums.txt; do
    gh release upload "v$VERSION" "$f" --clobber
done

echo ""
echo "Release published! https://github.com/$REPO/releases/tag/v$VERSION"
echo ""
ls -lh release/