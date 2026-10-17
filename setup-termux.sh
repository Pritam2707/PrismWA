#!/data/data/com.termux/files/usr/bin/bash

# ==============================================================================
# PrismBot Termux Auto-Setup & Dependencies Installer
# Optimized for Android ARM64 Termux Environment
# ==============================================================================

echo "🚀 [1/4] Updating Termux repositories..."
pkg update -y && pkg upgrade -y

echo "📦 [2/4] Installing Node.js LTS, Python, FFmpeg, and native build tools..."
pkg install -y nodejs-lts python ffmpeg yt-dlp make clang libjpeg-turbo git

echo "📥 [3/4] Installing NPM Project Dependencies..."
npm install --omit=dev --loglevel=warn

echo "⚙️ [4/4] Creating data folders and verifying permissions..."
mkdir -p data data/media data/auth_info stickers

echo "=============================================================================="
echo "✅ Setup Complete!"
echo "To start the bot, run:"
echo "  ./start.sh"
echo "  or: node index.js"
echo "=============================================================================="
