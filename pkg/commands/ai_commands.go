package commands

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/polymorfa/hypermeow"
	waProto "github.com/polymorfa/hypermeow/binary/proto"
	"google.golang.org/protobuf/proto"

	"whatsapp-bot/pkg/config"
	"whatsapp-bot/pkg/gemini"
	"whatsapp-bot/pkg/renderer"
	"whatsapp-bot/pkg/store"
)

func RegisterAICommands(r *CommandRegistry) {
	r.RegisterPrefix("@prism", HandleAIChat)
	r.RegisterPrefix("@bot", HandleAIChat)
	r.RegisterPrefix("!ai", HandleAIChat)
	r.RegisterPrefix("!ask", HandleAIChat)
	r.RegisterPrefix("!solve", HandleAIChat)
	r.RegisterPrefix("!explain", HandleAIChat)
	r.RegisterPrefix("!q", HandleAIChat)
	r.RegisterPrefix("!question", HandleAIChat)

	r.RegisterPrefix("!catchup", handleCatchUp)
	r.RegisterPrefix("!recap", handleCatchUp)

	r.RegisterPrefix("!ocr", handleOCR)
	r.RegisterPrefix("!read", handleOCR)
	r.RegisterPrefix("!scan", handleOCR)

	r.RegisterPrefix("!sum", handleSummarize)
	r.RegisterPrefix("!summarize", handleSummarize)
	r.RegisterPrefix("!tldr", handleSummarize)

	r.RegisterPrefix("!tr", handleTranslate)
	r.RegisterPrefix("!translate", handleTranslate)

	r.RegisterPrefix("!calc", handleCalc)
	r.RegisterPrefix("!math", handleCalc)

	r.RegisterPrefix("!weather", handleWeather)
	r.RegisterPrefix("!forecast", handleWeather)

	r.RegisterPrefix("!define", handleDefine)
	r.RegisterPrefix("!dict", handleDefine)


	r.RegisterPrefix("!fact", handleFact)
	r.RegisterPrefix("!mindblown", handleFact)
	r.RegisterPrefix("!quote", handleQuote)
	r.RegisterPrefix("!motivation", handleQuote)

	r.RegisterPrefix("!roast", handleRoast)
	r.RegisterPrefix("!cook", handleRoast)
	r.RegisterPrefix("!burn", handleRoast)
	r.RegisterPrefix("!brainrot", handleBrainrot)
	r.RegisterPrefix("!yap", handleBrainrot)
	r.RegisterPrefix("!skibidi", handleBrainrot)

	r.RegisterPrefix("!aimode", handleAIMode)
	r.RegisterPrefix("!provider", handleAIMode)
	r.RegisterPrefix("!aimodel", handleAIModel)
	r.RegisterPrefix("!model", handleAIModel)
	r.RegisterPrefix("!style", handleAIStyle)
	r.RegisterPrefix("!prismstyle", handleAIStyle)
	r.RegisterPrefix("!persona_style", handleAIStyle)

	for pKey, pCfg := range config.AppConfig.Personas {
		key := pKey
		cfg := pCfg
		r.RegisterPrefix(cfg.Prefix, func(c *CommandContext) (bool, error) {
			return handlePersona(c, key, cfg)
		})
	}
}

type ExtractedContext struct {
	MediaData  []byte
	MimeType   string
	IsImage    bool
	IsAudio    bool
	QuotedText string
}

func extractContextAndMedia(c *CommandContext) *ExtractedContext {
	res := &ExtractedContext{}
	if c.RawMsg == nil || c.RawMsg.Message == nil {
		return res
	}

	raw := c.RawMsg.Message
	var targetMedia whatsmeow.DownloadableMessage
	var targetMime string

	// 1. Direct media on current message
	if img := raw.GetImageMessage(); img != nil {
		targetMedia = img
		targetMime = img.GetMimetype()
		if targetMime == "" {
			targetMime = "image/jpeg"
		}
		res.IsImage = true
	} else if doc := raw.GetDocumentMessage(); doc != nil && isImageDocument(doc) {
		targetMedia = doc
		targetMime = doc.GetMimetype()
		if targetMime == "" {
			targetMime = "image/jpeg"
		}
		res.IsImage = true
	} else if aud := raw.GetAudioMessage(); aud != nil {
		targetMedia = aud
		targetMime = aud.GetMimetype()
		if targetMime == "" {
			targetMime = "audio/ogg"
		}
		res.IsAudio = true
	} else if vo := raw.GetViewOnceMessage(); vo != nil && vo.GetMessage() != nil {
		if vImg := vo.GetMessage().GetImageMessage(); vImg != nil {
			targetMedia = vImg
			targetMime = vImg.GetMimetype()
			res.IsImage = true
		}
	} else if vo2 := raw.GetViewOnceMessageV2(); vo2 != nil && vo2.GetMessage() != nil {
		if vImg := vo2.GetMessage().GetImageMessage(); vImg != nil {
			targetMedia = vImg
			targetMime = vImg.GetMimetype()
			res.IsImage = true
		}
	}

	// 2. Quoted message in ContextInfo
	var ctxInfo *waProto.ContextInfo
	if ext := raw.GetExtendedTextMessage(); ext != nil {
		ctxInfo = ext.GetContextInfo()
	} else if img := raw.GetImageMessage(); img != nil {
		ctxInfo = img.GetContextInfo()
	} else if doc := raw.GetDocumentMessage(); doc != nil {
		ctxInfo = doc.GetContextInfo()
	} else if aud := raw.GetAudioMessage(); aud != nil {
		ctxInfo = aud.GetContextInfo()
	}

	if ctxInfo != nil {
		quoted := ctxInfo.GetQuotedMessage()
		if quoted != nil {
			// Extract quoted text
			if quoted.Conversation != nil && *quoted.Conversation != "" {
				res.QuotedText = *quoted.Conversation
			} else if quoted.ExtendedTextMessage != nil && quoted.ExtendedTextMessage.Text != nil {
				res.QuotedText = *quoted.ExtendedTextMessage.Text
			} else if quoted.ImageMessage != nil && quoted.ImageMessage.Caption != nil {
				res.QuotedText = *quoted.ImageMessage.Caption
			} else if quoted.VideoMessage != nil && quoted.VideoMessage.Caption != nil {
				res.QuotedText = *quoted.VideoMessage.Caption
			}

			// Extract quoted media if direct media not already found
			if targetMedia == nil {
				if qImg := quoted.GetImageMessage(); qImg != nil {
					targetMedia = qImg
					targetMime = qImg.GetMimetype()
					if targetMime == "" {
						targetMime = "image/jpeg"
					}
					res.IsImage = true
				} else if qDoc := quoted.GetDocumentMessage(); qDoc != nil && isImageDocument(qDoc) {
					targetMedia = qDoc
					targetMime = qDoc.GetMimetype()
					if targetMime == "" {
						targetMime = "image/jpeg"
					}
					res.IsImage = true
				} else if qAud := quoted.GetAudioMessage(); qAud != nil {
					targetMedia = qAud
					targetMime = qAud.GetMimetype()
					if targetMime == "" {
						targetMime = "audio/ogg"
					}
					res.IsAudio = true
				} else if qVo := quoted.GetViewOnceMessage(); qVo != nil && qVo.GetMessage() != nil {
					if qvImg := qVo.GetMessage().GetImageMessage(); qvImg != nil {
						targetMedia = qvImg
						targetMime = qvImg.GetMimetype()
						res.IsImage = true
					}
				} else if qVo2 := quoted.GetViewOnceMessageV2(); qVo2 != nil && qVo2.GetMessage() != nil {
					if qvImg := qVo2.GetMessage().GetImageMessage(); qvImg != nil {
						targetMedia = qvImg
						targetMime = qvImg.GetMimetype()
						res.IsImage = true
					}
				}
			}
		}

		// 3. Fallback to in-memory store cache if stanzaId is known
		if targetMedia == nil && ctxInfo.GetStanzaID() != "" {
			if cached, ok := store.GlobalStore.Get(ctxInfo.GetStanzaID()); ok && cached.Raw != nil && cached.Raw.Message != nil {
				cMsg := cached.Raw.Message
				if cImg := cMsg.GetImageMessage(); cImg != nil {
					targetMedia = cImg
					targetMime = cImg.GetMimetype()
					res.IsImage = true
				} else if cDoc := cMsg.GetDocumentMessage(); cDoc != nil && isImageDocument(cDoc) {
					targetMedia = cDoc
					targetMime = cDoc.GetMimetype()
					res.IsImage = true
				}
				if res.QuotedText == "" && cached.Text != "" {
					res.QuotedText = cached.Text
				}
			}
		}
	}

	// Download media bytes if target found
	if targetMedia != nil && c.Client != nil {
		data, err := c.Client.Download(c.Ctx, targetMedia)
		if err == nil && len(data) > 0 {
			res.MediaData = data
			res.MimeType = targetMime
		}
	}

	return res
}

func isImageDocument(doc *waProto.DocumentMessage) bool {
	if doc == nil {
		return false
	}
	m := strings.ToLower(doc.GetMimetype())
	if strings.HasPrefix(m, "image/") {
		return true
	}
	fn := strings.ToLower(doc.GetFileName())
	return strings.HasSuffix(fn, ".png") || strings.HasSuffix(fn, ".jpg") || strings.HasSuffix(fn, ".jpeg") || strings.HasSuffix(fn, ".webp")
}

func stripAITriggers(raw string, cli *whatsmeow.Client) string {
	trimmed := strings.TrimSpace(raw)

	if cli != nil && cli.Store != nil && cli.Store.ID != nil {
		botUser := cli.Store.ID.ToNonAD().User
		if botUser != "" {
			trimmed = strings.ReplaceAll(trimmed, "@"+botUser, "")
		}
	}

	lower := strings.ToLower(trimmed)
	prefixes := []string{"@prism", "@bot", "!ai", "!ask", "!solve", "!explain", "!q", "!question"}
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			trimmed = strings.TrimSpace(trimmed[len(p):])
			lower = strings.ToLower(trimmed)
		}
	}

	trimmed = strings.ReplaceAll(trimmed, "@prism", "")
	trimmed = strings.ReplaceAll(trimmed, "@Prism", "")
	trimmed = strings.ReplaceAll(trimmed, "@PRISM", "")
	trimmed = strings.ReplaceAll(trimmed, "@bot", "")
	trimmed = strings.ReplaceAll(trimmed, "@Bot", "")
	trimmed = strings.ReplaceAll(trimmed, "@BOT", "")

	trimmed = strings.TrimLeft(strings.TrimSpace(trimmed), ":,-? ")

	return strings.TrimSpace(trimmed)
}

// isProblemSolvingIntent detects whether the user's input indicates intent to solve a mathematical,
// scientific, homework, or analytical problem, where background chat messages context should be ignored.
func isProblemSolvingIntent(c *CommandContext, cleanPrompt string, extCtx *ExtractedContext) bool {
	rawLower := strings.ToLower(strings.TrimSpace(c.RawText))
	cleanLower := strings.ToLower(strings.TrimSpace(cleanPrompt))

	// 1. Explicit command prefixes
	if strings.HasPrefix(rawLower, "!solve") ||
		strings.HasPrefix(rawLower, "!math") ||
		strings.HasPrefix(rawLower, "!calc") ||
		strings.HasPrefix(rawLower, "!calculate") ||
		strings.HasPrefix(rawLower, "!equation") {
		return true
	}

	// 2. Clear problem solving intent in natural language
	solvePrefixes := []string{
		"solve", "calculate", "evaluate", "compute", "simplify",
		"find the value", "find the solution", "solve the equation",
		"solve for", "prove that", "derive", "find derivative", "find integral",
	}
	for _, p := range solvePrefixes {
		if strings.HasPrefix(cleanLower, p+" ") || cleanLower == p {
			return true
		}
	}

	solvePhrases := []string{
		"solve this", "solve problem", "solve the problem", "solve equation",
		"can you solve", "please solve", "help me solve", "step by step solution",
		"solve the following", "show the steps to solve", "find x in",
	}
	for _, phrase := range solvePhrases {
		if strings.Contains(cleanLower, phrase) {
			return true
		}
	}

	// 3. Media attached with problem-solving intention
	// If an image is sent without text or with prompt asking to solve/answer
	if extCtx != nil && len(extCtx.MediaData) > 0 {
		if cleanPrompt == "" || strings.Contains(cleanLower, "solve") || strings.Contains(cleanLower, "solution") || strings.Contains(cleanLower, "answer") {
			return true
		}
	}

	return false
}

// buildRecentChatContext gathers up to `limit` recent messages strictly for the specific chat,
// formatted compactly and safely truncated to prevent context overload.
func buildRecentChatContext(chatJID string, excludeID string, limit int) string {
	if chatJID == "" {
		return ""
	}

	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	// 1. Try SQLite persistent store first
	if store.GlobalPersistent != nil {
		records := store.GetRecentChatHistoryPersistent(chatJID, limit, excludeID)
		if len(records) > 0 {
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("=== RECENT CHAT HISTORY (Last %d messages in this chat) ===\n", len(records)))
			for _, m := range records {
				sender := m.SenderName
				if sender == "" {
					if m.IsFromMe {
						sender = "You"
					} else {
						sender = strings.Split(m.SenderJID, "@")[0]
					}
				}
				timeStr := m.Timestamp.Format("15:04")
				text := strings.TrimSpace(m.Text)
				if len(text) > 200 {
					text = text[:197] + "..."
				}
				sb.WriteString(fmt.Sprintf("[%s] %s: %s\n", timeStr, sender, text))
			}
			sb.WriteString("=== END OF RECENT CHAT HISTORY ===")
			return sb.String()
		}
	}

	// 2. Fallback to in-memory store
	if store.GlobalStore != nil {
		messages := store.GlobalStore.GetRecentChatHistory(chatJID, limit, excludeID)
		if len(messages) > 0 {
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("=== RECENT CHAT HISTORY (Last %d messages in this chat) ===\n", len(messages)))
			for _, m := range messages {
				sender := m.PushName
				if sender == "" {
					sender = store.GlobalStore.GetContact(m.SenderJID.String())
				}
				if sender == "" {
					sender = strings.Split(m.SenderJID.User, "@")[0]
				}
				if sender == "" {
					sender = "User"
				}

				timeStr := m.Timestamp.Format("15:04")
				text := strings.TrimSpace(m.Text)
				// Truncate long individual messages to 200 characters to prevent context overload
				if len(text) > 200 {
					text = text[:197] + "..."
				}

				if text != "" && m.MediaType != "" {
					sb.WriteString(fmt.Sprintf("[%s] %s: [%s] %s\n", timeStr, sender, m.MediaType, text))
				} else if text != "" {
					sb.WriteString(fmt.Sprintf("[%s] %s: %s\n", timeStr, sender, text))
				} else if m.MediaType != "" {
					sb.WriteString(fmt.Sprintf("[%s] %s: [%s]\n", timeStr, sender, m.MediaType))
				}
			}
			sb.WriteString("=== END OF RECENT CHAT HISTORY ===")
			return sb.String()
		}
	}

	return ""
}

// buildSelfPersonaInstruction gathers past user messages from SQLite for this specific chat,
// filters out bot responses and commands, and constructs a dynamic persona prompt so the AI
// mimics how the user texts in this exact chat.
func buildSelfPersonaInstruction(chatKey, excludeID string) string {
	userName := ""
	if store.GlobalPersistent != nil {
		userName = store.GetUserName()
	}
	if userName == "" {
		userName = "the user"
	}

	userMsgs := store.GetChatUserMessages(chatKey, 15, excludeID)
	if len(userMsgs) == 0 {
		if config.AppConfig != nil && config.AppConfig.BaseInstructionSelf != "" {
			return config.AppConfig.BaseInstructionSelf
		}
		return fmt.Sprintf(`You are texting as %s themselves in this WhatsApp chat (Persona: Self User).
No prior chat history is recorded for this chat yet. Text naturally, casually, and authentically like %s texting on WhatsApp. Keep your reply concise, casual, and direct. Never sound like an AI assistant or corporate chatbot.`, userName, userName)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("You are texting as %s themselves in this WhatsApp chat (Persona: Self User).\n", userName))
	sb.WriteString(fmt.Sprintf("You must seamlessly emulate how %s texts in THIS chat. Adapt your persona, tone, slang, vocabulary, capitalization, and punctuation to match %s's authentic message history shown below.\n\n", userName, userName))
	sb.WriteString(fmt.Sprintf("=== AUTHENTIC MESSAGES WRITTEN BY %s (Total %d samples) ===\n", strings.ToUpper(userName), len(userMsgs)))
	for idx, m := range userMsgs {
		text := strings.TrimSpace(m.Text)
		if len(text) > 180 {
			text = text[:177] + "..."
		}
		sb.WriteString(fmt.Sprintf("%d. %q\n", idx+1, text))
	}
	sb.WriteString("=============================================================================\n\n")
	sb.WriteString(fmt.Sprintf(`Stylistic Persona Directives:
1. Emulate %s's exact texting style and personality shown above: notice if they text in lowercase, use abbreviations (e.g. u, ur, rn, lol, lmao, idk, fr, wya, hbu), use slang or regional dialect, and how they use punctuation or emojis.
2. Mirror %s's typical message length (e.g. short 1-line replies vs longer texts).
3. Do NOT sound like an AI assistant, customer service bot, or formal chatbot. Never say "Certainly!", "Sure thing!", "I'm here to help", or "As an AI...".
4. Text naturally as %s replying to the conversation.`, userName, userName, userName))

	return sb.String()
}

func HandleAIChat(c *CommandContext) (bool, error) {
	_ = c.React("🧠")
	cleanPrompt := stripAITriggers(c.RawText, c.Client)

	// Intelligently evaluate user intent for real command execution
	if cleanPrompt != "" {
		intent, err := EvaluateUserIntent(c.Ctx, c, cleanPrompt)
		if err == nil && intent != nil && intent.Intent != "chat" && intent.Confidence >= 0.70 {
			handled, execErr := DispatchIntent(c, intent)
			if handled || execErr != nil {
				return handled, execErr
			}
		}
	}

	extCtx := extractContextAndMedia(c)
	isSolve := isProblemSolvingIntent(c, cleanPrompt, extCtx)

	chatKey := c.ChatJID.ToNonAD().String()
	excludeID := ""
	if c.RawMsg != nil {
		excludeID = c.RawMsg.Info.ID
	}

	targetSender := "User"
	if c.RawMsg != nil && c.RawMsg.Info.PushName != "" {
		targetSender = c.RawMsg.Info.PushName
	} else if c.SenderJID.User != "" {
		targetSender = c.SenderJID.User
	}

	targetPrompt := cleanPrompt
	if targetPrompt == "" && extCtx.QuotedText != "" {
		targetPrompt = extCtx.QuotedText
	} else if targetPrompt == "" && store.GlobalStore != nil {
		// When triggered via @prism with no text, find the latest message from the other person to reply to
		recent := store.GlobalStore.GetRecentChatHistory(chatKey, 5, excludeID)
		for i := len(recent) - 1; i >= 0; i-- {
			if recent[i] != nil && strings.TrimSpace(recent[i].Text) != "" {
				targetPrompt = strings.TrimSpace(recent[i].Text)
				if recent[i].PushName != "" {
					targetSender = recent[i].PushName
				}
				break
			}
		}
	}
	if targetPrompt == "" {
		targetPrompt = "Hello"
	}

	finalPrompt := targetPrompt
	if len(extCtx.MediaData) > 0 {
		if cleanPrompt == "" {
			finalPrompt = "Please solve the question or problem shown in this image in detail with clear step-by-step reasoning and the final answer."
		} else {
			finalPrompt = fmt.Sprintf("%s\n\n(Please carefully examine the attached screenshot/image to answer or solve this question.)", cleanPrompt)
		}
	} else if extCtx.QuotedText != "" {
		if cleanPrompt == "" {
			finalPrompt = fmt.Sprintf("[Quoted Message to reply to from %s]:\n\"%s\"", targetSender, extCtx.QuotedText)
		} else {
			finalPrompt = fmt.Sprintf("[Quoted Context]:\n\"%s\"\n\n[Latest Message to reply to from %s]:\n\"%s\"", extCtx.QuotedText, targetSender, cleanPrompt)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var reply string
	var err error

	if isSolve {
		// When the user intends to solve a problem:
		// IGNORE chat messages context and previous chat history completely to prevent context overload.
		solveInstruction := "You are an expert mathematician, scientist, and academic tutor. When solving problems, provide a rigorous and clean step-by-step explanation. Structure each step with clear bold titles (e.g. **Step 1: Given & Formula**, **Step 2: Substitution**, **Final Answer**). Format all mathematical equations on standalone lines using standard LaTeX display blocks: $$<latex_equation>$$ or \\begin{aligned}...\\end{aligned}. Ensure all LaTeX syntax is valid and easy to read."

		reply, err = gemini.GlobalClient.SolveProblemForChat(ctx, c.ChatJID.String(), finalPrompt, solveInstruction, extCtx.MimeType, extCtx.MediaData)
		if err != nil {
			log.Printf("[AI] Solve problem error: %v", err)
			_ = c.React("❌")
			return true, nil
		}
		return true, sendMathOrTextResponse(c, reply, "📐 *[Math Solution]*")
	}

	// For conversational / general queries:
	// Pass the context of the specific chat only (last 50 messages) in the system instruction
	chatContext := buildRecentChatContext(chatKey, excludeID, 50)

	activeStyle := "formal"
	if gemini.GlobalClient != nil {
		activeStyle = gemini.GlobalClient.GetChatStyle(chatKey)
	}

	instruction := config.AppConfig.BaseInstructionFormal
	contextNote := fmt.Sprintf("Background chat history is provided above for conversation context only. You must respond directly to the latest message from %s (\"%s\"). Keep your reply direct, concise, and helpful.", targetSender, targetPrompt)
	promptToSend := finalPrompt

	if activeStyle == "genz" {
		instruction = config.AppConfig.BaseInstructionGenZ
		contextNote = fmt.Sprintf("Background chat history is provided above for conversation context only. Stay 100%% in your witty, hilarious, sarcastic Gen-Z persona, replying directly to the latest message from %s (\"%s\"). Never slip into corporate or robotic polite tones.", targetSender, targetPrompt)
		if chatContext != "" {
			instruction = fmt.Sprintf("%s\n\n%s\n\n%s", instruction, chatContext, contextNote)
		}
		if len(extCtx.MediaData) == 0 && extCtx.QuotedText == "" {
			promptToSend = fmt.Sprintf(`[Latest Message from %s to reply to]: "%s"`, targetSender, targetPrompt)
		}
	} else if activeStyle == "self" {
		instruction = buildSelfPersonaInstruction(chatKey, excludeID)
		// Clear synthetic history so previous turns don't invert roles or target stale messages
		if gemini.GlobalClient != nil {
			gemini.GlobalClient.ClearHistory(chatKey)
		}

		userName := ""
		if store.GlobalPersistent != nil {
			userName = store.GetUserName()
		}
		if userName == "" {
			userName = "the user"
		}

		// When in self style, send the message history directly in the prompt
		var pb strings.Builder
		if chatContext != "" {
			pb.WriteString(chatContext)
			pb.WriteString("\n\n")
		}
		if extCtx.QuotedText != "" {
			pb.WriteString(fmt.Sprintf("[Replying to Quoted Message]: \"%s\"\n\n", extCtx.QuotedText))
		}

		if c.RawMsg != nil && c.RawMsg.Info.IsFromMe {
			// User themselves invoked the bot
			pb.WriteString(fmt.Sprintf("[Directive]: You are %s. Text directly as %s replying to: %q in %s's authentic texting persona. Output ONLY the response text as %s with no quotes, prefixes, or commentary.", userName, userName, targetPrompt, userName, userName))
		} else {
			// Incoming message from someone else in the chat
			pb.WriteString(fmt.Sprintf("[Directive]: You are %s. Continue this conversation as %s replying naturally to the latest message from %s (%q) in %s's authentic texting persona. Output ONLY the response text as %s with no quotes, prefixes, or commentary.", userName, userName, targetSender, targetPrompt, userName, userName))
		}
		promptToSend = pb.String()
	} else {
		// Formal style
		if chatContext != "" {
			instruction = fmt.Sprintf("%s\n\n%s\n\n%s", instruction, chatContext, contextNote)
		}
		if len(extCtx.MediaData) == 0 && extCtx.QuotedText == "" {
			promptToSend = fmt.Sprintf(`[Latest Message from %s to reply to]: "%s"`, targetSender, targetPrompt)
		}
	}

	if len(extCtx.MediaData) > 0 {
		reply, err = gemini.GlobalClient.ChatWithMedia(ctx, chatKey, promptToSend, instruction, extCtx.MimeType, extCtx.MediaData)
	} else {
		reply, err = gemini.GlobalClient.Chat(ctx, chatKey, promptToSend, instruction)
	}

	if err != nil {
		log.Printf("[AI] Request error: %v", err)
		_ = c.React("❌")
		return true, nil
	}

	return true, sendMathOrTextResponse(c, reply, "")
}

func handlePersona(c *CommandContext, key string, cfg config.PersonaConfig) (bool, error) {
	_ = c.React("✨")
	prompt := strings.TrimSpace(c.RawText)
	if strings.HasPrefix(strings.ToLower(prompt), strings.ToLower(cfg.Prefix)) {
		prompt = strings.TrimSpace(prompt[len(cfg.Prefix):])
	}

	extCtx := extractContextAndMedia(c)
	isSolve := isProblemSolvingIntent(c, prompt, extCtx)

	finalPrompt := prompt
	if len(extCtx.MediaData) > 0 {
		if finalPrompt == "" {
			finalPrompt = "Please solve the question shown in this image in detail."
		} else {
			finalPrompt = fmt.Sprintf("%s\n\n(Please examine the attached screenshot/image.)", finalPrompt)
		}
	} else if extCtx.QuotedText != "" {
		if finalPrompt == "" {
			finalPrompt = fmt.Sprintf("[Quoted Message]:\n\"%s\"\n\nPlease respond to the above.", extCtx.QuotedText)
		} else {
			finalPrompt = fmt.Sprintf("[Quoted Message]:\n\"%s\"\n\n[User Question]:\n%s", extCtx.QuotedText, finalPrompt)
		}
	}

	if finalPrompt == "" {
		finalPrompt = "Hello"
	}

	ctx, cancel := context.WithTimeout(c.Ctx, 30*time.Second)
	defer cancel()

	var reply string
	var err error

	if isSolve {
		// Persona solve mode: ignore chat messages context
		solvePrompt := fmt.Sprintf("%s\n\nWhen solving problems, provide a rigorous step-by-step explanation with clear bold titles and LaTeX formatting $$...$$.", cfg.Prompt)
		reply, err = gemini.GlobalClient.SolveProblemForChat(ctx, c.ChatJID.String(), finalPrompt, solvePrompt, extCtx.MimeType, extCtx.MediaData)
	} else {
		chatKey := fmt.Sprintf("%s:%s", c.ChatJID.String(), key)
		excludeID := ""
		if c.RawMsg != nil {
			excludeID = c.RawMsg.Info.ID
		}
		chatContext := buildRecentChatContext(c.ChatJID.String(), excludeID, 50)
		instruction := cfg.Prompt
		if chatContext != "" {
			instruction = fmt.Sprintf("%s\n\n%s", cfg.Prompt, chatContext)
		}

		if len(extCtx.MediaData) > 0 {
			reply, err = gemini.GlobalClient.ChatWithMedia(ctx, chatKey, finalPrompt, instruction, extCtx.MimeType, extCtx.MediaData)
		} else {
			reply, err = gemini.GlobalClient.Chat(ctx, chatKey, finalPrompt, instruction)
		}
	}

	if err != nil {
		log.Printf("[Persona %s] Request error: %v", key, err)
		_ = c.React("❌")
		return true, nil
	}

	label := fmt.Sprintf("*[%s]*", cfg.Name)
	return true, sendMathOrTextResponse(c, reply, label)
}

func sendMathOrTextResponse(c *CommandContext, text, captionPrefix string) error {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}

	if renderer.ContainsMath(trimmed) {
		log.Printf("[MathRenderer] LaTeX/Math formula detected in AI response. Invoking pure Go renderer...")
		_ = c.React("📐")
		pngBytes, err := renderer.RenderMathImage(c.Ctx, trimmed)
		if err != nil {
			log.Printf("[MathRenderer] WARN: Math render failed: %v", err)
		} else if len(pngBytes) > 0 {
			uploaded, upErr := c.Client.Upload(context.Background(), pngBytes, whatsmeow.MediaImage)
			if upErr != nil {
				log.Printf("[MathRenderer] WARN: Failed to upload rendered math image: %v", upErr)
			} else {
				caption := renderer.SanitizeMathNotation(trimmed)
				if captionPrefix != "" {
					caption = fmt.Sprintf("%s\n\n%s", captionPrefix, caption)
				}
				if len(caption) > 1020 {
					caption = caption[:1015] + "..."
				}
				msg := &waProto.Message{
					ImageMessage: &waProto.ImageMessage{
						URL:           proto.String(uploaded.URL),
						DirectPath:    proto.String(uploaded.DirectPath),
						MediaKey:      uploaded.MediaKey,
						Mimetype:      proto.String("image/png"),
						FileEncSHA256: uploaded.FileEncSHA256,
						FileSHA256:    uploaded.FileSHA256,
						FileLength:    proto.Uint64(uint64(len(pngBytes))),
						Caption:       proto.String(caption),
					},
				}
				resp, sendErr := c.Client.SendMessage(c.Ctx, c.ChatJID, msg)
				if sendErr == nil {
					if resp.ID != "" {
						_ = store.RecordBotMessage(resp.ID, c.ChatJID.String(), caption)
					}
					_ = c.React("✅")
					return nil
				}
				log.Printf("[MathRenderer] WARN: Failed to send math image message: %v", sendErr)
			}
		}

		// Fallback: clean Unicode math notation
		cleanText := renderer.SanitizeMathNotation(trimmed)
		if captionPrefix != "" {
			cleanText = fmt.Sprintf("%s\n\n%s", captionPrefix, cleanText)
		}
		_ = c.React("✅")
		return c.ReplyText(cleanText)
	}

	if captionPrefix != "" {
		trimmed = fmt.Sprintf("%s\n\n%s", captionPrefix, trimmed)
	}
	_ = c.React("✅")
	return c.ReplyText(trimmed)
}

func handleOCR(c *CommandContext) (bool, error) {
	var imgMsg *waProto.ImageMessage

	if c.RawMsg.Message.GetImageMessage() != nil {
		imgMsg = c.RawMsg.Message.GetImageMessage()
	} else if quoted := c.RawMsg.Message.GetExtendedTextMessage().GetContextInfo().GetQuotedMessage(); quoted != nil {
		imgMsg = quoted.GetImageMessage()
	}

	if imgMsg == nil {
		return true, c.ReplyText("Send or reply to an image with !ocr to extract text.")
	}

	data, err := c.Client.Download(c.Ctx, imgMsg)
	if err != nil {
		log.Printf("[OCR] Download error: %v", err)
		_ = c.React("❌")
		return true, nil
	}

	ctx, cancel := context.WithTimeout(c.Ctx, 25*time.Second)
	defer cancel()

	mimeType := imgMsg.GetMimetype()
	if mimeType == "" {
		mimeType = "image/jpeg"
	}

	text, err := gemini.GlobalClient.OCR(ctx, mimeType, data)
	if err != nil {
		log.Printf("[OCR] Extraction error: %v", err)
		_ = c.React("❌")
		return true, nil
	}

	return true, c.ReplyText(fmt.Sprintf("*Extracted Text:*\n\n%s", strings.TrimSpace(text)))
}

func handleSummarize(c *CommandContext) (bool, error) {
	var targetText string
	parts := strings.Fields(c.RawText)
	if len(parts) > 1 {
		targetText = strings.Join(parts[1:], " ")
	} else if quoted := c.RawMsg.Message.GetExtendedTextMessage().GetContextInfo().GetQuotedMessage(); quoted != nil {
		targetText = quoted.GetConversation()
		if targetText == "" {
			targetText = quoted.GetExtendedTextMessage().GetText()
		}
	}

	targetText = strings.TrimSpace(targetText)
	if len(targetText) < 15 {
		return true, c.ReplyText("Reply to a message or type !summarize <text> to summarize.")
	}

	ctx, cancel := context.WithTimeout(c.Ctx, 25*time.Second)
	defer cancel()

	summary, err := gemini.GlobalClient.Summarize(ctx, targetText)
	if err != nil {
		log.Printf("[Summarize] Error: %v", err)
		_ = c.React("❌")
		return true, nil
	}

	return true, c.ReplyText(fmt.Sprintf("*Summary:*\n\n%s", strings.TrimSpace(summary)))
}

func handleTranslate(c *CommandContext) (bool, error) {
	parts := strings.Fields(c.RawText)
	targetLang := "English"
	var textToTranslate string

	if len(parts) >= 3 {
		targetLang = parts[1]
		textToTranslate = strings.Join(parts[2:], " ")
	} else if len(parts) == 2 {
		targetLang = parts[1]
		if quoted := c.RawMsg.Message.GetExtendedTextMessage().GetContextInfo().GetQuotedMessage(); quoted != nil {
			textToTranslate = quoted.GetConversation()
			if textToTranslate == "" {
				textToTranslate = quoted.GetExtendedTextMessage().GetText()
			}
		}
	}

	textToTranslate = strings.TrimSpace(textToTranslate)
	if textToTranslate == "" {
		return true, c.ReplyText("Usage: !tr <language> <text> or reply to a message with !tr <language>")
	}

	ctx, cancel := context.WithTimeout(c.Ctx, 25*time.Second)
	defer cancel()

	result, err := gemini.GlobalClient.Translate(ctx, textToTranslate, targetLang)
	if err != nil {
		log.Printf("[Translate] Error: %v", err)
		_ = c.React("❌")
		return true, nil
	}

	return true, c.ReplyText(fmt.Sprintf("*Translation (%s):*\n\n%s", targetLang, strings.TrimSpace(result)))
}

func handleCalc(c *CommandContext) (bool, error) {
	parts := strings.Fields(c.RawText)
	if len(parts) < 2 {
		return true, c.ReplyText("Usage: !calc <expression>\nExample: !calc 15% of 4500")
	}

	expr := strings.Join(parts[1:], " ")
	ctx, cancel := context.WithTimeout(c.Ctx, 20*time.Second)
	defer cancel()

	res, err := gemini.GlobalClient.Calculate(ctx, expr)
	if err != nil {
		log.Printf("[Calc] Error: %v", err)
		_ = c.React("❌")
		return true, nil
	}

	return true, sendMathOrTextResponse(c, res, "*Result:*")
}

func handleDefine(c *CommandContext) (bool, error) {
	parts := strings.Fields(c.RawText)
	if len(parts) < 2 {
		return true, c.ReplyText("Usage: !define <word>")
	}

	word := parts[1]
	ctx, cancel := context.WithTimeout(c.Ctx, 20*time.Second)
	defer cancel()

	res, err := gemini.GlobalClient.Define(ctx, word)
	if err != nil {
		log.Printf("[Define] Error: %v", err)
		_ = c.React("❌")
		return true, nil
	}

	return true, c.ReplyText(strings.TrimSpace(res))
}

func handleWeather(c *CommandContext) (bool, error) {
	parts := strings.Fields(c.RawText)
	if len(parts) < 2 {
		return true, c.ReplyText("Usage: !weather <city/location>\nExample: !weather Tokyo")
	}

	city := strings.Join(parts[1:], " ")
	ctx, cancel := context.WithTimeout(c.Ctx, 25*time.Second)
	defer cancel()

	_ = c.React("🌤️")
	res, err := gemini.GlobalClient.Weather(ctx, city)
	if err != nil {
		log.Printf("[Weather] Error: %v", err)
		_ = c.React("❌")
		return true, nil
	}

	return true, c.ReplyText(strings.TrimSpace(res))
}


func handleFact(c *CommandContext) (bool, error) {
	ctx, cancel := context.WithTimeout(c.Ctx, 20*time.Second)
	defer cancel()

	res, err := gemini.GlobalClient.FactOrQuote(ctx, true)
	if err != nil {
		log.Printf("[Fact] Error: %v", err)
		_ = c.React("❌")
		return true, nil
	}
	return true, c.ReplyText(strings.TrimSpace(res))
}

func handleQuote(c *CommandContext) (bool, error) {
	ctx, cancel := context.WithTimeout(c.Ctx, 20*time.Second)
	defer cancel()

	res, err := gemini.GlobalClient.FactOrQuote(ctx, false)
	if err != nil {
		log.Printf("[Quote] Error: %v", err)
		_ = c.React("❌")
		return true, nil
	}
	return true, c.ReplyText(strings.TrimSpace(res))
}

func handleCatchUp(c *CommandContext) (bool, error) {
	_ = c.React("⏳")

	parts := strings.Fields(c.RawText)
	requestedCount := 0
	if len(parts) > 1 {
		if n, err := strconv.Atoi(parts[1]); err == nil && n > 0 {
			if n < 5 {
				n = 5
			}
			if n > 100 {
				n = 100
			}
			requestedCount = n
		}
	}

	var messages []*store.CachedMessage
	var isAway bool

	excludeID := ""
	if c.RawMsg != nil {
		excludeID = c.RawMsg.Info.ID
	}

	if requestedCount > 0 {
		messages = store.GlobalStore.GetRecentChatHistory(c.ChatJID.String(), requestedCount, excludeID)
		isAway = false
	} else {
		messages, isAway = store.GlobalStore.GetChatHistorySinceUserLastMessage(c.ChatJID.String(), c.SenderJID.String(), excludeID, 50, 100)
	}

	if len(messages) < 2 {
		_ = c.React("ℹ️")
		return true, c.ReplyText("ℹ️ Not enough recent messages in chat cache to summarize yet (need at least 2 messages).\n\n_Messages are cached in real-time as conversations occur._")
	}

	var sb strings.Builder
	for _, m := range messages {
		sender := m.PushName
		if sender == "" {
			sender = store.GlobalStore.GetContact(m.SenderJID.String())
		}
		if sender == "" {
			sender = strings.Split(m.SenderJID.User, "@")[0]
		}

		timeStr := m.Timestamp.Format("15:04")
		text := strings.TrimSpace(m.Text)
		if text != "" && m.MediaType != "" {
			sb.WriteString(fmt.Sprintf("[%s] %s: [%s] %s\n", timeStr, sender, m.MediaType, text))
		} else if text != "" {
			sb.WriteString(fmt.Sprintf("[%s] %s: %s\n", timeStr, sender, text))
		} else if m.MediaType != "" {
			sb.WriteString(fmt.Sprintf("[%s] %s: [%s]\n", timeStr, sender, m.MediaType))
		}
	}

	if gemini.GlobalClient == nil {
		_ = c.React("❌")
		return true, nil
	}

	ctx, cancel := context.WithTimeout(c.Ctx, 35*time.Second)
	defer cancel()

	summary, err := gemini.GlobalClient.CatchUpSummary(ctx, sb.String(), len(messages), isAway)
	if err != nil {
		log.Printf("[CatchUp] Error: %v", err)
		_ = c.React("❌")
		return true, nil
	}

	header := fmt.Sprintf("⏳ *Catch-Up Summary* (Last %d messages)", len(messages))
	if isAway {
		header = fmt.Sprintf("⏳ *Catch-Up Summary* (%d messages while you were away)", len(messages))
	}

	output := fmt.Sprintf("%s\n────────────────────\n%s\n────────────────────\n_Tip: Use !catchup [50-100] to adjust summary depth._", header, strings.TrimSpace(summary))

	_ = c.React("✅")
	return true, c.ReplyText(output)
}

func handleRoast(c *CommandContext) (bool, error) {
	_ = c.React("🔥")
	if gemini.GlobalClient == nil {
		_ = c.React("❌")
		return true, nil
	}

	target := strings.TrimSpace(c.RawText)
	prefixes := []string{"!roast", "!cook", "!burn"}
	for _, p := range prefixes {
		if strings.HasPrefix(strings.ToLower(target), p) {
			target = strings.TrimSpace(target[len(p):])
			break
		}
	}

	extCtx := extractContextAndMedia(c)
	quoted := extCtx.QuotedText

	var prompt string
	if quoted != "" && target != "" {
		prompt = fmt.Sprintf("Cook this person brutally. Target: '%s'. Their message: \"%s\". Roast them with maximum Gen-Z reel brainrot disrespect, savage comebacks, and zero mercy.", target, quoted)
	} else if quoted != "" {
		prompt = fmt.Sprintf("Cook whoever wrote this message with zero mercy. Their message: \"%s\". Give a devastating, hilarious Gen-Z roast using modern internet brainrot slang.", quoted)
	} else if target != "" {
		prompt = fmt.Sprintf("Roast '%s' into oblivion. Make it a savage, creative, devastating Gen-Z roast packed with reel brainrot slang (aura, cooked, rizz, skill issue, touch grass, etc.).", target)
	} else {
		senderName := ""
		if c.RawMsg != nil {
			senderName = c.RawMsg.Info.PushName
		}
		if senderName == "" && store.GlobalStore != nil {
			senderName = store.GlobalStore.GetContact(c.SenderJID.String())
		}
		if senderName == "" {
			senderName = c.SenderJID.User
		}
		if senderName == "" {
			senderName = "this user"
		}
		prompt = fmt.Sprintf("The user '%s' asked to be roasted. Cook them ruthlessly. Deliver a savage, hilarious Gen-Z roast with peak reel brainrot energy. Tear into their aura and life choices with comedic violence.", senderName)
	}

	systemInstruction := `You are an elite, unhinged roast master with chronic Gen-Z reel brainrot.
Your mission is to cook the target with supreme comedic disrespect.
Keep it between 2-4 punchy sentences so it hits hard on a mobile screen. Don't be polite or soften the blow. Absolutely roast them.`

	req := gemini.GenerateRequest{
		Contents: []gemini.Content{
			{
				Role:  "user",
				Parts: []gemini.Part{{Text: prompt}},
			},
		},
		SystemInstruction: &gemini.SystemInstruction{
			Parts: []gemini.Part{{Text: systemInstruction}},
		},
		GenerationConfig: &gemini.GenerationConfig{
			Temperature: 0.9,
		},
	}

	ctx, cancel := context.WithTimeout(c.Ctx, 25*time.Second)
	defer cancel()

	reply, err := gemini.GlobalClient.GenerateForChat(ctx, c.ChatJID.String(), req)
	if err != nil {
		log.Printf("[Roast] Error: %v", err)
		_ = c.React("❌")
		return true, nil
	}

	return true, c.ReplyText(fmt.Sprintf("%s", strings.TrimSpace(reply)))
}

func handleBrainrot(c *CommandContext) (bool, error) {
	_ = c.React("🧠")
	if gemini.GlobalClient == nil {
		_ = c.React("❌")
		return true, nil
	}

	topic := strings.TrimSpace(c.RawText)
	prefixes := []string{"!brainrot", "!yap", "!skibidi"}
	for _, p := range prefixes {
		if strings.HasPrefix(strings.ToLower(topic), p) {
			topic = strings.TrimSpace(topic[len(p):])
			break
		}
	}

	if topic == "" {
		topic = "daily life, subway surfers gameplay, and sigma grindset"
	}

	prompt := fmt.Sprintf("Generate an unhinged, peak Gen-Z reel brainrot monologue about: %s. Pack it with subway surfers commentary, kai cenat, skibidi, mewing streaks, edging, rizzler, fanum tax, looksmaxxing, baby gronk, and chat is this real energy.", topic)

	systemInstruction := `You are an unhinged Gen-Z TikTok/Instagram Reels brainrot creator.
Speak entirely in maximum brainrot slang, stream-of-consciousness reel narration, ADHD energy, and slang overdose.
Use emojis generously (💀, 🗣️, 💯, 🤫🧏, 🗿, 😭). Keep it funny, chaotic, and under 150 words.`

	req := gemini.GenerateRequest{
		Contents: []gemini.Content{
			{
				Role:  "user",
				Parts: []gemini.Part{{Text: prompt}},
			},
		},
		SystemInstruction: &gemini.SystemInstruction{
			Parts: []gemini.Part{{Text: systemInstruction}},
		},
		GenerationConfig: &gemini.GenerationConfig{
			Temperature: 1.0,
		},
	}

	ctx, cancel := context.WithTimeout(c.Ctx, 25*time.Second)
	defer cancel()

	reply, err := gemini.GlobalClient.GenerateForChat(ctx, c.ChatJID.String(), req)
	if err != nil {
		log.Printf("[Brainrot] Error: %v", err)
		_ = c.React("❌")
		return true, nil
	}

	return true, c.ReplyText(fmt.Sprintf("🗣️ *[PEAK BRAINROT]* 💀\n\n%s", strings.TrimSpace(reply)))
}

func handleAIMode(c *CommandContext) (bool, error) {
	if gemini.GlobalClient == nil {
		_ = c.React("❌")
		return true, c.ReplyText("AI service is not initialized.")
	}

	chatKey := c.ChatJID.String()
	parts := strings.Fields(c.RawText)
	if len(parts) <= 1 {
		// Show current status for this chat and global defaults
		status := gemini.GlobalClient.GetProviderStatus()
		chatProvider, hasOverride := gemini.GlobalClient.GetChatProviderDirect(chatKey)
		effectiveModel := gemini.GlobalClient.GetChatModel(chatKey)
		globalProvider := gemini.GlobalClient.GetProvider()

		var sb strings.Builder
		sb.WriteString("🤖 *AI Provider Engine Status*\n\n")

		if hasOverride {
			sb.WriteString(fmt.Sprintf("• *This Chat Provider:* `%s` *(Custom Override)*\n", strings.ToUpper(chatProvider)))
		} else {
			sb.WriteString(fmt.Sprintf("• *This Chat Provider:* `%s` *(Inheriting Global)*\n", strings.ToUpper(chatProvider)))
		}
		sb.WriteString(fmt.Sprintf("• *Active Model:* `%s`\n", effectiveModel))
		sb.WriteString(fmt.Sprintf("• *Global Default:* `%s`\n\n", strings.ToUpper(globalProvider)))

		sb.WriteString("*Available Provider Modes:*\n")
		providers := []struct {
			name string
			desc string
		}{
			{"gemini", "Google Gemini Flash/Pro (Moderated standard)"},
			{"openrouter", "OpenRouter (Uncensored Dolphin, Venice, Hermes, etc.)"},
			{"groq", "GroqCloud (Ultra-low latency Llama-3.3)"},
		}

		for _, p := range providers {
			var state string
			if p.name == "ollama" {
				state = "🟢 Ready (Local REST API)"
			} else if status[p.name] {
				state = "🟢 Ready"
			} else {
				state = "🔴 Key missing in .env"
			}
			activeMark := ""
			if p.name == chatProvider {
				activeMark = " 👈 *(THIS CHAT)*"
			}
			sb.WriteString(fmt.Sprintf("• *%s*: %s%s\n  _%s_\n", strings.ToUpper(p.name), state, activeMark, p.desc))
		}

		sb.WriteString("\n*Commands:*\n")
		sb.WriteString("• Set this chat: `!aimode openrouter` | `!aimode groq` | `!aimode gemini` | `!aimode ollama`\n")
		sb.WriteString("• Reset this chat to default: `!aimode reset`\n")
		sb.WriteString("• Change model: `!aimodel <model_name>`\n")
		if c.IsSenderOwner() {
			sb.WriteString("• Change global default: `!aimode global <provider>`\n")
		}

		_ = c.React("🤖")
		return true, c.ReplyText(sb.String())
	}

	action := strings.ToLower(strings.TrimSpace(parts[1]))

	// 1. Reset chat override
	if action == "reset" || action == "default" {
		if c.IsGroup && !c.IsSenderAdmin() && !c.IsSenderOwner() {
			_ = c.React("⛔")
			return true, c.ReplyText("Only group admins or bot owners can reset the AI provider for this group.")
		}
		gemini.GlobalClient.ClearChatOverride(chatKey)
		_ = c.React("✅")
		return true, c.ReplyText(fmt.Sprintf("🔄 *AI Provider Reset to Global Default!*\n\n• *Provider:* `%s`\n• *Model:* `%s`",
			strings.ToUpper(gemini.GlobalClient.GetChatProvider(chatKey)), gemini.GlobalClient.GetChatModel(chatKey)))
	}

	// 2. Global provider change
	if action == "global" {
		if !c.IsSenderOwner() {
			_ = c.React("⛔")
			return true, c.ReplyText("Only bot owners can change the global default AI provider.")
		}
		if len(parts) < 3 {
			return true, c.ReplyText("Usage: `!aimode global <gemini|openrouter|groq|ollama>`")
		}
		target := strings.ToLower(strings.TrimSpace(parts[2]))
		if err := gemini.GlobalClient.SetProvider(target); err != nil {
			_ = c.React("❌")
			return true, c.ReplyText(fmt.Sprintf("⚠️ Could not set global provider: %v", err))
		}
		_ = c.React("✅")
		return true, c.ReplyText(fmt.Sprintf("🌐 *Global Default AI Provider Updated!*\n\n• *Provider:* `%s`\n• *Model:* `%s`",
			strings.ToUpper(gemini.GlobalClient.GetProvider()), gemini.GlobalClient.GetActiveModel()))
	}

	// 3. Set provider for THIS chat
	if c.IsGroup && !c.IsSenderAdmin() && !c.IsSenderOwner() {
		_ = c.React("⛔")
		return true, c.ReplyText("Only group admins or bot owners can change the AI provider for this chat.")
	}

	if err := gemini.GlobalClient.SetChatProvider(chatKey, action); err != nil {
		_ = c.React("❌")
		return true, c.ReplyText(fmt.Sprintf("⚠️ Could not switch provider for this chat: %v", err))
	}

	_ = c.React("✅")
	return true, c.ReplyText(fmt.Sprintf("✅ *AI Provider Set for this Chat!*\n\n• *Chat Provider:* `%s`\n• *Active Model:* `%s`\n\n_(To revert to global default anytime, type `!aimode reset`)_",
		strings.ToUpper(gemini.GlobalClient.GetChatProvider(chatKey)), gemini.GlobalClient.GetChatModel(chatKey)))
}

func handleAIModel(c *CommandContext) (bool, error) {
	if gemini.GlobalClient == nil {
		_ = c.React("❌")
		return true, c.ReplyText("AI service is not initialized.")
	}

	chatKey := c.ChatJID.String()
	parts := strings.Fields(c.RawText)
	currentProvider := gemini.GlobalClient.GetChatProvider(chatKey)
	currentModel := gemini.GlobalClient.GetChatModel(chatKey)

	if len(parts) <= 1 {
		var sb strings.Builder
		sb.WriteString("🧠 *AI Model Configuration (This Chat)*\n\n")
		sb.WriteString(fmt.Sprintf("• *Provider:* `%s`\n", strings.ToUpper(currentProvider)))
		sb.WriteString(fmt.Sprintf("• *Active Model:* `%s`\n\n", currentModel))

		sb.WriteString("*Popular & Uncensored Recommendations for " + strings.ToUpper(currentProvider) + ":*\n")
		switch currentProvider {
		case "openrouter":
			sb.WriteString("• `cognitivecomputations/dolphin-mistral-24b-venice-edition` (Uncensored de-aligned)\n")
			sb.WriteString("• `nousresearch/hermes-3-llama-3.1-405b` (Uncensored creative/agentic)\n")
			sb.WriteString("• `meta-llama/llama-3.3-70b-instruct:free` (Free tier)\n")
			sb.WriteString("• `mistralai/mistral-nemo` (Lenient creative)\n")
		case "groq":
			sb.WriteString("• `llama-3.3-70b-versatile` (Flagship 70B reasoning - 280 t/s)\n")
			sb.WriteString("• `llama-3.1-8b-instant` (Instant response - 560 t/s)\n")
			sb.WriteString("• `openai/gpt-oss-120b` (OpenAI 120B open-weight - 500 t/s)\n")
			sb.WriteString("• `openai/gpt-oss-20b` (OpenAI 20B ultra-high speed - 1000 t/s)\n")
			sb.WriteString("• `qwen/qwen3.8-27b` (Qwen 27B high performance - 450 t/s)\n")
			sb.WriteString("• `minimaxai/minimax-m2.7` (MiniMax 196k context)\n")
		
		default: // gemini
			sb.WriteString("• `gemini-3.5-flash-lite` (Fast, multimodal, default)\n")
			sb.WriteString("• `gemini-2.5-pro` (Complex reasoning)\n")
		}

		sb.WriteString("\n*To switch model for this chat:*\n`!aimodel <model_name>`\n`!aimodel reset`")
		_ = c.React("🧠")
		return true, c.ReplyText(sb.String())
	}

	if c.IsGroup && !c.IsSenderAdmin() && !c.IsSenderOwner() {
		_ = c.React("⛔")
		return true, c.ReplyText("Only group admins or bot owners can change the AI model.")
	}

	arg := strings.TrimSpace(parts[1])
	if arg == "reset" || arg == "default" {
		gemini.GlobalClient.SetChatModel(chatKey, "")
		_ = c.React("✅")
		return true, c.ReplyText(fmt.Sprintf("🔄 *Model Reset to Provider Default!*\n• *Provider:* `%s`\n• *Model:* `%s`",
			strings.ToUpper(currentProvider), gemini.GlobalClient.GetChatModel(chatKey)))
	}

	gemini.GlobalClient.SetChatModel(chatKey, arg)
	_ = c.React("✅")
	return true, c.ReplyText(fmt.Sprintf("✅ *Model Updated for this Chat!*\n• *Provider:* `%s`\n• *Model:* `%s`",
		strings.ToUpper(currentProvider), arg))
}

func handleAIStyle(c *CommandContext) (bool, error) {
	if gemini.GlobalClient == nil {
		return true, c.ReplyText("AI service is not initialized.")
	}

	chatKey := c.ChatJID.ToNonAD().String()
	parts := strings.Fields(c.RawText)
	currentStyle := gemini.GlobalClient.GetChatStyle(chatKey)
	globalStyle := gemini.GlobalClient.GetGlobalStyle()
	overrideStyle, hasOverride := gemini.GlobalClient.GetChatStyleDirect(chatKey)

	if len(parts) <= 1 {
		var sb strings.Builder
		sb.WriteString("🎭 *PrismBot Persona Style (This Chat)*\n\n")

		styleDisplay := "👔 FORMAL"
		if currentStyle == "genz" {
			styleDisplay = "🧢 GEN-Z (Savage / Brainrot)"
		} else if currentStyle == "self" {
			styleDisplay = "👤 SELF USER (Mimic Chat Style)"
		}
		sb.WriteString(fmt.Sprintf("• *Active Style:* `%s`\n", styleDisplay))
		if hasOverride {
			sb.WriteString(fmt.Sprintf("• *Chat Override:* `Custom (%s)`\n", strings.ToUpper(overrideStyle)))
		} else {
			sb.WriteString(fmt.Sprintf("• *Inheriting Default:* `%s`\n", strings.ToUpper(globalStyle)))
		}

		sb.WriteString("\n*Available Persona Styles:*\n")
		sb.WriteString("1️⃣ 👔 `!style formal` (Default)\n")
		sb.WriteString("   _Professional, polite, articulate, objective, step-by-step clarity._\n\n")
		sb.WriteString("2️⃣ 🧢 `!style genz`\n")
		sb.WriteString("   _Savage roasts, peak reel brainrot humor, witty comebacks, unhinged energy._\n\n")
		sb.WriteString("3️⃣ 👤 `!style self` (or `!style user`)\n")
		sb.WriteString("   _Mimics your persona and texting style based on your message history in this specific chat._\n\n")
		sb.WriteString("🔄 `!style reset`\n")
		sb.WriteString("   _Revert this chat to the global default style._")

		_ = c.React("🎭")
		return true, c.ReplyText(sb.String())
	}

	if c.IsGroup && !c.IsSenderAdmin() && !c.IsSenderOwner() {
		_ = c.React("⛔")
		return true, c.ReplyText("Only group admins or bot owners can change the persona style.")
	}

	target := strings.ToLower(strings.TrimSpace(strings.Join(parts[1:], " ")))
	switch target {
	case "formal", "professional", "normal":
		_ = gemini.GlobalClient.SetChatStyle(chatKey, "formal")
		_ = c.React("👔")
		return true, c.ReplyText("👔 *Persona Style Updated to FORMAL!*\n@Prism will now respond with an articulate, polite, and professional tone in this chat.")

	case "genz", "brainrot", "savage", "roast":
		_ = gemini.GlobalClient.SetChatStyle(chatKey, "genz")
		_ = c.React("🧢")
		return true, c.ReplyText("🧢 *Persona Style Updated to GEN-Z!*\n@Prism is now set to savage reel brainrot mode in this chat. Get ready for unhinged comebacks and zero mercy! 🔥")

	case "self", "user", "self user", "self_user", "me":
		_ = gemini.GlobalClient.SetChatStyle(chatKey, "self")
		_ = c.React("👤")
		return true, c.ReplyText("👤 *Persona Style Updated to SELF USER!*\n@Prism will now mimic your personal texting style based on your message history in this specific chat! 🎭")

	case "reset", "default", "clear":
		_ = gemini.GlobalClient.SetChatStyle(chatKey, "")
		_ = c.React("🔄")
		return true, c.ReplyText(fmt.Sprintf("🔄 *Persona Style Reset!*\nThis chat will now follow the global default style (*%s*).",
			strings.ToUpper(gemini.GlobalClient.GetGlobalStyle())))

	default:
		_ = c.React("⚠️")
		return true, c.ReplyText("Invalid style. Choose either `!style formal`, `!style genz`, or `!style self` (or `!style reset`).")
	}
}



