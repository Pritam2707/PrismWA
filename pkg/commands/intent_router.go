package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"whatsapp-bot/pkg/gemini"
)

// UserIntent represents the classified intent and extracted parameters
type UserIntent struct {
	Intent     string            `json:"intent"`
	Confidence float64           `json:"confidence"`
	Parameters map[string]string `json:"parameters"`
	Reasoning  string            `json:"reasoning,omitempty"`
}

// FastPathIntent performs zero-latency heuristic evaluation for unambiguous command requests
func FastPathIntent(cleanPrompt string, hasMedia, hasQuotedMedia, hasQuotedText bool) *UserIntent {
	trimmed := strings.TrimSpace(cleanPrompt)
	lower := strings.ToLower(trimmed)

	if lower == "" {
		return nil
	}

	// 1. Sticker intent
	if lower == "sticker" || lower == "s" || lower == "make sticker" || lower == "create sticker" ||
		lower == "turn this into a sticker" || lower == "convert to sticker" || lower == "sticker this" ||
		strings.HasPrefix(lower, "sticker ") || strings.HasPrefix(lower, "make sticker ") {
		return &UserIntent{
			Intent:     "sticker",
			Confidence: 0.99,
			Reasoning:  "fast-path sticker trigger",
		}
	}

	// 2. Catch up / recap intent
	if lower == "catchup" || lower == "catch up" || lower == "recap" ||
		lower == "catch me up" || lower == "what did i miss" || lower == "what happened while i was away" ||
		strings.HasPrefix(lower, "catchup ") || strings.HasPrefix(lower, "recap ") {
		count := ""
		fields := strings.Fields(lower)
		if len(fields) > 1 {
			count = fields[1]
		}
		params := make(map[string]string)
		if count != "" {
			params["count"] = count
		}
		return &UserIntent{
			Intent:     "catchup",
			Confidence: 0.99,
			Parameters: params,
			Reasoning:  "fast-path catchup trigger",
		}
	}

	// 3. OCR / Extract text intent
	if lower == "ocr" || lower == "read" || lower == "scan" ||
		lower == "extract text" || lower == "read text" || lower == "ocr this" || lower == "scan this" {
		return &UserIntent{
			Intent:     "ocr",
			Confidence: 0.99,
			Reasoning:  "fast-path ocr trigger",
		}
	}

	// 4. View-once reveal intent
	if lower == "viewonce" || lower == "vo" || lower == "reveal" || lower == "open view once" ||
		lower == "reveal view once" || lower == "show view once" || lower == "reveal this" {
		return &UserIntent{
			Intent:     "viewonce",
			Confidence: 0.99,
			Reasoning:  "fast-path viewonce trigger",
		}
	}

	// 5. Weather intent
	weatherPrefixes := []string{"weather in ", "weather for ", "weather at ", "forecast for ", "forecast in ", "weather "}
	for _, p := range weatherPrefixes {
		if strings.HasPrefix(lower, p) {
			city := strings.TrimSpace(trimmed[len(p):])
			if city != "" {
				return &UserIntent{
					Intent:     "weather",
					Confidence: 0.98,
					Parameters: map[string]string{"city": city},
					Reasoning:  "fast-path weather trigger",
				}
			}
		}
	}

	// 6. Remind intent
	// e.g. "remind me in 10m to call John", "remind 10m check oven", "remind me to call John in 10m"
	remindInRegex := regexp.MustCompile(`(?i)^remind(?:\s+me)?\s+in\s+(\d+[smhd])\s+(?:to\s+)?(.+)$`)
	if m := remindInRegex.FindStringSubmatch(trimmed); len(m) == 3 {
		return &UserIntent{
			Intent:     "remind",
			Confidence: 0.98,
			Parameters: map[string]string{
				"duration": strings.ToLower(m[1]),
				"message":  strings.TrimSpace(m[2]),
			},
			Reasoning: "fast-path remind in <dur> <msg>",
		}
	}

	remindDirectRegex := regexp.MustCompile(`(?i)^remind\s+(\d+[smhd])\s+(.+)$`)
	if m := remindDirectRegex.FindStringSubmatch(trimmed); len(m) == 3 {
		return &UserIntent{
			Intent:     "remind",
			Confidence: 0.98,
			Parameters: map[string]string{
				"duration": strings.ToLower(m[1]),
				"message":  strings.TrimSpace(m[2]),
			},
			Reasoning: "fast-path remind <dur> <msg>",
		}
	}

	// 7. Calculate / math intent
	calcPrefixes := []string{"calc ", "calculate ", "math ", "solve equation "}
	for _, p := range calcPrefixes {
		if strings.HasPrefix(lower, p) {
			expr := strings.TrimSpace(trimmed[len(p):])
			if expr != "" {
				return &UserIntent{
					Intent:     "calc",
					Confidence: 0.98,
					Parameters: map[string]string{"expression": expr},
					Reasoning:  "fast-path calc trigger",
				}
			}
		}
	}

	// 8. Define word intent
	definePrefixes := []string{"define ", "meaning of ", "definition of "}
	for _, p := range definePrefixes {
		if strings.HasPrefix(lower, p) {
			word := strings.TrimSpace(trimmed[len(p):])
			// If it's 1-3 words, high confidence definition
			if word != "" && len(strings.Fields(word)) <= 3 {
				return &UserIntent{
					Intent:     "define",
					Confidence: 0.97,
					Parameters: map[string]string{"word": word},
					Reasoning:  "fast-path define trigger",
				}
			}
		}
	}

	// 9. Translate intent
	// e.g. "translate 'hello world' to French", "translate to Spanish: hello world"
	trToRegex := regexp.MustCompile(`(?i)^translate\s+to\s+([a-zA-Z]+)[:\s]+(.+)$`)
	if m := trToRegex.FindStringSubmatch(trimmed); len(m) == 3 {
		return &UserIntent{
			Intent:     "translate",
			Confidence: 0.98,
			Parameters: map[string]string{
				"target_lang": m[1],
				"text":        strings.TrimSpace(m[2]),
			},
			Reasoning: "fast-path translate to <lang> <text>",
		}
	}

	// 10. Deleted messages intent
	if lower == "deleted" || lower == "deleted messages" || lower == "show deleted" ||
		lower == "show deleted messages" || lower == "what was deleted" {
		return &UserIntent{
			Intent:     "deleted",
			Confidence: 0.99,
			Reasoning:  "fast-path deleted trigger",
		}
	}

	// 11. Notes intent
	if lower == "notes" || lower == "show notes" || lower == "list notes" || lower == "my notes" {
		return &UserIntent{
			Intent:     "notes_list",
			Confidence: 0.99,
			Reasoning:  "fast-path notes_list trigger",
		}
	}

	// 12. Song Party / Voice Call Party intent (checked before general music/video)
	partyPrefixes := []string{"song party ", "start party ", "party start ", "songparty ", "party play ", "party add "}
	for _, p := range partyPrefixes {
		if strings.HasPrefix(lower, p) {
			query := strings.TrimSpace(trimmed[len(p):])
			return &UserIntent{
				Intent:     "party",
				Confidence: 0.99,
				Parameters: map[string]string{"query": query},
				Reasoning:  "fast-path song party trigger",
			}
		}
	}
	if lower == "song party" || lower == "party status" || lower == "songparty" {
		return &UserIntent{
			Intent:     "party",
			Confidence: 0.99,
			Reasoning:  "fast-path song party status trigger",
		}
	}

	// 12b. Music / Song / Video playback intent
	playPrefixes := []string{"play song ", "play music ", "play video ", "play ", "song ", "video "}
	for _, p := range playPrefixes {
		if strings.HasPrefix(lower, p) {
			query := strings.TrimSpace(trimmed[len(p):])
			if query != "" {
				intentType := "play"
				if strings.HasPrefix(p, "video") || strings.HasPrefix(p, "play video") {
					intentType = "video"
				}
				return &UserIntent{
					Intent:     intentType,
					Confidence: 0.99,
					Parameters: map[string]string{"query": query},
					Reasoning:  "fast-path play trigger",
				}
			}
		}
	}

	// 12c. Social Media Downloader intent
	dlPrefixes := []string{"download ", "dl ", "ig ", "insta ", "tiktok ", "twitter ", "reddit "}
	for _, p := range dlPrefixes {
		if strings.HasPrefix(lower, p) {
			urlStr := strings.TrimSpace(trimmed[len(p):])
			if socialURLRe.MatchString(urlStr) {
				return &UserIntent{
					Intent:     "download",
					Confidence: 0.99,
					Parameters: map[string]string{"url": urlStr},
					Reasoning:  "fast-path download trigger",
				}
			}
		}
	}

	// 13. Ping / Status / Uptime intent
	if lower == "ping" || lower == "uptime" || lower == "bot status" || lower == "status" || lower == "are you alive" {
		return &UserIntent{
			Intent:     "status",
			Confidence: 0.99,
			Reasoning:  "fast-path status trigger",
		}
	}

	// 14. Roast intent
	if lower == "roast me" || lower == "roast" || lower == "cook me" || lower == "cook him" || lower == "burn" ||
		strings.HasPrefix(lower, "roast ") || strings.HasPrefix(lower, "cook ") || strings.HasPrefix(lower, "burn ") {
		target := ""
		for _, p := range []string{"roast ", "cook ", "burn "} {
			if strings.HasPrefix(lower, p) {
				target = strings.TrimSpace(trimmed[len(p):])
				break
			}
		}
		return &UserIntent{
			Intent:     "roast",
			Confidence: 0.99,
			Parameters: map[string]string{"target": target},
			Reasoning:  "fast-path roast trigger",
		}
	}

	// 15. Brainrot intent
	if lower == "brainrot" || lower == "yap" || lower == "skibidi" ||
		strings.HasPrefix(lower, "brainrot ") || strings.HasPrefix(lower, "yap ") || strings.HasPrefix(lower, "skibidi ") {
		topic := ""
		for _, p := range []string{"brainrot ", "yap ", "skibidi "} {
			if strings.HasPrefix(lower, p) {
				topic = strings.TrimSpace(trimmed[len(p):])
				break
			}
		}
		return &UserIntent{
			Intent:     "brainrot",
			Confidence: 0.99,
			Parameters: map[string]string{"topic": topic},
			Reasoning:  "fast-path brainrot trigger",
		}
	}

	return nil
}

// ClassifyIntentWithGemini invokes Gemini to evaluate complex or conversational user intent
func ClassifyIntentWithGemini(ctx context.Context, prompt string, hasMedia, hasQuotedMedia bool, quotedText string) (*UserIntent, error) {
	if gemini.GlobalClient == nil {
		return nil, fmt.Errorf("gemini client not initialized")
	}

	systemInstruction := `You are PrismBot's Intelligent Intent Router on WhatsApp.
Your job is to analyze the user's message and determine whether the user intends to execute a real bot action or tool, or if they just want general conversation / Q&A / advice.

Available Actionable Intents:
- "play": User asks to play, download, stream, or listen to a song, music, track, or audio (e.g., "play fade", "play bohemian rhapsody", "song stay with me"). Parameter: "query".
- "video": User asks to play, download, or watch a video (e.g., "video alan walker fade", "play video ..."). Parameter: "query".
- "catchup": User wants a summary or recap of missed messages / recent chat while they were away or asleep.
- "remind": User wants the bot to schedule a real reminder or timer for them. Parameters: "duration" (format like "10m", "2h", "30s", "1d"), "message" (what to remind them of).
- "sticker": User wants to create or convert an image/video to a WhatsApp sticker.
- "ocr": User wants to extract, scan, or read text from an image.
- "viewonce": User wants to reveal, open, or resend a view-once photo or video.
- "weather": User wants current weather or forecast for a specific city/location. Parameter: "city".
- "calc": User specifically asks to calculate a mathematical formula, arithmetic, or unit conversion. Parameter: "expression".
- "define": User asks for dictionary definition, meaning, or pronunciation of a specific word or short idiom. Parameter: "word".
- "translate": User wants text translated to another language. Parameters: "target_lang", "text".
- "summarize": User specifically requests a summary of an article, long text, or quoted message. Parameter: "text".
- "notes_list": User wants to view, list, or navigate their saved notes or notes tree.
- "notes_add": User wants to save or add a new note. Parameters: "note_path", "note_content".
- "notes_get": User wants to view/read a specific saved note. Parameter: "note_path".
- "deleted": User asks to see recently deleted or revoked messages.
- "transcribe": User wants audio or voice message transcribed to text.
- "imagine": User asks to generate/draw an AI image. Parameter: "prompt".
- "status": User asks if bot is running/alive, bot ping, or bot uptime.
- "chat": User is having a conversation, asking knowledge questions, seeking advice, asking for code, or roleplaying. ALSO choose "chat" if the user is merely asking *about* a feature (e.g. "what can you do?", "how do reminders work?") rather than asking to perform it.

CRITICAL INSTRUCTIONS:
- You must output strictly valid JSON matching this schema:
{
  "intent": "<one of the intents above>",
  "confidence": <float between 0.0 and 1.0>,
  "parameters": {
    "<key>": "<value>"
  },
  "reasoning": "<short explanation>"
}
- If in doubt, or if the user is asking a general question, default to "chat" with confidence 1.0.
- Do NOT wrap JSON in markdown code blocks if possible, just return raw JSON.`

	contextHint := ""
	if hasMedia || hasQuotedMedia {
		contextHint += "\n[Media Attachment Present: Yes]"
	}
	if quotedText != "" {
		contextHint += fmt.Sprintf("\n[Quoted Text in Message: \"%s\"]", quotedText)
	}

	fullPrompt := fmt.Sprintf("Analyze this user message for intent:%s\n\n\"%s\"", contextHint, prompt)

	req := gemini.GenerateRequest{
		Contents: []gemini.Content{
			{
				Role:  "user",
				Parts: []gemini.Part{{Text: fullPrompt}},
			},
		},
		SystemInstruction: &gemini.SystemInstruction{
			Parts: []gemini.Part{{Text: systemInstruction}},
		},
		GenerationConfig: &gemini.GenerationConfig{
			Temperature:     0.1,
			MaxOutputTokens: 300,
		},
	}

	respStr, err := gemini.GlobalClient.Generate(ctx, req)
	if err != nil {
		return nil, err
	}

	cleaned := strings.TrimSpace(respStr)
	// Remove markdown backticks if returned
	if strings.HasPrefix(cleaned, "```json") {
		cleaned = strings.TrimPrefix(cleaned, "```json")
		cleaned = strings.TrimSuffix(cleaned, "```")
		cleaned = strings.TrimSpace(cleaned)
	} else if strings.HasPrefix(cleaned, "```") {
		cleaned = strings.TrimPrefix(cleaned, "```")
		cleaned = strings.TrimSuffix(cleaned, "```")
		cleaned = strings.TrimSpace(cleaned)
	}

	var intent UserIntent
	if err := json.Unmarshal([]byte(cleaned), &intent); err != nil {
		log.Printf("[IntentRouter] WARN: Failed to parse Gemini intent JSON: %v. Raw: %s", err, cleaned)
		return &UserIntent{Intent: "chat", Confidence: 0.5}, nil
	}

	if intent.Parameters == nil {
		intent.Parameters = make(map[string]string)
	}

	return &intent, nil
}

// EvaluateUserIntent combines fast-path heuristics and Gemini semantic intent classification
func EvaluateUserIntent(ctx context.Context, c *CommandContext, cleanPrompt string) (*UserIntent, error) {
	extCtx := extractContextAndMedia(c)
	hasMedia := len(extCtx.MediaData) > 0
	hasQuotedMedia := false
	hasQuotedText := extCtx.QuotedText != ""

	// Check if quoted message has media
	if c.RawMsg != nil && c.RawMsg.Message != nil {
		if ext := c.RawMsg.Message.GetExtendedTextMessage(); ext != nil {
			if q := ext.GetContextInfo().GetQuotedMessage(); q != nil {
				if q.GetImageMessage() != nil || q.GetVideoMessage() != nil || q.GetAudioMessage() != nil || q.GetViewOnceMessage() != nil || q.GetViewOnceMessageV2() != nil {
					hasQuotedMedia = true
				}
			}
		}
	}

	// 1. Check Fast-Path Heuristics
	fastIntent := FastPathIntent(cleanPrompt, hasMedia, hasQuotedMedia, hasQuotedText)
	if fastIntent != nil && fastIntent.Confidence >= 0.95 {
		log.Printf("[IntentRouter] Fast-path detected intent '%s' (conf: %.2f): %s", fastIntent.Intent, fastIntent.Confidence, fastIntent.Reasoning)
		return fastIntent, nil
	}

	// In self style or if style is persona-driven, conversational text is the primary intent.
	// Skip slow LLM intent classification to prevent context deadline exceeded errors.
	if c != nil && gemini.GlobalClient != nil {
		chatKey := c.ChatJID.ToNonAD().String()
		if gemini.GlobalClient.GetChatStyle(chatKey) == "self" {
			return &UserIntent{Intent: "chat", Confidence: 1.0}, nil
		}
	}

	// 2. Evaluate with Gemini
	geminiCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	intent, err := ClassifyIntentWithGemini(geminiCtx, cleanPrompt, hasMedia, hasQuotedMedia, extCtx.QuotedText)
	if err != nil {
		log.Printf("[IntentRouter] Gemini classification error: %v. Falling back to chat.", err)
		return &UserIntent{Intent: "chat", Confidence: 0.5}, nil
	}

	log.Printf("[IntentRouter] Gemini classified intent '%s' (conf: %.2f): %s", intent.Intent, intent.Confidence, intent.Reasoning)
	return intent, nil
}

// DispatchIntent executes the appropriate bot capability based on the classified intent
func DispatchIntent(c *CommandContext, intent *UserIntent) (bool, error) {
	if intent == nil || intent.Intent == "chat" || intent.Confidence < 0.70 {
		return false, nil // Fall through to standard conversational AI chat
	}

	log.Printf("[IntentRouter] Executing intent '%s' with parameters %v", intent.Intent, intent.Parameters)

	switch intent.Intent {
	case "catchup":
		if countStr, ok := intent.Parameters["count"]; ok && countStr != "" {
			c.RawText = "!catchup " + countStr
		} else {
			c.RawText = "!catchup"
		}
		return handleCatchUp(c)

	case "remind":
		durStr := intent.Parameters["duration"]
		msgText := intent.Parameters["message"]
		if durStr == "" {
			durStr = "10m"
		}
		if msgText == "" {
			msgText = "Reminder from Prism"
		}

		dur, err := parseDuration(strings.ToLower(durStr))
		if err != nil {
			return true, c.ReplyText(fmt.Sprintf("⏰ I recognized you want a reminder, but couldn't parse the duration '%s'. Please use formats like 10m, 2h, 30s.", durStr))
		}

		targetTime := time.Now().Add(dur)
		item := CreateReminderFromDashboard(c.ChatJID.String(), c.SenderJID.String(), msgText, targetTime, c.Client)
		_ = c.React("⏰")
		return true, c.ReplyText(fmt.Sprintf("⏰ *Reminder Scheduled!*\n\n📝 *Message:* %s\n⏳ *In:* %s\n🔔 *Trigger At:* %s", item.Message, dur, targetTime.Format("15:04:05")))

	case "sticker":
		extCtx := extractContextAndMedia(c)
		hasImg := extCtx.IsImage || (c.RawMsg != nil && c.RawMsg.Message != nil && c.RawMsg.Message.GetImageMessage() != nil)
		if !hasImg && c.RawMsg != nil && c.RawMsg.Message != nil {
			if ext := c.RawMsg.Message.GetExtendedTextMessage(); ext != nil {
				if q := ext.GetContextInfo().GetQuotedMessage(); q != nil && q.GetImageMessage() != nil {
					hasImg = true
				}
			}
		}

		if !hasImg {
			_ = c.React("ℹ️")
			return true, c.ReplyText("📸 Please send or reply to an image with your request to convert it into a sticker!")
		}
		return handleSticker(c)

	case "ocr":
		extCtx := extractContextAndMedia(c)
		hasImg := extCtx.IsImage || (c.RawMsg != nil && c.RawMsg.Message != nil && c.RawMsg.Message.GetImageMessage() != nil)
		if !hasImg && c.RawMsg != nil && c.RawMsg.Message != nil {
			if ext := c.RawMsg.Message.GetExtendedTextMessage(); ext != nil {
				if q := ext.GetContextInfo().GetQuotedMessage(); q != nil && q.GetImageMessage() != nil {
					hasImg = true
				}
			}
		}

		if !hasImg {
			_ = c.React("ℹ️")
			return true, c.ReplyText("🔍 Please send or reply to an image to extract text with OCR!")
		}
		return handleOCR(c)

	case "viewonce":
		return handleViewOnce(c)

	case "weather":
		city := intent.Parameters["city"]
		if city == "" {
			return true, c.ReplyText("🌤️ Which city would you like the weather forecast for?")
		}
		c.RawText = "!weather " + city
		return handleWeather(c)

	case "calc":
		expr := intent.Parameters["expression"]
		if expr == "" {
			expr = strings.TrimSpace(c.RawText)
		}
		c.RawText = "!calc " + expr
		return handleCalc(c)

	case "define":
		word := intent.Parameters["word"]
		if word == "" {
			return true, c.ReplyText("📖 What word would you like me to define?")
		}
		c.RawText = "!define " + word
		return handleDefine(c)

	case "translate":
		targetLang := intent.Parameters["target_lang"]
		textToTranslate := intent.Parameters["text"]
		if targetLang == "" {
			targetLang = "English"
		}
		if textToTranslate != "" {
			c.RawText = fmt.Sprintf("!tr %s %s", targetLang, textToTranslate)
		} else {
			c.RawText = "!tr " + targetLang
		}
		return handleTranslate(c)

	case "summarize":
		text := intent.Parameters["text"]
		if text != "" {
			c.RawText = "!summarize " + text
		} else {
			c.RawText = "!summarize"
		}
		return handleSummarize(c)

	case "notes_list":
		return handleNotesCommand(c)

	case "notes_add":
		notePath := intent.Parameters["note_path"]
		noteContent := intent.Parameters["note_content"]
		if notePath == "" {
			notePath = fmt.Sprintf("quicknote_%d", time.Now().Unix()%10000)
		}
		c.RawText = fmt.Sprintf("!note %s %s", notePath, noteContent)
		return handleAddNoteCommand(c)

	case "notes_get":
		notePath := intent.Parameters["note_path"]
		if notePath == "" {
			return true, c.ReplyText("📁 Which note would you like to retrieve?")
		}
		c.RawText = "!getnote " + notePath
		return handleGetNoteCommand(c)

	case "deleted":
		return handleDeletedCommands(c)

	case "transcribe":
		return handleTranscribe(c)

	case "imagine":
		prompt := intent.Parameters["prompt"]
		if prompt == "" {
			return true, c.ReplyText("🎨 What would you like me to imagine? (e.g., 'imagine a futuristic cyberpunk city')")
		}
		c.RawText = "!imagine " + prompt
		return handleImagine(c)

	case "status":
		return handleStatus(c)

	case "play":
		query := intent.Parameters["query"]
		if query == "" {
			return true, c.ReplyText("🎵 Which song would you like me to play? (e.g., 'play fade')")
		}
		c.RawText = "!play " + query
		return handlePlayMusic(c)

	case "party":
		query := intent.Parameters["query"]
		if query != "" {
			c.RawText = "!party " + query
		} else {
			c.RawText = "!party status"
		}
		return handlePartyCommand(c)

	case "video":
		query := intent.Parameters["query"]
		if query == "" {
			return true, c.ReplyText("🎥 Which video would you like me to play? (e.g., 'video fade')")
		}
		c.RawText = "!video " + query
		return handlePlayVideo(c)

	case "roast":
		target := intent.Parameters["target"]
		if target != "" {
			c.RawText = "!roast " + target
		} else {
			c.RawText = "!roast"
		}
		return handleRoast(c)

	case "brainrot":
		topic := intent.Parameters["topic"]
		if topic != "" {
			c.RawText = "!brainrot " + topic
		} else {
			c.RawText = "!brainrot"
		}
		return handleBrainrot(c)

	case "download":
		urlStr := intent.Parameters["url"]
		if urlStr != "" {
			c.RawText = "!dl " + urlStr
		} else {
			c.RawText = "!dl"
		}
		return handleSocialDownload(c)

	default:
		return false, nil
	}
}
