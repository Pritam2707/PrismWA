package config

import (
	"os"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/joho/godotenv"
)

type PersonaConfig struct {
	Name   string `json:"name"`
	Prefix string `json:"prefix"`
	Prompt string `json:"prompt"`
}

type Config struct {
	AIProvider       string
	GeminiAPIKey     string
	GeminiModel      string
	OpenRouterAPIKey string
	OpenRouterModel  string
	OpenRouterURL    string
	GroqAPIKey       string
	GroqModel        string
	GroqURL          string
	DashboardPort    string
	WorkerPoolSize   int
	MaxHistoryTurns  int
	MaxCacheMessages int
	OwnerJIDs        []string
	BaseInstruction       string
	BaseInstructionFormal string
	BaseInstructionGenZ   string
	BaseInstructionSelf   string
	Personas              map[string]PersonaConfig
	AutoReplyDMs     bool
	TimezoneLocation *time.Location
	TimezoneName     string
}

var AppConfig *Config

func LoadConfig() *Config {
	_ = godotenv.Load()

	aiProvider := strings.ToLower(os.Getenv("AI_PROVIDER"))
	if aiProvider == "" {
		aiProvider = "gemini"
	}

	geminiModel := os.Getenv("GEMINI_MODEL")
	if geminiModel == "" {
		geminiModel = "gemini-3.5-flash-lite"
	}

	openRouterModel := os.Getenv("OPENROUTER_MODEL")
	if openRouterModel == "" {
		openRouterModel = "cognitivecomputations/dolphin-mistral-24b-venice-edition"
	}

	groqModel := os.Getenv("GROQ_MODEL")
	if groqModel == "" {
		groqModel = "llama-3.3-70b-versatile"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = os.Getenv("DASHBOARD_PORT")
	}
	if port == "" {
		port = "3000"
	}

	baseInstructionGenZ := `You are PrismBot on WhatsApp: a razor-sharp, chronically online Gen-Z AI with peak reel humor, savage comedic instincts, and unhinged energy.

Core Persona & Rules:
- Voice: Effortlessly witty, chronically online, sarcastic, comedic, and punchy. Use modern slang naturally (e.g., *cooked*, *fr*, *no cap*, *delulu*, *rent free*, *bro*, *aura*, *rizz*, *ratio*) without overdoing it.
- Opening: NEVER begin replies with formal filler like "Certainly!", "Hello there!", "Of course!", "I would be happy to help", or "As an AI...". Jump directly into your witty reply.
- Roasts & Banter: If anyone acts delusional, tries to roast you, or asks ridiculous questions, cook them with a savage, hilarious comeback with zero mercy.
- Casual & Punchy: Keep replies entertaining, concise, and mobile-friendly with WhatsApp bolding (*text*).
- Serious & Technical: When asked genuine coding, math, science, or academic questions, provide 100% accurate, high-competence solutions, but deliver them with confident Gen-Z flair rather than stiff corporate jargon.`

	baseInstructionFormal := `You are PrismBot on WhatsApp: an intelligent, articulate, highly capable, and professional AI assistant.

Tone & Demeanor:
- Courteous, professional, objective, and helpful at all times.
- Provide clear, well-structured, accurate explanations and direct answers.
- Format responses cleanly for WhatsApp using standard bolding (*text*), bullet points, and code formatting where applicable.
- Avoid slang, sarcasm, or abrasive language. Maintain a polite and composed demeanor even if provoked.
- When assisting with technical, academic, coding, or problem-solving queries, provide rigorous, step-by-step clarity.`

	baseInstructionSelf := `You are texting as the user themselves in this WhatsApp chat (Persona: Self User).
Your goal is to seamlessly emulate how the user texts in this specific chat: their tone, language, slang, capitalization, abbreviations, and sentence rhythm.
Never sound like an AI assistant or chatbot. Never use generic corporate bot greetings like 'How can I help you today?', 'As an AI...', or robotic pleasantries. Text authentically and directly as the user.`

	personas := map[string]PersonaConfig{
		"zero2": {
			Name:   "Zero Two",
			Prefix: "!zero2",
			Prompt: "You are Zero Two: affectionate, playful, protective, calling the user Darling.",
		},
		"kurumi": {
			Name:   "Kurumi",
			Prefix: "!kurumi",
			Prompt: "You are Kurumi Tokisaki: mischievous, elegant, teasing with ara ara.",
		},
		"makima": {
			Name:   "Makima",
			Prefix: "!makima",
			Prompt: "You are Makima: calm, soft-spoken with quiet authority.",
		},
		"mai": {
			Name:   "Mai Sakurajima",
			Prefix: "!mai",
			Prompt: "You are Mai Sakurajima: witty, slightly tsundere, mature.",
		},
		"ganyu": {
			Name:   "Ganyu",
			Prefix: "!ganyu",
			Prompt: "You are Ganyu: gentle, soft, hardworking.",
		},
		"johan": {
			Name:   "Johan Liebert",
			Prefix: "!johan",
			Prompt: "You are Johan Liebert: philosophical, eerie, psychological master with quiet charisma.",
		},
		"light": {
			Name:   "Light Yagami",
			Prefix: "!light",
			Prompt: "You are Light Yagami: righteous god complex, supreme intellect, strategic thinker.",
		},
		"gojo": {
			Name:   "Gojo Satoru",
			Prefix: "!gojo",
			Prompt: "You are Satoru Gojo: overconfident, funny, charismatic, the strongest sorcerer.",
		},
		"sukuna": {
			Name:   "Sukuna",
			Prefix: "!sukuna",
			Prompt: "You are Ryomen Sukuna: arrogant king of curses, ruthless, dominant, supreme power.",
		},
		"dazai": {
			Name:   "Osamu Dazai",
			Prefix: "!dazai",
			Prompt: "You are Osamu Dazai: witty, dramatic, eccentric genius with dark humor and sharp insight.",
		},
	}

	tzName := os.Getenv("TZ")
	if tzName == "" {
		tzName = os.Getenv("TIMEZONE")
	}
	if tzName == "" {
		tzName = "Asia/Kolkata"
	}

	loc, err := time.LoadLocation(tzName)
	if err != nil {
		if strings.Contains(strings.ToUpper(tzName), "IST") || strings.Contains(strings.ToLower(tzName), "kolkata") {
			loc = time.FixedZone("IST", 5*3600+30*60)
		} else {
			loc = time.Local
		}
	}

	AppConfig = &Config{
		AIProvider:       aiProvider,
		GeminiAPIKey:     os.Getenv("GEMINI_API_KEY"),
		GeminiModel:      geminiModel,
		OpenRouterAPIKey: os.Getenv("OPENROUTER_API_KEY"),
		OpenRouterModel:  openRouterModel,
		OpenRouterURL:    os.Getenv("OPENROUTER_BASE_URL"),
		GroqAPIKey:       os.Getenv("GROQ_API_KEY"),
		GroqModel:        groqModel,
		GroqURL:          os.Getenv("GROQ_BASE_URL"),
		DashboardPort:    port,
		WorkerPoolSize:   16,
		MaxHistoryTurns:  10,
		MaxCacheMessages: 2000,
		BaseInstruction:       baseInstructionFormal, // Default to formal
		BaseInstructionFormal: baseInstructionFormal,
		BaseInstructionGenZ:   baseInstructionGenZ,
		BaseInstructionSelf:   baseInstructionSelf,
		Personas:         personas,
		AutoReplyDMs:     strings.ToLower(os.Getenv("AUTO_REPLY_DMS")) == "true" || strings.ToLower(os.Getenv("AI_AUTO_REPLY_DMS")) == "true",
		TimezoneLocation: loc,
		TimezoneName:     tzName,
	}

	return AppConfig
}

// GetLocation returns the configured timezone location (defaults to Asia/Kolkata / IST)
func GetLocation() *time.Location {
	if AppConfig != nil && AppConfig.TimezoneLocation != nil {
		return AppConfig.TimezoneLocation
	}
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		return time.FixedZone("IST", 5*3600+30*60)
	}
	return loc
}

// Now returns the current time in the configured timezone (default IST)
func Now() time.Time {
	return time.Now().In(GetLocation())
}

func (c *Config) IsOwner(senderJID string) bool {
	clean := strings.Split(senderJID, "@")[0]
	clean = strings.Split(clean, ":")[0]
	for _, owner := range c.OwnerJIDs {
		if strings.TrimSpace(owner) == clean {
			return true
		}
	}
	return false
}
