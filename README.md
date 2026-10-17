# PrismBot

WhatsApp automation bot built with Go, SQLite, and a web dashboard.

---

## Branches

This repository maintains two versions across Git branches:

| Branch | Stack | Status |
|--------|-------|--------|
| `v2` (default) | Go 1.22+, `modernc.org/sqlite`, pure Go WhatsApp engine | Active |
| `v1` | Node.js (ESM), `@whiskeysockets/baileys`, Express | Archived |

Switch to the legacy JavaScript version at any time:
```bash
git checkout v1
```

---

## Architecture

- **Engine:** Pure Go WhatsApp socket client using `hypermeow` and `meowcaller`.
- **Database:** Local SQLite (`modernc.org/sqlite`) with WAL mode. Requires no CGO.
- **Worker Pool:** Bounded worker pool handling asynchronous message dispatch and media pipelines.
- **AI Routing:** Multi-provider interface supporting Google Gemini, OpenRouter, Groq, and Ollama with fallback routing.
- **Dashboard:** Embedded HTTP server running on `:8080` providing real-time stats, log streaming, and bot controls.

---

## Prerequisites

- **Go:** 1.22 or higher
- **FFmpeg:** Required for audio transcoding and sticker conversions
- **yt-dlp:** Required for media download commands

---

## Setup

### 1. Clone & Configure

```bash
git clone <repo-url>
cd WhatsappBot
cp .env.example .env  # Or create .env manually
```

Configure your credentials in `.env`:

```env
# AI Provider: gemini | openrouter | groq | ollama
AI_PROVIDER=gemini
GEMINI_API_KEY=your_key_here
GEMINI_MODEL=gemini-2.5-flash

# Optional: Voice Synthesis (Fish Audio & RVC)
FISH_AUDIO_API_KEY=your_fish_audio_key
RVC_API_URL=http://127.0.0.1:8000

# Optional: Search
SERPER_API_KEY=your_serper_key
```

### 2. Build & Run

**Windows:**
```bat
build.bat
prismbot.exe
```

**Linux / macOS:**
```bash
chmod +x build.sh start.sh
./build.sh
./start.sh
```

**Direct Run (Development):**
```bash
go run .
```

Scan the terminal QR code using WhatsApp on your phone (Linked Devices).

---

## Configuration Reference

| Variable | Default | Description |
|----------|---------|-------------|
| `AI_PROVIDER` | `gemini` | Primary AI provider (`gemini`, `openrouter`, `groq`, `ollama`) |
| `GEMINI_API_KEY` | — | Google Gemini API key |
| `GEMINI_MODEL` | `gemini-2.5-flash` | Gemini model name |
| `OPENROUTER_API_KEY` | — | OpenRouter API key |
| `OPENROUTER_MODEL` | `cognitivecomputations/dolphin-mistral-24b-venice-edition` | OpenRouter model ID |
| `GROQ_API_KEY` | — | Groq Cloud API key |
| `GROQ_MODEL` | `llama-3.3-70b-versatile` | Groq model name |
| `FISH_AUDIO_API_KEY` | — | API key for Fish Audio TTS |
| `RVC_API_URL` | `http://127.0.0.1:8000` | Local RVC inference endpoint |
| `SERPER_API_KEY` | — | Serper Google Search API key |
| `PORT` | `8080` | Dashboard HTTP port |

---

## Core Commands

All commands accept either `!` or `.` prefixes.

### AI & Chat
- `!ask <prompt>` — Queries the configured AI provider.
- `!draw <prompt>` — Generates an image using HuggingFace / Gemini.
- `!persona <name>` — Switches active persona.
- `!mode <formal|genz|default>` — Sets tone preset.

### Media & Voice
- `!s` / `!sticker` — Converts quoted image or video to WebP sticker.
- `!toimg` — Converts sticker to image.
- `!tts <voice> <text>` — Generates speech using Fish Audio.
- `!play <song name>` — Streams audio from YouTube.
- `!yts <query>` — Searches YouTube tracks.

### Group Administration
- `!tagall <msg>` — Mentions every group member.
- `!kick @user` — Removes member from group.
- `!promote @user` / `!demote @user` — Modifies member admin privileges.
- `!mute` / `!unmute` — Toggles group message permissions.
- `!deleted` — Retrieves recent deleted message log.

### Utilities
- `!note save <key> <val>` — Saves persistent snippet to database.
- `!note get <key>` — Retrieves snippet.
- `!deadline add <date> <desc>` — Sets group deadline.
- `!remind <duration> <text>` — Sets reminder timer.
- `!ping` / `!stats` — Reports runtime latency, memory, and goroutine counts.

---

## Cross Compilation

Compile for Android Termux (ARM64 / ARMv7):

```bash
# On Linux / WSL
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o prismbot-termux-arm64 .

# On Windows
build-termux.bat
```

---

## Directory Layout

```
├── dashboard/        # Dashboard frontend static assets (HTML/CSS/JS)
├── pkg/
│   ├── commands/     # Command implementations and routing logic
│   ├── config/       # Environment loading and runtime configuration
│   ├── dashboard/    # HTTP handlers and WebSocket log streamer
│   ├── gemini/       # LLM provider clients
│   ├── meowcaller/   # Voice call handler and radio mixer
│   ├── pool/         # Worker pool implementation
│   ├── store/        # SQLite persistent storage and cache
│   ├── tts/          # TTS generation clients
│   └── whatsapp/     # WhatsApp connection manager and event dispatcher
├── stickers/         # Pre-bundled WebP sticker assets
├── build.bat         # Windows build script
├── build.sh          # Linux build script
├── main.go           # Entry point
└── start.sh          # Production runner with crash recovery
```

---

## License

Personal and internal use.
