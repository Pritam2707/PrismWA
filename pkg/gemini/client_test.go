package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChatWithMedia_HistorySanitization(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req GenerateRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		resp := GenerateResponse{
			Candidates: []Candidate{
				{
					Content: Content{
						Parts: []Part{{Text: "Mocked Gemini reply"}},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient("fake-key", "gemini-test")
	client.httpClient = ts.Client()
	client.SetBaseURL(ts.URL)

	chatKey := "test-chat-123"
	mediaBytes := []byte("massive-binary-image-data-here-123456789")

	reply, err := client.ChatWithMedia(context.Background(), chatKey, "What is this image?", "Test instruction", "image/png", mediaBytes)
	if err != nil {
		t.Fatalf("ChatWithMedia failed: %v", err)
	}
	if reply != "Mocked Gemini reply" {
		t.Errorf("unexpected reply: %s", reply)
	}

	// Verify history does NOT contain raw base64 data to prevent context overload
	hist := client.GetHistory(chatKey)
	if len(hist) < 2 {
		t.Fatalf("expected at least 2 history items, got %d", len(hist))
	}

	userTurn := hist[0]
	for _, p := range userTurn.Parts {
		if p.InlineData != nil {
			t.Errorf("expected inline data to be stripped from history to prevent context overload, but found InlineData: %+v", p.InlineData)
		}
		if strings.Contains(p.Text, "massive-binary-image-data") {
			t.Errorf("expected raw media string to be omitted from history")
		}
		if p.Text == "[Attached Media: image/png]" {
			// Expected placeholder
		}
	}
}

func TestSolveProblem_Isolation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := GenerateResponse{
			Candidates: []Candidate{
				{
					Content: Content{
						Parts: []Part{{Text: "Step 1: Formula\nFinal Answer: x = 5"}},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient("fake-key", "gemini-test")
	client.httpClient = ts.Client()
	client.SetBaseURL(ts.URL)

	chatKey := "isolated-solve-chat"
	client.histories[chatKey] = []Content{
		{Role: "user", Parts: []Part{{Text: "Old chat conversation"}}},
		{Role: "model", Parts: []Part{{Text: "Old response"}}},
	}

	res, err := client.SolveProblem(context.Background(), "2x = 10", "Math tutor instruction", "", nil)
	if err != nil {
		t.Fatalf("SolveProblem failed: %v", err)
	}
	if !strings.Contains(res, "Final Answer: x = 5") {
		t.Errorf("unexpected solve result: %s", res)
	}

	if len(client.histories[chatKey]) != 2 {
		t.Errorf("expected history length to remain 2, got %d", len(client.histories[chatKey]))
	}
}

func TestAPIKeyLeakPrevention_503(t *testing.T) {
	secretKey := "AIzaSySecretApiKey123456789"

	// Mock server that returns 503 Service Unavailable
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify API key was passed in header, NOT in URL query string
		if r.URL.Query().Get("key") != "" {
			t.Errorf("Security flaw: API key found in URL query parameters: %s", r.URL.RawQuery)
		}
		if r.Header.Get("x-goog-api-key") != secretKey {
			t.Errorf("Expected x-goog-api-key header to be %s, got %s", secretKey, r.Header.Get("x-goog-api-key"))
		}

		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("<html><body>503 Service Unavailable</body></html>"))
	}))
	defer ts.Close()

	client := NewClient(secretKey, "gemini-test")
	client.httpClient = ts.Client()
	client.SetBaseURL(ts.URL)

	_, err := client.Generate(context.Background(), GenerateRequest{
		Contents: []Content{{Role: "user", Parts: []Part{{Text: "Hello"}}}},
	})
	if err == nil {
		t.Fatalf("expected error from 503 response, got nil")
	}

	errStr := err.Error()
	if strings.Contains(errStr, secretKey) {
		t.Fatalf("CRITICAL SECURITY ERROR: Secret API key leaked in error message: %s", errStr)
	}
	if strings.Contains(errStr, "AIzaSy") {
		t.Fatalf("CRITICAL SECURITY ERROR: API key prefix leaked in error message: %s", errStr)
	}
	if !strings.Contains(errStr, "503") {
		t.Errorf("expected 503 status code in error, got: %s", errStr)
	}
}

func TestSanitizeError(t *testing.T) {
	secretKey := "AIzaSyVerySecret123"
	rawErr := errors.New("Post https://api.google.com/test?key=" + secretKey + ": connection timed out")
	sanitized := sanitizeError(rawErr, secretKey)
	if strings.Contains(sanitized.Error(), secretKey) {
		t.Errorf("sanitizeError failed to redact secret key: %s", sanitized.Error())
	}
	if !strings.Contains(sanitized.Error(), "[REDACTED]") {
		t.Errorf("expected [REDACTED] in error string, got %s", sanitized.Error())
	}
}

func TestOpenAICompatible_OpenRouter(t *testing.T) {
	apiKey := "sk-or-v1-secret-openrouter-key"
	expectedModel := "cognitivecomputations/dolphin-mistral-24b-venice-edition"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Headers
		if r.Header.Get("Authorization") != "Bearer "+apiKey {
			t.Errorf("expected Authorization header Bearer %s, got %s", apiKey, r.Header.Get("Authorization"))
		}
		if r.Header.Get("HTTP-Referer") == "" {
			t.Errorf("expected HTTP-Referer header to be present")
		}

		// Verify Request Body
		var body OpenAIChatRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if body.Model != expectedModel {
			t.Errorf("expected model %s, got %s", expectedModel, body.Model)
		}
		if len(body.Messages) == 0 {
			t.Errorf("expected at least 1 message in messages array")
		}

		resp := OpenAIChatResponse{
			Choices: []struct {
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
			}{
				{
					Message: struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					}{
						Role:    "assistant",
						Content: "Uncensored response from Dolphin Venice",
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient("gemini-key", "gemini-3.5-flash-lite")
	client.httpClient = ts.Client()
	client.ConfigureOpenRouter(apiKey, expectedModel, ts.URL)
	if err := client.SetProvider("openrouter"); err != nil {
		t.Fatalf("SetProvider failed: %v", err)
	}

	reply, err := client.Generate(context.Background(), GenerateRequest{
		Contents: []Content{
			{Role: "user", Parts: []Part{{Text: "Hello Dolphin"}}},
		},
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if reply != "Uncensored response from Dolphin Venice" {
		t.Errorf("unexpected reply: %s", reply)
	}
}

func TestOpenAICompatible_Groq(t *testing.T) {
	apiKey := "gsk_secretGroqKey123"
	expectedModel := "llama-3.3-70b-versatile"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+apiKey {
			t.Errorf("expected Authorization header Bearer %s, got %s", apiKey, r.Header.Get("Authorization"))
		}

		var body OpenAIChatRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Model != expectedModel {
			t.Errorf("expected model %s, got %s", expectedModel, body.Model)
		}

		resp := OpenAIChatResponse{
			Choices: []struct {
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
			}{
				{
					Message: struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					}{
						Role:    "assistant",
						Content: "Blazing fast Groq response",
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient("gemini-key", "gemini-3.5-flash-lite")
	client.httpClient = ts.Client()
	client.ConfigureGroq(apiKey, expectedModel, ts.URL)
	if err := client.SetProvider("groq"); err != nil {
		t.Fatalf("SetProvider failed: %v", err)
	}

	reply, err := client.Generate(context.Background(), GenerateRequest{
		Contents: []Content{
			{Role: "user", Parts: []Part{{Text: "Hello Groq"}}},
		},
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if reply != "Blazing fast Groq response" {
		t.Errorf("unexpected reply: %s", reply)
	}
}

func TestProviderSwitching_Validation(t *testing.T) {
	client := NewClient("", "")

	// OpenRouter should fail without key
	if err := client.SetProvider("openrouter"); err == nil {
		t.Errorf("expected error setting openrouter without key, got nil")
	}

	// Groq should fail without key
	if err := client.SetProvider("groq"); err == nil {
		t.Errorf("expected error setting groq without key, got nil")
	}

	// Ollama requires no API key
	if err := client.SetProvider("ollama"); err != nil {
		t.Errorf("expected ollama to succeed without key, got: %v", err)
	}
	if client.GetProvider() != "ollama" {
		t.Errorf("expected provider ollama, got %s", client.GetProvider())
	}
}

func TestChatOverrides_PersistenceAndRouting(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ai_override_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	client := NewClient("gemini-test-key", "gemini-3.5-flash-lite")
	client.overridesFile = filepath.Join(tempDir, "chat_ai_settings.json")
	client.ConfigureOpenRouter("or-test-key", "dolphin-mistral-24b", "")
	client.ConfigureGroq("groq-test-key", "llama-3.3-70b-versatile", "")

	chatA := "12345678@s.whatsapp.net"
	chatB := "98765432@g.us"

	// Initial: both chats should inherit global provider (gemini)
	if p := client.GetChatProvider(chatA); p != "gemini" {
		t.Errorf("expected chatA provider gemini, got %s", p)
	}
	if p := client.GetChatProvider(chatB); p != "gemini" {
		t.Errorf("expected chatB provider gemini, got %s", p)
	}

	// Override chatA to OpenRouter with custom model
	if err := client.SetChatProvider(chatA, "openrouter"); err != nil {
		t.Fatalf("SetChatProvider failed: %v", err)
	}
	client.SetChatModel(chatA, "nousresearch/hermes-3-llama-3.1-405b")

	// Override chatB to Groq
	if err := client.SetChatProvider(chatB, "groq"); err != nil {
		t.Fatalf("SetChatProvider failed: %v", err)
	}

	// Verify chatA has OpenRouter and custom model
	if p := client.GetChatProvider(chatA); p != "openrouter" {
		t.Errorf("expected chatA provider openrouter, got %s", p)
	}
	if m := client.GetChatModel(chatA); m != "nousresearch/hermes-3-llama-3.1-405b" {
		t.Errorf("expected chatA model hermes, got %s", m)
	}

	// Verify chatB has Groq and default groq model
	if p := client.GetChatProvider(chatB); p != "groq" {
		t.Errorf("expected chatB provider groq, got %s", p)
	}
	if m := client.GetChatModel(chatB); m != "llama-3.3-70b-versatile" {
		t.Errorf("expected chatB default model llama-3.3-70b-versatile, got %s", m)
	}

	// Verify persona chat keys format (e.g. "12345678@s.whatsapp.net:sukuna") inherits chatA override
	personaKey := chatA + ":sukuna"
	if p := client.GetChatProvider(personaKey); p != "openrouter" {
		t.Errorf("expected persona chat to inherit chatA provider openrouter, got %s", p)
	}

	// Verify persistence: create a new client pointing to same overrides file
	client2 := NewClient("gemini-test-key", "gemini-3.5-flash-lite")
	client2.overridesFile = filepath.Join(tempDir, "chat_ai_settings.json")
	client2.ConfigureOpenRouter("or-test-key", "dolphin-mistral-24b", "")
	client2.ConfigureGroq("groq-test-key", "llama-3.3-70b-versatile", "")
	client2.loadChatOverrides()

	if p := client2.GetChatProvider(chatA); p != "openrouter" {
		t.Errorf("expected restored client to have chatA provider openrouter, got %s", p)
	}
	if m := client2.GetChatModel(chatA); m != "nousresearch/hermes-3-llama-3.1-405b" {
		t.Errorf("expected restored client to have chatA model hermes, got %s", m)
	}

	// Clear override on chatA
	client.ClearChatOverride(chatA)
	if p := client.GetChatProvider(chatA); p != "gemini" {
		t.Errorf("expected cleared chatA to revert to global gemini, got %s", p)
	}
}

func TestOpenAICompatible_RateLimit429(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Rate limit reached for model openai/gpt-oss-120b: Limit 8000 TPM.",
			},
		})
	}))
	defer ts.Close()

	client := NewClient("", "")
	client.httpClient = ts.Client()
	client.ConfigureGroq("groq-secret-key", "openai/gpt-oss-120b", ts.URL)
	_ = client.SetProvider("groq")

	_, err := client.Generate(context.Background(), GenerateRequest{
		Contents: []Content{{Role: "user", Parts: []Part{{Text: "Hello"}}}},
	})
	if err == nil {
		t.Fatalf("expected error on 429, got nil")
	}
	if !strings.Contains(err.Error(), "429") || !strings.Contains(err.Error(), "retry after 5s") {
		t.Errorf("expected 429 and retry after in error message, got: %v", err)
	}
}

func TestPersonaStyle_GlobalAndChatOverrides(t *testing.T) {
	tempDir := t.TempDir()
	overridesPath := filepath.Join(tempDir, "chat_ai_settings.json")

	client := NewClient("gemini-test-key", "gemini-3.5-flash-lite")
	client.overridesFile = overridesPath

	// 1. Verify default style is formal
	if s := client.GetGlobalStyle(); s != "formal" {
		t.Errorf("expected default global style formal, got %s", s)
	}
	chatA := "11111@s.whatsapp.net"
	chatB := "22222@s.whatsapp.net"

	// 2. Chat with no override inherits formal
	if s := client.GetChatStyle(chatA); s != "formal" {
		t.Errorf("expected chatA style formal, got %s", s)
	}

	// 3. Set chatA to genz
	if err := client.SetChatStyle(chatA, "genz"); err != nil {
		t.Fatalf("SetChatStyle failed: %v", err)
	}
	if s := client.GetChatStyle(chatA); s != "genz" {
		t.Errorf("expected chatA style genz, got %s", s)
	}
	// chatB still formal
	if s := client.GetChatStyle(chatB); s != "formal" {
		t.Errorf("expected chatB style formal, got %s", s)
	}

	// 3b. Test 3rd style: self / user
	if err := client.SetChatStyle(chatB, "self"); err != nil {
		t.Fatalf("SetChatStyle self failed: %v", err)
	}
	if s := client.GetChatStyle(chatB); s != "self" {
		t.Errorf("expected chatB style self, got %s", s)
	}
	// Test alias "self user"
	chatC := "33333@s.whatsapp.net"
	if err := client.SetChatStyle(chatC, "self user"); err != nil {
		t.Fatalf("SetChatStyle 'self user' failed: %v", err)
	}
	if s := client.GetChatStyle(chatC); s != "self" {
		t.Errorf("expected chatC normalized style self, got %s", s)
	}

	// 4. Test invalid style
	if err := client.SetGlobalStyle("invalid"); err == nil {
		t.Errorf("expected error for invalid global style, got nil")
	}
	if err := client.SetChatStyle(chatA, "invalid"); err == nil {
		t.Errorf("expected error for invalid chat style, got nil")
	}

	// 5. Test persistence
	client2 := NewClient("gemini-test-key", "gemini-3.5-flash-lite")
	client2.overridesFile = overridesPath
	client2.loadChatOverrides()

	if s := client2.GetChatStyle(chatA); s != "genz" {
		t.Errorf("expected restored client to have chatA style genz, got %s", s)
	}
	if s := client2.GetChatStyle(chatB); s != "self" {
		t.Errorf("expected restored client to have chatB style self, got %s", s)
	}

	// 6. Reset chatA style
	if err := client.SetChatStyle(chatA, ""); err != nil {
		t.Fatalf("clearing chat style failed: %v", err)
	}
	if s := client.GetChatStyle(chatA); s != "formal" {
		t.Errorf("expected reset chatA style to revert to formal, got %s", s)
	}
}



