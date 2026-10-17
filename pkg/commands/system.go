package commands

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"whatsapp-bot/pkg/pool"
	"whatsapp-bot/pkg/store"
)

var startTime = time.Now()

func RegisterSystemCommands(r *CommandRegistry) {
	r.RegisterPrefix("!help", handleHelp)
	r.RegisterPrefix("!menu", handleHelp)
	r.RegisterPrefix("!ping", handlePing)
	r.RegisterPrefix("!status", handleStatus)
	r.RegisterPrefix("!botstatus", handleStatus)
	r.RegisterPrefix("!uptime", handleStatus)
	r.RegisterPrefix("!sleep", handleSleep)
	r.RegisterPrefix("!wake", handleWake)
	r.RegisterPrefix("!antidelete", handleAntiDeleteToggle)
	r.RegisterPrefix("!features", handleFeaturesToggle)
}

func handleHelp(c *CommandContext) (bool, error) {
	fields := strings.Fields(strings.TrimSpace(c.RawText))
	sub := ""
	if len(fields) > 1 {
		sub = strings.ToLower(fields[1])
	}

	switch sub {
	case "party", "caller", "voip", "music":
		msg := `*MeowCaller VoIP & Live Song Party Commands*

• !party start <song> — Start a WhatsApp VoIP call and stream song in call
• !party call — Voice call current chat/contact to connect to party
• !party add <song> — Add a track to the live party queue (YouTube search / URL)
• !party queue — View current playing track & upcoming playlist
• !party skip — Skip currently playing song
• !party pause / !party resume — Pause or resume party music
• !party stop / !party hangup — End song party and disconnect call
• !party voicebot [on|off] — Toggle Real-Time AI Voice Chatbot (Sub-400ms Groq Whisper)
• !party voice <name> — Set AI character voice (e.g., elon_musk, trump, kurumi_en)
• !party voices — List all 29 available AI character voices
• !party status — Check VoIP call status, codec & party telemetry

Tip: Real-Time Audio Monitoring & Radio DJ controls are live on the Web Dashboard!`
		return true, c.ReplyText(msg)

	case "ai", "chat":
		msg := `*AI & Conversational Commands*

• @prism <text> / !ai <text> — Conversational AI chat with multi-modal vision
• !aimode <gemini|openrouter|groq> — Switch global AI provider
• !model <name> — Override active AI model (e.g., llama-3.3-70b-versatile)
• !style <formal|genz|self> — Switch tone persona (Formal, Gen-Z Savage, or Self)
• !catchup / !recap — Summarize recent unread group chat messages
• !roast / !cook / !burn — Generate savage roast of quoted message or user
• !brainrot / !yap / !skibidi — Translate text to Gen-Z brainrot slang
• !ocr / !read / !scan — Extract text from attached or quoted image
• !sum <text> / !summarize — Summarize long articles or messages
• !tr <lang> <text> — Translate text to any language
• !calc <expression> — Evaluate mathematical expressions
• !define <word> — Dictionary definition, pronunciation & examples
• !quote / !fact — Daily inspirational quote or mindblowing fact
• !zero2, !kurumi, !makima, !mai, !ganyu, !sukuna — Anime roleplay personas`
		return true, c.ReplyText(msg)

	case "media", "dl", "download":
		msg := `*Media, Downloader & Audio Commands*

• !dl <url> — Universal social downloader (Instagram Reels, TikTok, FB, Reddit, X, Threads, Pin)
• !play <song> / !song <query> — Search & download high-quality audio
• !video <query> — Search & download YouTube video (choose resolution 1-5)
• !vo / !viewonce / !rv — Reveal & resend View-Once photo, video, or voice note
• !transcribe / !analyze — Transcribe audio/video speech to text
• !img <query> — Search & send images
• !imagine <prompt> / !draw — Generate AI images
• !tts <voice> <text> — Text-to-speech voice synthesis (!voices for list)
• !s / !sticker — Convert image or video to WhatsApp sticker
• !toimg — Convert sticker back to regular image
• !togif — Convert animated sticker or video to GIF
• !toaudio / !tomp3 / !tovn — Extract audio from video or voice note`
		return true, c.ReplyText(msg)

	case "tools", "util", "notes":
		msg := `*Tools, Notes & Productivity Commands*

• !notes / !tree / !ls — Interactive file & notes directory tree
• !cd <folder|num|..> / !pwd — Navigate directories
• !addnote <path> [text] — Save note or save attached media (PDFs, images, docs)
• !getnote <path|num> — Retrieve saved note or download file
• !delnote <path|num> — Delete note or folder
• !pfp [@user|group|num] — Fetch profile picture of user, group, or self
• !grouppfp / !gpfp — Fetch current group profile picture
• !remind <time> <msg> — Schedule a reminder (e.g. !remind 30m standup)
• !reminders / !cancelremind <id> — List or cancel active reminders
• !poll "Question" "Opt1" "Opt2" — Create interactive poll
• !qr <text> — Generate downloadable QR code image
• !react <emoji> / !r <emoji> — React to quoted message with emoji
• !autoreply <add|list|del> — Configure keyword auto-reply rules`
		return true, c.ReplyText(msg)

	case "admin", "group":
		msg := `*Admin, Group & Moderation Commands*

• !mute @user / !unmute @user — Mute or unmute user in group
• !mutelist — View all currently muted members
• !welcome <on|off|status> — Toggle auto-welcome message for newcomers
• !setwelcome <template> — Set custom welcome message
• !tagall <msg> / !everyone — Mention all group members
• !stats — Message stats in this chat
• !leaderboard — Top active chatters in group
• !report <text> — Submit daily standup achievements
• !mystatus / !streak — View your current streak, points, and reports
• !reportboard / !streaks — View streak leaderboard & point rankings
• !reportgroup <add|remove|list> — Configure target groups for daily AI summary
• !generatereport — Trigger on-demand AI summary of today's reports
• !setdeadline <YYYY-MM-DD> <title> — Set group deadline countdown banner
• !deadline — Display visual countdown and days left
• !updatepfp — Force render and update group profile picture
• !cleardeadline — Remove group deadline`
		return true, c.ReplyText(msg)

	default:
		// Master Directory with all commands
		msg := `*PrismBot Command Directory*

*MeowCaller VoIP & Live Song Party:*
• !party start <song> — Start voice call & stream music in call
• !party call — Voice call current chat to join active party
• !party add <song> — Queue song to live party playlist
• !party queue — View current song & upcoming playlist
• !party skip — Skip current track to next in queue
• !party pause / !party resume — Pause or resume music
• !party stop / !party hangup — End call & stop party
• !party voicebot [on|off] — Toggle Real-Time AI Voice Chatbot
• !party voice <name> / !party voices — AI character voices
• !party status — Check call status & audio telemetry

*AI & Conversational:*
• @prism <text> / !ai <text> — Conversational AI chat & vision
• !aimode <gemini|openrouter|groq> — Switch global AI provider
• !model <name> — Override active AI model
• !style <formal|genz|self> — Switch tone persona
• !catchup / !recap — Summarize missed group messages
• !roast / !cook / !burn — Savage AI roast of quoted message
• !brainrot / !yap / !skibidi — Gen-Z brainrot slang
• !ocr / !read / !scan — Extract text from image
• !sum <text> / !summarize — Summarize long texts
• !tr <lang> <text> — Translate text to any language
• !calc <expr> — Math calculator
• !define <word> — Dictionary definition & examples
• !quote / !fact — Daily quotes & facts
• !zero2, !kurumi, !makima, !mai, !ganyu, !sukuna

*Media, Downloader & Audio:*
• !dl <url> — Universal downloader (IG Reels, TikTok, FB, Reddit, X)
• !play <song> / !video <query> — YouTube music & video
• !vo / !viewonce / !rv — Reveal View-Once photos/videos/audio
• !transcribe / !analyze — Speech-to-text audio analyzer
• !img <query> / !imagine <prompt> — Search & AI images
• !tts <voice> <text> — Speech synthesis (!voices for list)
• !s / !sticker — Convert image or video to sticker
• !toimg / !togif / !toaudio — Convert media formats

*Storage & Hierarchical Notes:*
• !notes / !tree / !ls — Browse folder directory tree
• !cd <dir|num|..> / !pwd — Navigate folders
• !addnote <path> [text] — Save note or media/PDF
• !getnote <path|num> — Retrieve note or file
• !delnote <path|num> — Delete note or folder

*Reports, Streaks & Deadlines:*
• !report <text> — Submit daily standup achievements
• !mystatus / !streak — View streak, points & stats
• !reportboard / !streaks — Points leaderboard
• !reportgroup <add|remove|list> — Configure daily summary
• !generatereport — Trigger AI summary
• !setdeadline <YYYY-MM-DD> <title> — Group countdown banner
• !deadline / !updatepfp / !cleardeadline

*Admin & Group:*
• !mute @user / !unmute @user / !mutelist
• !welcome <on|off|status> / !setwelcome <msg>
• !tagall <msg> / !everyone — Mention all members
• !stats / !leaderboard — Chat activity rankings

*System & Utilities:*
• !pfp [@user|group] / !grouppfp — Fetch HD user or group profile picture
• !remind <time> <msg> / !reminders / !cancelremind <id>
• !poll "Q" "Opt1" "Opt2" — Create interactive poll
• !qr <text> — Generate downloadable QR code image
• !react <emoji> — React to quoted message
• !autoreply — Configure automated reply rules
• !status / !uptime / !ping — Health & performance stats
• !sleep / !wake — Pause/resume bot processing
• !antidelete — Toggle anti-delete logger
• !features <on|off> — Toggle interactive features

_💡 Tip: Type !help <category> (e.g. !help party, !help ai, !help media, !help tools, !help admin)_`
		return true, c.ReplyText(msg)
	}
}

func handlePing(c *CommandContext) (bool, error) {
	latency := time.Since(c.RawMsg.Info.Timestamp)
	if latency < 0 {
		latency = 0
	}
	text := fmt.Sprintf("Pong: %dms (goroutines: %d)", latency.Milliseconds(), runtime.NumGoroutine())
	return true, c.ReplyText(text)
}

func handleStatus(c *CommandContext) (bool, error) {
	trimmed := strings.TrimSpace(c.RawText)
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "!status report") {
		c.RawText = "!report " + strings.TrimSpace(trimmed[len("!status report"):])
		return HandleReport(c)
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	uptime := time.Since(startTime).Truncate(time.Second)
	allocMB := float64(m.Alloc) / (1024 * 1024)
	sysMB := float64(m.Sys) / (1024 * 1024)

	chatJID := c.ChatJID.String()
	antiDelete := "disabled"
	if store.GlobalStore.IsAntiDeleteEnabled(chatJID) {
		antiDelete = "enabled"
	}

	features := "enabled"
	if c.IsGroup && !store.GlobalStore.IsFeaturesEnabled(chatJID) {
		features = "disabled"
	}

	state := "active"
	if store.GlobalStore.IsSleeping() {
		state = "sleeping"
	}

	statusText := fmt.Sprintf(`*Status*
- State: %s
- Uptime: %s
- Memory: %.2f MB alloc / %.2f MB sys
- Goroutines: %d
- Workers: %d active / %d processed
- Anti-delete: %s
- Features: %s
- Cached messages: %d`,
		state,
		uptime.String(),
		allocMB, sysMB,
		runtime.NumGoroutine(),
		pool.GlobalPool.ActiveWorkers(),
		pool.GlobalPool.TotalProcessed(),
		antiDelete,
		features,
		store.GlobalStore.Count(),
	)

	return true, c.ReplyText(statusText)
}

func handleSleep(c *CommandContext) (bool, error) {
	store.GlobalStore.SetSleeping(true)
	return true, c.ReplyText("Bot entered sleep mode. Send !wake to resume.")
}

func handleWake(c *CommandContext) (bool, error) {
	wasSleeping := store.GlobalStore.IsSleeping()
	store.GlobalStore.SetSleeping(false)
	if wasSleeping {
		return true, c.ReplyText("Bot resumed.")
	}
	return true, c.ReplyText("Bot is already active.")
}

func handleAntiDeleteToggle(c *CommandContext) (bool, error) {
	if !c.IsSenderOwner() {
		return true, c.ReplyText("Only the bot owner can toggle anti-delete.")
	}

	chatJID := c.ChatJID.String()
	enabled := !store.GlobalStore.IsAntiDeleteEnabled(chatJID)
	store.GlobalStore.ToggleAntiDelete(chatJID, enabled)

	if enabled {
		return true, c.ReplyText("Anti-delete enabled for this chat.")
	}
	return true, c.ReplyText("Anti-delete disabled for this chat.")
}

func handleFeaturesToggle(c *CommandContext) (bool, error) {
	if !c.IsSenderOwner() {
		return true, c.ReplyText("Only the bot owner can toggle features.")
	}

	parts := strings.Fields(strings.ToLower(c.RawText))
	chatJID := c.ChatJID.String()

	if len(parts) > 1 {
		switch parts[1] {
		case "on", "enable":
			store.GlobalStore.ToggleFeatures(chatJID, true)
			return true, c.ReplyText("Features enabled for this chat.")
		case "off", "disable":
			store.GlobalStore.ToggleFeatures(chatJID, false)
			return true, c.ReplyText("Features disabled for this chat.")
		}
	}

	status := "disabled"
	if store.GlobalStore.IsFeaturesEnabled(chatJID) {
		status = "enabled"
	}
	return true, c.ReplyText(fmt.Sprintf("Features for this chat: %s. Use !features on or !features off to change.", status))
}
