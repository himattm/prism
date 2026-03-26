#!/bin/bash
# Restore prism from latest GitHub release

set -e

CLAUDE_DIR="$HOME/.claude"
BINARY_PATH="$CLAUDE_DIR/prism"

# Detect OS/arch
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case $ARCH in
    x86_64) ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
esac

BINARY_URL="https://github.com/himattm/prism/releases/latest/download/prism-${OS}-${ARCH}"

echo "Downloading latest release for ${OS}-${ARCH}..."
curl -fsSL "$BINARY_URL" -o "$BINARY_PATH.new"
chmod +x "$BINARY_PATH.new"
mv "$BINARY_PATH.new" "$BINARY_PATH"

echo "Restored to release version: $($BINARY_PATH version)"

# Show available backups
echo ""
echo "Backups available:"
ls -la "$BINARY_PATH.backup."* 2>/dev/null || echo "  (none)"
