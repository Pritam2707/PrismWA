# Archived Node.js Version (PrismBot)

This directory contains the legacy Node.js (Baileys) version of PrismBot.

## Why is this archived?
The bot has been completely transitioned to a high-performance pure-Go architecture located at the project root (`main.go`, `pkg/`, `start.sh`).

## Contents
- `index.js`: Main bot entry point (Baileys socket)
- `commands/`: JavaScript command handlers
- `dashboard/`: Express dashboard server & static assets
- `handlers/`: Event handlers (auto-reply, etc.)
- `services/`: TTS, Gemini router, command dispatcher
- `store/`: In-memory message and chat stores
- `utils/`: Anti-spam, helpers, sleep manager
- `config/`: Constants and voice mappings
- `auth_info/`: Multi-file Baileys WhatsApp authentication credentials
- `node_modules/`: Installed npm dependencies

## Running (if needed)
To run this archived version independently:
```bash
cd archive/nodejs
node index.js
```
*(Ensure `.env` in the project root or this folder is configured with your API keys)*
