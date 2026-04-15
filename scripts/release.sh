#!/bin/bash
set -e

REPO="rexadb/rexadb"
VERSION=$(git describe --tags 2>/dev/null || echo "latest")

echo "Building release for $VERSION..."

# Clean previous builds
rm -rf release
mkdir -p release

# Build for all platforms
echo "Building for macOS (Intel)..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o release/rexadb-darwin-amd64 .

echo "Building for macOS (Apple Silicon)..."
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o release/rexadb-darwin-arm64 .

echo "Building for Linux..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o release/rexadb-linux-amd64 .

echo "Building for Windows..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o release/rexadb-windows-amd64.exe .

# Create checksums
cd release
sha256sum rexadb-* > checksums.txt
cd ..

echo "Done! Binaries in release/"
ls -lh release/
