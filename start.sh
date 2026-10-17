#!/data/data/com.termux/files/usr/bin/bash

# ==============================================================================
# PrismBot Pure Go Termux Launcher Script
# ==============================================================================

# Keep Termux awake in background
termux-wake-lock 2>/dev/null || true

export GOLANG_PROTOBUF_REGISTRATION_CONFLICT=ignore
export PORT=3000

echo "🤖 Starting PrismBot (Pure Go) in Termux..."
echo "🌐 Dashboard will be live at: http://localhost:3000"

CURRENT_DIR=$(pwd)
if [[ "$CURRENT_DIR" == *"/sdcard"* || "$CURRENT_DIR" == *"/storage/emulated"* ]]; then
    echo "⚠️  WARNING: You are inside Android shared storage ($CURRENT_DIR)."
    echo "   Android prevents executing binaries directly from /sdcard (noexec)."
    echo "   If the bot fails with 'Permission denied', please move to Termux home:"
    echo "     cp -r $CURRENT_DIR ~/WhatsappBot && cd ~/WhatsappBot && ./start.sh"
    echo ""
fi

# Locate suitable binary
TARGET_BIN=""
if [ -f "./prismbot" ]; then
    chmod +x ./prismbot 2>/dev/null || true
    TARGET_BIN="./prismbot"
elif [ -f "./prismbot-termux-arm64" ]; then
    chmod +x ./prismbot-termux-arm64 2>/dev/null || true
    TARGET_BIN="./prismbot-termux-arm64"
fi

# If binary is missing or cannot run, auto-build if Go is installed
if [ -z "$TARGET_BIN" ] || ! "$TARGET_BIN" --help >/dev/null 2>&1; then
    if command -v go >/dev/null 2>&1; then
        echo "🔨 Building PrismBot natively with Go..."
        CGO_ENABLED=0 go build -ldflags="-s -w -X google.golang.org/protobuf/reflect/protoregistry.conflictPolicy=ignore" -o prismbot .
        chmod +x ./prismbot
        TARGET_BIN="./prismbot"
    elif [ -z "$TARGET_BIN" ]; then
        echo "❌ Error: Neither ./prismbot nor ./prismbot-termux-arm64 found!"
        echo "Run ./setup-termux-go.sh to set up and compile."
        exit 1
    fi
fi

exec "$TARGET_BIN"
