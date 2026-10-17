package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"whatsapp-bot/pkg/gemini"
	"whatsapp-bot/pkg/store"
)

func TestHandleAuditDeleted(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dash_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	_ = store.InitPersistentStore(tempDir)
	_ = store.InitMessageStore(context.Background(), 100)

	chatJID := "999888777@g.us"
	audit := store.DeletedMessageAudit{
		ID:         "TEST_AUDIT_1",
		ChatJID:    chatJID,
		ChatName:   "Alpha Group",
		SenderJID:  "111222@s.whatsapp.net",
		SenderName: "Bob",
		PushName:   "Bob P",
		Text:       "Secret message",
		MediaType:  "text",
		DeletedAt:  time.Now(),
	}
	store.GlobalPersistent.RecordDeleted(audit)

	req := httptest.NewRequest(http.MethodGet, "/api/audit/deleted", nil)
	w := httptest.NewRecorder()

	handleAuditDeleted(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var resp struct {
		DeletedMsgs map[string][]map[string]interface{} `json:"deletedMsgs"`
		DeletedVo   map[string]interface{}              `json:"deletedVo"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	items, ok := resp.DeletedMsgs[chatJID]
	if !ok || len(items) == 0 {
		t.Fatalf("Expected deletedMsgs to contain chat %s", chatJID)
	}

	item := items[0]
	if item["senderName"] != "Bob" {
		t.Fatalf("Expected senderName 'Bob', got '%v'", item["senderName"])
	}
	if item["chatName"] != "Alpha Group" {
		t.Fatalf("Expected chatName 'Alpha Group', got '%v'", item["chatName"])
	}
	if item["text"] != "Secret message" {
		t.Fatalf("Expected text 'Secret message', got '%v'", item["text"])
	}

	msgObj, ok := item["message"].(map[string]interface{})
	if !ok || msgObj["conversation"] != "Secret message" {
		t.Fatalf("Expected message.conversation to be 'Secret message', got '%v'", item["message"])
	}
}

func TestNewEndpoints(t *testing.T) {
	// Test /api/storage
	reqStorage := httptest.NewRequest(http.MethodGet, "/api/storage", nil)
	wStorage := httptest.NewRecorder()
	handleStorage(wStorage, reqStorage)
	if wStorage.Code != http.StatusOK {
		t.Fatalf("Expected /api/storage status 200, got %d", wStorage.Code)
	}

	// Test /api/chats
	reqChats := httptest.NewRequest(http.MethodGet, "/api/chats", nil)
	wChats := httptest.NewRecorder()
	handleChats(wChats, reqChats)
	if wChats.Code != http.StatusOK {
		t.Fatalf("Expected /api/chats status 200, got %d", wChats.Code)
	}

	// Test /api/welcome
	reqWelcome := httptest.NewRequest(http.MethodGet, "/api/welcome", nil)
	wWelcome := httptest.NewRecorder()
	handleWelcome(wWelcome, reqWelcome)
	if wWelcome.Code != http.StatusOK {
		t.Fatalf("Expected /api/welcome status 200, got %d", wWelcome.Code)
	}

	// Test /api/moderation/mutes
	reqMutes := httptest.NewRequest(http.MethodGet, "/api/moderation/mutes", nil)
	wMutes := httptest.NewRecorder()
	handleModerationMutes(wMutes, reqMutes)
	if wMutes.Code != http.StatusOK {
		t.Fatalf("Expected /api/moderation/mutes status 200, got %d", wMutes.Code)
	}
}

func TestAIEndpoints(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ai_dash_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	client := gemini.NewClient("test-key", "gemini-3.5-flash-lite")
	client.ConfigureOpenRouter("test-or-key", "dolphin-mistral-test", "")
	client.ConfigureGroq("test-groq-key", "llama-test", "")

	// 1. Test /api/ai/config
	reqConfig := httptest.NewRequest(http.MethodGet, "/api/ai/config", nil)
	wConfig := httptest.NewRecorder()
	handleAIConfig(wConfig, reqConfig)
	if wConfig.Code != http.StatusOK {
		t.Fatalf("expected 200 from /api/ai/config, got %d", wConfig.Code)
	}

	var confResp map[string]interface{}
	if err := json.Unmarshal(wConfig.Body.Bytes(), &confResp); err != nil {
		t.Fatalf("failed to decode config response: %v", err)
	}
	if confResp["activeProvider"] != "gemini" {
		t.Errorf("expected activeProvider gemini, got %v", confResp["activeProvider"])
	}

	// 2. Test /api/ai/provider (switch to openrouter)
	bodyProv := `{"provider":"openrouter"}`
	reqProv := httptest.NewRequest(http.MethodPost, "/api/ai/provider", strings.NewReader(bodyProv))
	wProv := httptest.NewRecorder()
	handleAISetProvider(wProv, reqProv)
	if wProv.Code != http.StatusOK {
		t.Fatalf("expected 200 from /api/ai/provider, got %d", wProv.Code)
	}
	if client.GetProvider() != "openrouter" {
		t.Errorf("expected provider openrouter, got %s", client.GetProvider())
	}

	// 3. Test /api/ai/model
	bodyModel := `{"model":"nousresearch/hermes-3-llama-3.1-405b"}`
	reqModel := httptest.NewRequest(http.MethodPost, "/api/ai/model", strings.NewReader(bodyModel))
	wModel := httptest.NewRecorder()
	handleAISetModel(wModel, reqModel)
	if wModel.Code != http.StatusOK {
		t.Fatalf("expected 200 from /api/ai/model, got %d", wModel.Code)
	}
	if client.GetActiveModel() != "nousresearch/hermes-3-llama-3.1-405b" {
		t.Errorf("expected active model updated, got %s", client.GetActiveModel())
	}

	// 4. Test /api/ai/chat-override (set custom override for a chat)
	testChatJID := "919876543210@s.whatsapp.net"
	bodyOverride := `{"chatJid":"` + testChatJID + `","provider":"groq","model":"llama-3.3-70b-versatile"}`
	reqOverride := httptest.NewRequest(http.MethodPost, "/api/ai/chat-override", strings.NewReader(bodyOverride))
	wOverride := httptest.NewRecorder()
	handleAIChatOverride(wOverride, reqOverride)
	if wOverride.Code != http.StatusOK {
		t.Fatalf("expected 200 from /api/ai/chat-override, got %d", wOverride.Code)
	}

	// Verify chat override in client
	if client.GetChatProvider(testChatJID) != "groq" {
		t.Errorf("expected chat provider groq, got %s", client.GetChatProvider(testChatJID))
	}
	if client.GetChatModel(testChatJID) != "llama-3.3-70b-versatile" {
		t.Errorf("expected chat model llama-3.3-70b-versatile, got %s", client.GetChatModel(testChatJID))
	}

	// 5. Test /api/ai/chat-override DELETE (reset back to global default)
	reqDelete := httptest.NewRequest(http.MethodDelete, "/api/ai/chat-override?chatJid="+testChatJID, nil)
	wDelete := httptest.NewRecorder()
	handleAIChatOverride(wDelete, reqDelete)
	if wDelete.Code != http.StatusOK {
		t.Fatalf("expected 200 from delete /api/ai/chat-override, got %d", wDelete.Code)
	}

	// Chat should now inherit global default (openrouter)
	if client.GetChatProvider(testChatJID) != "openrouter" {
		t.Errorf("expected reset chat to inherit global openrouter, got %s", client.GetChatProvider(testChatJID))
	}

	// 6. Test /api/ai/style (toggle global style to genz)
	bodyStyle := `{"style":"genz"}`
	reqStyle := httptest.NewRequest(http.MethodPost, "/api/ai/style", strings.NewReader(bodyStyle))
	wStyle := httptest.NewRecorder()
	handleAISetStyle(wStyle, reqStyle)
	if wStyle.Code != http.StatusOK {
		t.Fatalf("expected 200 from /api/ai/style, got %d", wStyle.Code)
	}
	if client.GetGlobalStyle() != "genz" {
		t.Errorf("expected global style genz, got %s", client.GetGlobalStyle())
	}

	// 7. Test setting per-chat style override
	bodyChatStyle := `{"chatJid":"` + testChatJID + `","style":"formal"}`
	reqChatStyle := httptest.NewRequest(http.MethodPost, "/api/ai/chat-override", strings.NewReader(bodyChatStyle))
	wChatStyle := httptest.NewRecorder()
	handleAIChatOverride(wChatStyle, reqChatStyle)
	if wChatStyle.Code != http.StatusOK {
		t.Fatalf("expected 200 from setting chat style override, got %d", wChatStyle.Code)
	}
	if client.GetChatStyle(testChatJID) != "formal" {
		t.Errorf("expected chat style formal, got %s", client.GetChatStyle(testChatJID))
	}
}

func TestMeowCallerVoiceBotEndpoints(t *testing.T) {
	// Test GET config
	reqGet := httptest.NewRequest(http.MethodGet, "/api/meowcaller/voicebot/config", nil)
	wGet := httptest.NewRecorder()
	handleMeowCallerVoiceBotConfig(wGet, reqGet)
	if wGet.Code != http.StatusOK {
		t.Fatalf("expected 200 from GET voicebot config, got %d", wGet.Code)
	}

	// Test POST config toggle
	bodyConfig := `{"enabled":true,"geminiModel":"gemini-3.5-flash-lite","voice":"elon_musk","serverUrl":"ws://127.0.0.1:6006"}`
	reqPost := httptest.NewRequest(http.MethodPost, "/api/meowcaller/voicebot/config", strings.NewReader(bodyConfig))
	wPost := httptest.NewRecorder()
	handleMeowCallerVoiceBotConfig(wPost, reqPost)
	if wPost.Code != http.StatusOK {
		t.Fatalf("expected 200 from POST voicebot config, got %d", wPost.Code)
	}

	var respPost struct {
		Success  bool `json:"success"`
		VoiceBot struct {
			Enabled     bool   `json:"enabled"`
			GeminiModel string `json:"geminiModel"`
		} `json:"voiceBot"`
	}
	if err := json.Unmarshal(wPost.Body.Bytes(), &respPost); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !respPost.Success || !respPost.VoiceBot.Enabled {
		t.Errorf("expected voiceBot to be enabled, got %+v", respPost)
	}

	// Test POST connect endpoint
	bodyConn := `{"serverUrl":"ws://127.0.0.1:9999"}`
	reqConn := httptest.NewRequest(http.MethodPost, "/api/meowcaller/voicebot/connect", strings.NewReader(bodyConn))
	wConn := httptest.NewRecorder()
	handleMeowCallerVoiceBotConnect(wConn, reqConn)
	if wConn.Code != http.StatusOK {
		t.Fatalf("expected 200 from voicebot connect, got %d", wConn.Code)
	}

	var respConn struct {
		Success   bool   `json:"success"`
		Connected bool   `json:"connected"`
		Error     string `json:"error"`
	}
	if err := json.Unmarshal(wConn.Body.Bytes(), &respConn); err != nil {
		t.Fatalf("failed to decode connect response: %v", err)
	}
	if respConn.Success {
		t.Errorf("expected failure for unreachable port 9999, got success")
	}
	if respConn.Error == "" {
		t.Errorf("expected error message for unreachable port 9999")
	}
}


