package gemini

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"whatsapp-bot/pkg/config"
)

type ChatAISetting struct {
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
	Style    string `json:"style,omitempty"` // "formal", "genz", or "self"
}

type Part struct {
	Text       string      `json:"text,omitempty"`
	InlineData *InlineData `json:"inlineData,omitempty"`
}

type InlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type Content struct {
	Role  string `json:"role,omitempty"`
	Parts []Part `json:"parts"`
}

type GenerateRequest struct {
	Contents          []Content          `json:"contents"`
	SystemInstruction *SystemInstruction `json:"systemInstruction,omitempty"`
	GenerationConfig  *GenerationConfig  `json:"generationConfig,omitempty"`
}

type SystemInstruction struct {
	Parts []Part `json:"parts"`
}

type GenerationConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

type Candidate struct {
	Content Content `json:"content"`
}

type GenerateResponse struct {
	Candidates []Candidate `json:"candidates"`
	Error      *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error,omitempty"`
}

type OpenAIMessageContent struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *OpenAIImageURL `json:"image_url,omitempty"`
}

type OpenAIImageURL struct {
	URL string `json:"url"`
}

type OpenAIMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type OpenAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type OpenAIChatResponse struct {
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string      `json:"message"`
		Code    interface{} `json:"code"`
	} `json:"error,omitempty"`
}

type GeminiClient struct {
	provider        string // "gemini", "openrouter", "groq", "ollama"
	apiKey          string
	model           string
	baseURL         string
	openRouterKey   string
	openRouterModel string
	openRouterURL   string
	groqKey         string
	groqModel       string
	groqURL         string
	httpClient      *http.Client
	mu              sync.RWMutex
	histories       map[string][]Content
	maxTurns        int
	bufPool         sync.Pool

	chatOverrides   map[string]ChatAISetting
	overridesFile   string
	globalStyle     string // "formal" (default), "genz", or "self"
}

var GlobalClient *GeminiClient

func NewClient(apiKey, model string) *GeminiClient {
	if model == "" {
		model = "gemini-3.5-flash-lite"
	}
	tr := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression: false,
	}
	c := &GeminiClient{
		provider:    "gemini",
		apiKey:      apiKey,
		model:       model,
		globalStyle: "formal",
		httpClient: &http.Client{
			Transport: tr,
			Timeout:   45 * time.Second,
		},
		histories:     make(map[string][]Content),
		maxTurns:      10,
		chatOverrides: make(map[string]ChatAISetting),
		overridesFile: filepath.Join("data", "chat_ai_settings.json"),
		bufPool: sync.Pool{
			New: func() any {
				return new(bytes.Buffer)
			},
		},
	}
	c.loadChatOverrides()
	GlobalClient = c
	return c
}

func (g *GeminiClient) ConfigureOpenRouter(apiKey, model, customURL string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.openRouterKey = apiKey
	if model != "" {
		g.openRouterModel = model
	} else {
		g.openRouterModel = "cognitivecomputations/dolphin-mistral-24b-venice-edition"
	}
	if customURL != "" {
		g.openRouterURL = customURL
	} else {
		g.openRouterURL = "https://openrouter.ai/api/v1/chat/completions"
	}
}

func (g *GeminiClient) ConfigureGroq(apiKey, model, customURL string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.groqKey = apiKey
	if model != "" {
		g.groqModel = model
	} else {
		g.groqModel = "llama-3.3-70b-versatile"
	}
	if customURL != "" {
		g.groqURL = customURL
	} else {
		g.groqURL = "https://api.groq.com/openai/v1/chat/completions"
	}
}

func (g *GeminiClient) SetProvider(p string) error {
	p = strings.ToLower(strings.TrimSpace(p))
	if p == "groqcloud" {
		p = "groq"
	}
	if p != "gemini" && p != "openrouter" && p != "groq" && p != "ollama" {
		return fmt.Errorf("unknown provider %q (valid: gemini, openrouter, groq, ollama)", p)
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	switch p {
	case "openrouter":
		if g.openRouterKey == "" {
			return fmt.Errorf("OPENROUTER_API_KEY is not configured in .env")
		}
	case "groq":
		if g.groqKey == "" {
			return fmt.Errorf("GROQ_API_KEY is not configured in .env")
		}
	case "gemini":
		if g.apiKey == "" {
			return fmt.Errorf("GEMINI_API_KEY is not configured in .env")
		}
	}

	g.provider = p
	return nil
}

func (g *GeminiClient) GetProvider() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.provider == "" {
		return "gemini"
	}
	return g.provider
}

func (g *GeminiClient) GetGroqKey() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.groqKey
}

func (g *GeminiClient) GetActiveModel() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	switch g.provider {
	case "openrouter":
		return g.openRouterModel
	case "groq":
		return g.groqModel
	default:
		return g.model
	}
}

func (g *GeminiClient) SetActiveModel(m string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	switch g.provider {
	case "openrouter":
		g.openRouterModel = m
	case "groq":
		g.groqModel = m
	default:
		g.model = m
	}
}

func (g *GeminiClient) GetProviderStatus() map[string]bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return map[string]bool{
		"gemini":     g.apiKey != "",
		"openrouter": g.openRouterKey != "",
		"groq":       g.groqKey != "",
		"ollama":     true,
	}
}

func cleanChatKey(chatKey string) string {
	chatKey = strings.TrimSpace(chatKey)
	if at := strings.Index(chatKey, "@"); at != -1 {
		userPart := chatKey[:at]
		domainPart := chatKey[at:]
		if colon := strings.Index(userPart, ":"); colon != -1 {
			userPart = userPart[:colon]
		}
		if colon := strings.Index(domainPart, ":"); colon != -1 {
			domainPart = domainPart[:colon]
		}
		return userPart + domainPart
	}
	if at := strings.Index(chatKey, ":"); at != -1 {
		chatKey = chatKey[:at]
	}
	return chatKey
}

func (g *GeminiClient) loadChatOverrides() {
	if g.overridesFile == "" {
		return
	}
	data, err := os.ReadFile(g.overridesFile)
	if err != nil {
		return
	}
	var loaded map[string]ChatAISetting
	if err := json.Unmarshal(data, &loaded); err == nil && loaded != nil {
		g.mu.Lock()
		g.chatOverrides = loaded
		g.mu.Unlock()
	}
}

func (g *GeminiClient) saveChatOverridesLocked() {
	if g.overridesFile == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(g.overridesFile), 0755)
	data, err := json.MarshalIndent(g.chatOverrides, "", "  ")
	if err == nil {
		_ = os.WriteFile(g.overridesFile, data, 0644)
	}
}

func (g *GeminiClient) SetChatProvider(chatKey, provider string) error {
	chatKey = cleanChatKey(chatKey)
	if chatKey == "" {
		return errors.New("chat key cannot be empty")
	}
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "groqcloud" {
		provider = "groq"
	}
	if provider != "gemini" && provider != "openrouter" && provider != "groq" && provider != "ollama" {
		return fmt.Errorf("unknown provider %q (valid: gemini, openrouter, groq, ollama)", provider)
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	switch provider {
	case "openrouter":
		if g.openRouterKey == "" {
			return errors.New("OPENROUTER_API_KEY is not configured in .env")
		}
	case "groq":
		if g.groqKey == "" {
			return errors.New("GROQ_API_KEY is not configured in .env")
		}
	case "gemini":
		if g.apiKey == "" {
			return errors.New("GEMINI_API_KEY is not configured in .env")
		}
	}

	setting := g.chatOverrides[chatKey]
	setting.Provider = provider
	g.chatOverrides[chatKey] = setting
	g.saveChatOverridesLocked()
	return nil
}

func (g *GeminiClient) SetChatModel(chatKey, model string) {
	chatKey = cleanChatKey(chatKey)
	if chatKey == "" {
		return
	}
	model = strings.TrimSpace(model)

	g.mu.Lock()
	defer g.mu.Unlock()

	setting := g.chatOverrides[chatKey]
	setting.Model = model
	g.chatOverrides[chatKey] = setting
	g.saveChatOverridesLocked()
}

func (g *GeminiClient) ClearChatOverride(chatKey string) {
	chatKey = cleanChatKey(chatKey)
	if chatKey == "" {
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	delete(g.chatOverrides, chatKey)
	g.saveChatOverridesLocked()
}

func (g *GeminiClient) GetChatProvider(chatKey string) string {
	chatKey = cleanChatKey(chatKey)
	g.mu.RLock()
	defer g.mu.RUnlock()

	if setting, ok := g.chatOverrides[chatKey]; ok && setting.Provider != "" {
		return setting.Provider
	}
	if g.provider == "" {
		return "gemini"
	}
	return g.provider
}

func (g *GeminiClient) GetChatProviderDirect(chatKey string) (provider string, hasOverride bool) {
	chatKey = cleanChatKey(chatKey)
	g.mu.RLock()
	defer g.mu.RUnlock()

	if setting, ok := g.chatOverrides[chatKey]; ok && setting.Provider != "" {
		return setting.Provider, true
	}
	if g.provider == "" {
		return "gemini", false
	}
	return g.provider, false
}

func (g *GeminiClient) GetChatModel(chatKey string) string {
	chatKey = cleanChatKey(chatKey)
	g.mu.RLock()
	defer g.mu.RUnlock()

	provider := g.provider
	if setting, ok := g.chatOverrides[chatKey]; ok {
		if setting.Provider != "" {
			provider = setting.Provider
		}
		if setting.Model != "" {
			return setting.Model
		}
	}

	switch provider {
	case "openrouter":
		return g.openRouterModel
	case "groq":
		return g.groqModel
	default:
		return g.model
	}
}

func (g *GeminiClient) GetAllChatOverrides() map[string]ChatAISetting {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make(map[string]ChatAISetting, len(g.chatOverrides))
	for k, v := range g.chatOverrides {
		result[k] = v
	}
	return result
}

func normalizePersonaStyle(style string) string {
	style = strings.ToLower(strings.TrimSpace(style))
	switch style {
	case "formal", "professional", "normal":
		return "formal"
	case "genz", "brainrot", "savage", "roast":
		return "genz"
	case "self", "user", "self user", "self_user", "me":
		return "self"
	default:
		return style
	}
}

func (g *GeminiClient) SetGlobalStyle(style string) error {
	style = normalizePersonaStyle(style)
	if style != "formal" && style != "genz" && style != "self" {
		return fmt.Errorf("invalid persona style %q (must be 'formal', 'genz', or 'self')", style)
	}
	g.mu.Lock()
	g.globalStyle = style
	g.mu.Unlock()

	if config.AppConfig != nil {
		if style == "genz" {
			config.AppConfig.BaseInstruction = config.AppConfig.BaseInstructionGenZ
		} else if style == "self" {
			config.AppConfig.BaseInstruction = config.AppConfig.BaseInstructionSelf
		} else {
			config.AppConfig.BaseInstruction = config.AppConfig.BaseInstructionFormal
		}
	}

	// Reset any cached test sessions so playground tests reflect new style
	g.ClearHistory("dash_test_")
	g.ClearHistory("dash_test_default")
	g.ClearHistory("dash_test_prism")
	return nil
}

func (g *GeminiClient) GetGlobalStyle() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.globalStyle == "" {
		return "formal"
	}
	return g.globalStyle
}

func (g *GeminiClient) SetChatStyle(chatKey, style string) error {
	cleaned := cleanChatKey(chatKey)
	if cleaned == "" {
		return errors.New("chat key cannot be empty")
	}
	style = normalizePersonaStyle(style)
	if style != "" && style != "formal" && style != "genz" && style != "self" {
		return fmt.Errorf("invalid persona style %q (must be 'formal', 'genz', 'self', or empty to inherit)", style)
	}

	g.mu.Lock()
	setting := g.chatOverrides[cleaned]
	setting.Style = style
	g.chatOverrides[cleaned] = setting
	g.saveChatOverridesLocked()
	g.mu.Unlock()

	// Clear conversation history so previous assistant tone doesn't contaminate new persona
	g.ClearHistory(chatKey)
	if cleaned != chatKey {
		g.ClearHistory(cleaned)
	}
	return nil
}

func (g *GeminiClient) GetChatStyle(chatKey string) string {
	chatKey = cleanChatKey(chatKey)
	g.mu.RLock()
	defer g.mu.RUnlock()

	if setting, ok := g.chatOverrides[chatKey]; ok && setting.Style != "" {
		return setting.Style
	}
	if g.globalStyle == "" {
		return "formal"
	}
	return g.globalStyle
}

func (g *GeminiClient) GetChatStyleDirect(chatKey string) (style string, hasOverride bool) {
	chatKey = cleanChatKey(chatKey)
	g.mu.RLock()
	defer g.mu.RUnlock()

	if setting, ok := g.chatOverrides[chatKey]; ok && setting.Style != "" {
		return setting.Style, true
	}
	return "", false
}


func (g *GeminiClient) SetBaseURL(url string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.baseURL = url
}

func (g *GeminiClient) apiURL() string {
	g.mu.RLock()
	base := g.baseURL
	g.mu.RUnlock()

	if base != "" {
		return fmt.Sprintf("%s/v1beta/models/%s:generateContent", strings.TrimRight(base, "/"), g.model)
	}
	return fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", g.model)
}

func sanitizeErrorString(msg, apiKey string) string {
	if apiKey != "" {
		msg = strings.ReplaceAll(msg, apiKey, "[REDACTED]")
	}
	re := regexp.MustCompile(`key=[a-zA-Z0-9_\-]+`)
	return re.ReplaceAllString(msg, "key=[REDACTED]")
}

func sanitizeError(err error, apiKey string) error {
	if err == nil {
		return nil
	}
	return errors.New(sanitizeErrorString(err.Error(), apiKey))
}

func (g *GeminiClient) generateOpenAICompatible(ctx context.Context, provider, modelOverride string, req GenerateRequest) (string, error) {
	g.mu.RLock()
	var endpoint string
	var apiKey string
	var model string

	switch provider {
	case "openrouter":
		endpoint = g.openRouterURL
		if endpoint == "" {
			endpoint = "https://openrouter.ai/api/v1/chat/completions"
		}
		apiKey = g.openRouterKey
		model = g.openRouterModel
	case "groq":
		endpoint = g.groqURL
		if endpoint == "" {
			endpoint = "https://api.groq.com/openai/v1/chat/completions"
		}
		apiKey = g.groqKey
		model = g.groqModel
	case "ollama":
		endpoint = g.baseURL
		if endpoint == "" {
			endpoint = "http://localhost:11434/v1/chat/completions"
		}
		apiKey = "ollama"
		model = g.openRouterModel
	}
	g.mu.RUnlock()

	if modelOverride != "" {
		model = modelOverride
	}

	if endpoint == "" {
		return "", fmt.Errorf("no API endpoint configured for provider %s", provider)
	}

	var messages []OpenAIMessage

	// 1. System instruction if present
	if req.SystemInstruction != nil && len(req.SystemInstruction.Parts) > 0 {
		var sysText strings.Builder
		for _, p := range req.SystemInstruction.Parts {
			sysText.WriteString(p.Text)
		}
		if sysText.Len() > 0 {
			messages = append(messages, OpenAIMessage{
				Role:    "system",
				Content: sysText.String(),
			})
		}
	}

	// 2. Chat contents
	for _, c := range req.Contents {
		role := c.Role
		if role == "model" {
			role = "assistant"
		} else if role == "" {
			role = "user"
		}

		hasMedia := false
		for _, p := range c.Parts {
			if p.InlineData != nil {
				hasMedia = true
				break
			}
		}

		if hasMedia {
			var multiParts []OpenAIMessageContent
			for _, p := range c.Parts {
				if p.InlineData != nil {
					multiParts = append(multiParts, OpenAIMessageContent{
						Type: "image_url",
						ImageURL: &OpenAIImageURL{
							URL: fmt.Sprintf("data:%s;base64,%s", p.InlineData.MimeType, p.InlineData.Data),
						},
					})
				}
				if p.Text != "" {
					multiParts = append(multiParts, OpenAIMessageContent{
						Type: "text",
						Text: p.Text,
					})
				}
			}
			messages = append(messages, OpenAIMessage{
				Role:    role,
				Content: multiParts,
			})
		} else {
			var sb strings.Builder
			for _, p := range c.Parts {
				sb.WriteString(p.Text)
			}
			messages = append(messages, OpenAIMessage{
				Role:    role,
				Content: sb.String(),
			})
		}
	}

	temp := 0.7
	if req.GenerationConfig != nil && req.GenerationConfig.Temperature > 0 {
		temp = req.GenerationConfig.Temperature
	}

	openAIReq := OpenAIChatRequest{
		Model:       model,
		Messages:    messages,
		Temperature: temp,
	}
	if req.GenerationConfig != nil && req.GenerationConfig.MaxOutputTokens > 0 {
		openAIReq.MaxTokens = req.GenerationConfig.MaxOutputTokens
	}

	reqBytes, err := json.Marshal(openAIReq)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBytes))
	if err != nil {
		return "", sanitizeError(err, apiKey)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" && apiKey != "ollama" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}
	if provider == "openrouter" {
		httpReq.Header.Set("HTTP-Referer", "https://github.com/PrismBot")
		httpReq.Header.Set("X-Title", "PrismBot WhatsApp")
	}

	resp, err := g.httpClient.Do(httpReq)
	if err != nil {
		return "", sanitizeError(err, apiKey)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp OpenAIChatResponse
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter := resp.Header.Get("Retry-After")
			resetTokens := resp.Header.Get("x-ratelimit-reset-tokens")
			waitHint := ""
			if retryAfter != "" {
				waitHint = fmt.Sprintf(" (retry after %ss)", retryAfter)
			} else if resetTokens != "" {
				waitHint = fmt.Sprintf(" (resets in %s)", resetTokens)
			}
			if errResp.Error != nil && errResp.Error.Message != "" {
				return "", fmt.Errorf("%s rate limit exceeded (HTTP 429)%s: %s", provider, waitHint, sanitizeErrorString(errResp.Error.Message, apiKey))
			}
			return "", fmt.Errorf("%s rate limit exceeded (HTTP 429)%s. Please try again in a few moments.", provider, waitHint)
		}
		if errResp.Error != nil && errResp.Error.Message != "" {
			return "", fmt.Errorf("%s API error (HTTP %d): %s", provider, resp.StatusCode, sanitizeErrorString(errResp.Error.Message, apiKey))
		}
		return "", fmt.Errorf("%s service error (HTTP %d)", provider, resp.StatusCode)
	}

	var chatResp OpenAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("failed to parse %s response: %w", provider, err)
	}

	if len(chatResp.Choices) > 0 {
		reply := chatResp.Choices[0].Message.Content
		if reply != "" {
			return reply, nil
		}
	}

	return "", fmt.Errorf("empty response from %s", provider)
}

func (g *GeminiClient) generateGemini(ctx context.Context, modelOverride string, req GenerateRequest) (string, error) {
	g.mu.RLock()
	key := g.apiKey
	model := g.model
	if modelOverride != "" {
		model = modelOverride
	}
	if model == "" {
		model = "gemini-3.5-flash-lite"
	}
	base := g.baseURL
	g.mu.RUnlock()

	if key == "" {
		return "", fmt.Errorf("GEMINI_API_KEY is not configured")
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model)
	if base != "" {
		url = fmt.Sprintf("%s/v1beta/models/%s:generateContent", strings.TrimRight(base, "/"), model)
	}

	buf := g.bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer g.bufPool.Put(buf)

	if err := json.NewEncoder(buf).Encode(req); err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, buf)
	if err != nil {
		return "", sanitizeError(err, key)
	}
	httpReq.Header.Set("x-goog-api-key", key)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(httpReq)
	if err != nil {
		return "", sanitizeError(err, key)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gemini service error (HTTP %d)", resp.StatusCode)
	}

	var genResp GenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		return "", fmt.Errorf("failed to parse Gemini response: %w", err)
	}

	if genResp.Error != nil {
		return "", fmt.Errorf("Gemini API error (%d): %s", genResp.Error.Code, genResp.Error.Message)
	}

	if len(genResp.Candidates) > 0 && len(genResp.Candidates[0].Content.Parts) > 0 {
		var sb strings.Builder
		for _, p := range genResp.Candidates[0].Content.Parts {
			sb.WriteString(p.Text)
		}
		return sb.String(), nil
	}

	return "", fmt.Errorf("empty response from Gemini")
}

func (g *GeminiClient) GenerateForChat(ctx context.Context, chatKey string, req GenerateRequest) (string, error) {
	provider := g.GetChatProvider(chatKey)
	model := g.GetChatModel(chatKey)
	if provider == "openrouter" || provider == "groq"  {
		return g.generateOpenAICompatible(ctx, provider, model, req)
	}
	return g.generateGemini(ctx, model, req)
}

func (g *GeminiClient) Generate(ctx context.Context, req GenerateRequest) (string, error) {
	return g.GenerateForChat(ctx, "", req)
}

func (g *GeminiClient) Chat(ctx context.Context, chatKey, prompt, systemInstruction string) (string, error) {
	return g.ChatWithMedia(ctx, chatKey, prompt, systemInstruction, "", nil)
}

func (g *GeminiClient) ChatWithMedia(ctx context.Context, chatKey, prompt, systemInstruction, mimeType string, data []byte) (string, error) {
	g.mu.Lock()
	history := g.histories[chatKey]
	if history == nil {
		history = make([]Content, 0, g.maxTurns*2)
	}

	var parts []Part
	if len(data) > 0 {
		b64 := base64.StdEncoding.EncodeToString(data)
		parts = append(parts, Part{
			InlineData: &InlineData{
				MimeType: mimeType,
				Data:     b64,
			},
		})
	}
	if prompt != "" {
		parts = append(parts, Part{Text: prompt})
	}

	userContent := Content{
		Role:  "user",
		Parts: parts,
	}
	contents := append(history, userContent)
	g.mu.Unlock()

	req := GenerateRequest{
		Contents: contents,
	}
	if systemInstruction != "" {
		req.SystemInstruction = &SystemInstruction{
			Parts: []Part{{Text: systemInstruction}},
		}
	}

	reply, err := g.GenerateForChat(ctx, chatKey, req)
	if err != nil {
		return "", err
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	// In the saved history, replace large binary media with a compact placeholder to prevent context overload on subsequent turns
	var historyUserParts []Part
	for _, p := range parts {
		if p.InlineData != nil {
			historyUserParts = append(historyUserParts, Part{
				Text: fmt.Sprintf("[Attached Media: %s]", p.InlineData.MimeType),
			})
		} else {
			historyUserParts = append(historyUserParts, p)
		}
	}
	historyUserContent := Content{
		Role:  "user",
		Parts: historyUserParts,
	}

	modelContent := Content{
		Role:  "model",
		Parts: []Part{{Text: reply}},
	}
	newHistory := append(history, historyUserContent, modelContent)

	maxItems := g.maxTurns * 2
	if len(newHistory) > maxItems {
		newHistory = newHistory[len(newHistory)-maxItems:]
	}
	g.histories[chatKey] = newHistory

	return reply, nil
}

// SolveProblem executes a standalone, context-free problem solving call.
// It explicitly ignores any multi-turn chat history or room transcript to prevent context overload.
func (g *GeminiClient) SolveProblemForChat(ctx context.Context, chatKey, prompt, systemInstruction, mimeType string, data []byte) (string, error) {
	var parts []Part
	if len(data) > 0 {
		b64 := base64.StdEncoding.EncodeToString(data)
		parts = append(parts, Part{
			InlineData: &InlineData{
				MimeType: mimeType,
				Data:     b64,
			},
		})
	}
	if prompt != "" {
		parts = append(parts, Part{Text: prompt})
	}

	req := GenerateRequest{
		Contents: []Content{
			{
				Role:  "user",
				Parts: parts,
			},
		},
	}
	if systemInstruction != "" {
		req.SystemInstruction = &SystemInstruction{
			Parts: []Part{{Text: systemInstruction}},
		}
	}

	return g.GenerateForChat(ctx, chatKey, req)
}

func (g *GeminiClient) SolveProblem(ctx context.Context, prompt, systemInstruction, mimeType string, data []byte) (string, error) {
	return g.SolveProblemForChat(ctx, "", prompt, systemInstruction, mimeType, data)
}

// SolveProblemWithModel generates a single-turn concise response using the specified model and max output tokens
func (g *GeminiClient) SolveProblemWithModel(ctx context.Context, model, prompt, systemInstruction string) (string, error) {
	req := GenerateRequest{
		Contents: []Content{
			{
				Role:  "user",
				Parts: []Part{{Text: prompt}},
			},
		},
		GenerationConfig: &GenerationConfig{
			Temperature:     0.7,
			MaxOutputTokens: 100,
		},
	}
	if systemInstruction != "" {
		req.SystemInstruction = &SystemInstruction{
			Parts: []Part{{Text: systemInstruction}},
		}
	}
	if strings.HasPrefix(model, "llama-") || strings.HasPrefix(model, "mixtral") || (g.GetProvider() == "groq" && !strings.HasPrefix(model, "gemini-")) {
		return g.generateOpenAICompatible(ctx, "groq", model, req)
	}
	return g.generateGemini(ctx, model, req)
}

// SolveMultiTurnWithModelStream streams multi-turn response tokens in real-time, calling onChunk as tokens arrive.
// Returns the aggregated final text. Falls back gracefully to non-streaming if stream connection encounters errors.
func (g *GeminiClient) SolveMultiTurnWithModelStream(ctx context.Context, model string, contents []Content, systemInstruction string, onChunk func(chunk string) error) (string, error) {
	req := GenerateRequest{
		Contents: contents,
		GenerationConfig: &GenerationConfig{
			Temperature:     0.7,
			MaxOutputTokens: 100,
		},
	}
	if systemInstruction != "" {
		req.SystemInstruction = &SystemInstruction{
			Parts: []Part{{Text: systemInstruction}},
		}
	}

	if strings.HasPrefix(model, "llama-") || strings.HasPrefix(model, "mixtral") || (g.GetProvider() == "groq" && !strings.HasPrefix(model, "gemini-")) {
		text, err := g.streamOpenAICompatible(ctx, "groq", model, req, onChunk)
		if err == nil && strings.TrimSpace(text) != "" {
			return text, nil
		}
		// Fallback to non-streaming
		return g.generateOpenAICompatible(ctx, "groq", model, req)
	}

	text, err := g.streamGemini(ctx, model, req, onChunk)
	if err == nil && strings.TrimSpace(text) != "" {
		return text, nil
	}
	// Fallback to non-streaming
	return g.generateGemini(ctx, model, req)
}

// SolveProblemWithModelStream streams single-turn response tokens in real-time, calling onChunk as tokens arrive.
// Returns the aggregated final text. Falls back gracefully to non-streaming if stream connection encounters errors.
func (g *GeminiClient) SolveProblemWithModelStream(ctx context.Context, model, prompt, systemInstruction string, onChunk func(chunk string) error) (string, error) {
	contents := []Content{
		{
			Role:  "user",
			Parts: []Part{{Text: prompt}},
		},
	}
	return g.SolveMultiTurnWithModelStream(ctx, model, contents, systemInstruction, onChunk)
}

func (g *GeminiClient) streamGemini(ctx context.Context, modelOverride string, req GenerateRequest, onChunk func(string) error) (string, error) {
	g.mu.RLock()
	key := g.apiKey
	model := g.model
	if modelOverride != "" {
		model = modelOverride
	}
	if model == "" {
		model = "gemini-3.5-flash-lite"
	}
	base := g.baseURL
	g.mu.RUnlock()

	if key == "" {
		return "", fmt.Errorf("GEMINI_API_KEY is not configured")
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:streamGenerateContent?alt=sse&key=%s", model, key)
	if base != "" {
		url = fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?alt=sse&key=%s", strings.TrimRight(base, "/"), model, key)
	}

	buf := g.bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer g.bufPool.Put(buf)

	if err := json.NewEncoder(buf).Encode(req); err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, buf)
	if err != nil {
		return "", sanitizeError(err, key)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(httpReq)
	if err != nil {
		return "", sanitizeError(err, key)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gemini streaming HTTP %d", resp.StatusCode)
	}

	var fullReply strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			var chunk struct {
				Candidates []struct {
					Content struct {
						Parts []struct {
							Text string `json:"text"`
						} `json:"parts"`
					} `json:"content"`
				} `json:"candidates"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err == nil {
				for _, cand := range chunk.Candidates {
					for _, p := range cand.Content.Parts {
						if p.Text != "" {
							fullReply.WriteString(p.Text)
							if onChunk != nil {
								if err := onChunk(p.Text); err != nil {
									return fullReply.String(), err
								}
							}
						}
					}
				}
			}
		}
	}

	if fullReply.Len() == 0 {
		return "", fmt.Errorf("empty streaming response from Gemini")
	}
	return fullReply.String(), nil
}

func (g *GeminiClient) streamOpenAICompatible(ctx context.Context, provider, modelOverride string, req GenerateRequest, onChunk func(string) error) (string, error) {
	g.mu.RLock()
	var endpoint, apiKey, model string
	switch provider {
	case "openrouter":
		endpoint = g.openRouterURL
		apiKey = g.openRouterKey
		model = g.openRouterModel
	case "groq":
		endpoint = g.groqURL
		if endpoint == "" {
			endpoint = "https://api.groq.com/openai/v1/chat/completions"
		}
		apiKey = g.groqKey
		model = g.groqModel
	default:
		endpoint = "https://api.groq.com/openai/v1/chat/completions"
		apiKey = g.groqKey
		model = g.groqModel
	}
	g.mu.RUnlock()

	if modelOverride != "" {
		model = modelOverride
	}

	if endpoint == "" || apiKey == "" {
		return "", fmt.Errorf("missing endpoint or API key for provider %s", provider)
	}

	var messages []OpenAIMessage
	if req.SystemInstruction != nil && len(req.SystemInstruction.Parts) > 0 {
		var sysText strings.Builder
		for _, p := range req.SystemInstruction.Parts {
			sysText.WriteString(p.Text)
		}
		if sysText.Len() > 0 {
			messages = append(messages, OpenAIMessage{
				Role:    "system",
				Content: sysText.String(),
			})
		}
	}

	for _, c := range req.Contents {
		role := c.Role
		if role == "model" {
			role = "assistant"
		} else if role == "" {
			role = "user"
		}
		var sb strings.Builder
		for _, p := range c.Parts {
			sb.WriteString(p.Text)
		}
		messages = append(messages, OpenAIMessage{
			Role:    role,
			Content: sb.String(),
		})
	}

	openAIReq := OpenAIChatRequest{
		Model:       model,
		Messages:    messages,
		Temperature: 0.7,
		MaxTokens:   100,
		Stream:      true,
	}

	reqBytes, err := json.Marshal(openAIReq)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBytes))
	if err != nil {
		return "", sanitizeError(err, apiKey)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := g.httpClient.Do(httpReq)
	if err != nil {
		return "", sanitizeError(err, apiKey)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s streaming HTTP %d", provider, resp.StatusCode)
	}

	var fullReply strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
			if data == "[DONE]" {
				break
			}
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err == nil {
				for _, ch := range chunk.Choices {
					tok := ch.Delta.Content
					if tok != "" {
						fullReply.WriteString(tok)
						if onChunk != nil {
							if err := onChunk(tok); err != nil {
								return fullReply.String(), err
							}
						}
					}
				}
			}
		}
	}

	if fullReply.Len() == 0 {
		return "", fmt.Errorf("empty streaming response from %s", provider)
	}
	return fullReply.String(), nil
}

func (g *GeminiClient) SetMaxTurns(turns int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if turns > 0 {
		g.maxTurns = turns
	}
}

func (g *GeminiClient) GetHistory(chatKey string) []Content {
	g.mu.RLock()
	defer g.mu.RUnlock()
	hist := g.histories[chatKey]
	if hist == nil {
		return nil
	}
	copied := make([]Content, len(hist))
	copy(copied, hist)
	return copied
}

func (g *GeminiClient) ClearHistory(chatKey string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.histories, chatKey)
}

func (g *GeminiClient) OCR(ctx context.Context, mimeType string, data []byte) (string, error) {
	prompt := "Extract all visible text. Preserve line breaks, paragraphs, and list structure."
	b64 := base64.StdEncoding.EncodeToString(data)

	req := GenerateRequest{
		Contents: []Content{
			{
				Role: "user",
				Parts: []Part{
					{
						InlineData: &InlineData{
							MimeType: mimeType,
							Data:     b64,
						},
					},
					{Text: prompt},
				},
			},
		},
	}
	return g.Generate(ctx, req)
}

func (g *GeminiClient) Summarize(ctx context.Context, text string) (string, error) {
	prompt := fmt.Sprintf("Summarize clearly in bullet points with key takeaways:\n\n%s", text)
	req := GenerateRequest{
		Contents: []Content{
			{
				Role:  "user",
				Parts: []Part{{Text: prompt}},
			},
		},
	}
	return g.Generate(ctx, req)
}

func (g *GeminiClient) Translate(ctx context.Context, text, targetLang string) (string, error) {
	prompt := fmt.Sprintf("Translate directly to %s. Output only the translation:\n\n%s", targetLang, text)
	req := GenerateRequest{
		Contents: []Content{
			{
				Role:  "user",
				Parts: []Part{{Text: prompt}},
			},
		},
	}
	return g.Generate(ctx, req)
}

func (g *GeminiClient) Calculate(ctx context.Context, expr string) (string, error) {
	prompt := fmt.Sprintf("Evaluate this expression: \"%s\". Give the exact result with a one-line explanation.", expr)
	req := GenerateRequest{
		Contents: []Content{
			{
				Role:  "user",
				Parts: []Part{{Text: prompt}},
			},
		},
	}
	return g.Generate(ctx, req)
}

func (g *GeminiClient) Define(ctx context.Context, word string) (string, error) {
	prompt := fmt.Sprintf("Define \"%s\": pronunciation, definition, synonyms, and one example sentence.", word)
	req := GenerateRequest{
		Contents: []Content{
			{
				Role:  "user",
				Parts: []Part{{Text: prompt}},
			},
		},
	}
	return g.Generate(ctx, req)
}

func (g *GeminiClient) Weather(ctx context.Context, city string) (string, error) {
	prompt := fmt.Sprintf("Current weather report for \"%s\": temperature, condition, humidity, and wind.", city)
	req := GenerateRequest{
		Contents: []Content{
			{
				Role:  "user",
				Parts: []Part{{Text: prompt}},
			},
		},
	}
	return g.Generate(ctx, req)
}

func (g *GeminiClient) FactOrQuote(ctx context.Context, isFact bool) (string, error) {
	var prompt string
	if isFact {
		prompt = "Give one verified scientific or historical fact in two sentences."
	} else {
		prompt = "Give one brief quote with author and context."
	}
	req := GenerateRequest{
		Contents: []Content{
			{
				Role:  "user",
				Parts: []Part{{Text: prompt}},
			},
		},
	}
	return g.Generate(ctx, req)
}

func (g *GeminiClient) CatchUpSummary(ctx context.Context, conversation string, msgCount int, isAway bool) (string, error) {
	systemPrompt := `You are PrismBot's Catch-Up AI Assistant on WhatsApp.
Your task is to analyze the provided conversation transcript and create a concise, high-signal, beautifully structured recap of what took place while the user was away.

Formatting Guidelines for WhatsApp:
- Use emojis and WhatsApp bold (*heading*) formatting.
- Structure your response into clean, scannable sections:
  📌 *Key Highlights & Decisions* (2-4 clear bullet points)
  👥 *Notable Updates & Discussions* (who talked about what, shared links, media, or updates)
  ⚡ *Action Items & Questions* (any open questions, requests, or tasks for the group or user)
- If the conversation was casual chit-chat or nothing major happened, briefly say so in a warm, friendly way.
- Do NOT repeat every message; synthesize the key points.
- Keep the summary under 250 words so it is quick to read on a mobile screen.`

	userPrompt := fmt.Sprintf("Here are the last %d messages from the chat:\n\n%s\n\nPlease provide a clear catch-up summary.", msgCount, conversation)

	req := GenerateRequest{
		Contents: []Content{
			{
				Role:  "user",
				Parts: []Part{{Text: userPrompt}},
			},
		},
		SystemInstruction: &SystemInstruction{
			Parts: []Part{{Text: systemPrompt}},
		},
	}

	return g.Generate(ctx, req)
}
