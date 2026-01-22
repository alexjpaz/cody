#!/bin/bash
set -e

REPO="alexjpaz/cody"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Detect OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
    linux*)  OS="linux" ;;
    darwin*) OS="darwin" ;;
    *)       echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    amd64)   ARCH="amd64" ;;
    arm64)   ARCH="arm64" ;;
    aarch64) ARCH="arm64" ;;
    *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

BINARY="cody-${OS}-${ARCH}"

# Get latest release tag
echo "Fetching latest release..."
LATEST=$(curl -s "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST" ]; then
    echo "Failed to fetch latest release"
    exit 1
fi

echo "Latest version: $LATEST"

# Download URL
URL="https://github.com/${REPO}/releases/download/${LATEST}/${BINARY}"

echo "Downloading $BINARY..."
curl -sL "$URL" -o /tmp/cody

# Make executable
chmod +x /tmp/cody

# Install
echo "Installing to ${INSTALL_DIR}/cody..."
if [ -w "$INSTALL_DIR" ]; then
    mv /tmp/cody "${INSTALL_DIR}/cody"
else
    sudo mv /tmp/cody "${INSTALL_DIR}/cody"
fi

echo "Done! cody $LATEST installed to ${INSTALL_DIR}/cody"
