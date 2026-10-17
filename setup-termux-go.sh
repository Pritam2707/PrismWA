#!/data/data/com.termux/files/usr/bin/bash
set -euo pipefail

# ==============================================================================
# PrismBot Pure Go Termux Environment Setup
# ==============================================================================

echo "🚀 [1/3] Updating Termux packages..."
pkg update -y

echo "📦 [2/3] Installing utilities and dependencies..."
pkg install -y ca-certificates golang git ffmpeg yt-dlp termux-tools

# Acquire wake lock to keep bot running when screen is off
termux-wake-lock 2>/dev/null || true

CURRENT_DIR=$(pwd)
if [[ "$CURRENT_DIR" == *"/sdcard"* || "$CURRENT_DIR" == *"/storage/emulated"* ]]; then
    echo "⚠️  WARNING: You are in Android shared storage ($CURRENT_DIR)."
    echo "   Android mounts /sdcard with 'noexec', which prevents executing binaries."
    echo "   Recommendation: Move/clone into Termux home:"
    echo "     cp -r $CURRENT_DIR ~/WhatsappBot && cd ~/WhatsappBot"
fi

echo "⚙️ [3/3] Compiling PrismBot with Go..."
CGO_ENABLED=0 go build -ldflags="-s -w" -o prismbot .
chmod +x ./prismbot

mkdir -p data data/media auth_info

echo "=============================================================================="
echo "✅ Setup Complete!"
echo "Make sure your .env file has GEMINI_API_KEY configured."
echo "To run PrismBot:"
echo "  ./start.sh"
echo "  or: ./prismbot"
echo "=============================================================================="
