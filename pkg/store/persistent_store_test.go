package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPersistentStoreDeletedMessages(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "store_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ps := InitPersistentStore(tempDir)
	if ps == nil {
		t.Fatalf("InitPersistentStore returned nil")
	}

	chat1 := "123456789@g.us"
	audit1 := DeletedMessageAudit{
		ID:         "MSG001",
		ChatJID:    chat1,
		ChatName:   "Test Group",
		SenderJID:  "user1@s.whatsapp.net",
		SenderName: "Alice",
		PushName:   "Alice P",
		Text:       "Hello deleted world",
		MediaType:  "text",
		DeletedAt:  time.Now(),
	}

	ps.RecordDeleted(audit1)

	// Verify querying for chat
	msgs := ps.GetDeletedMessagesForChat(chat1, 10)
	if len(msgs) != 1 {
		t.Fatalf("Expected 1 message for chat %s, got %d", chat1, len(msgs))
	}
	if msgs[0].Text != "Hello deleted world" {
		t.Fatalf("Expected text 'Hello deleted world', got '%s'", msgs[0].Text)
	}
	if msgs[0].ChatName != "Test Group" {
		t.Fatalf("Expected ChatName 'Test Group', got '%s'", msgs[0].ChatName)
	}

	// Verify SQLite table query
	if ps.db != nil {
		var text string
		row := ps.db.QueryRow("SELECT text FROM deleted_messages WHERE id = ?", "MSG001")
		if err := row.Scan(&text); err != nil {
			t.Fatalf("Failed to query SQLite deleted_messages table: %v", err)
		}
		if text != "Hello deleted world" {
			t.Fatalf("Expected SQLite text 'Hello deleted world', got '%s'", text)
		}
	}

	// Verify JSON file format
	jsonPath := filepath.Join(tempDir, "deleted_messages.json")
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatalf("Expected deleted_messages.json to exist at %s", jsonPath)
	}

	// Verify delete entry
	deleted := ps.DeleteAuditEntry(chat1, "MSG001")
	if !deleted {
		t.Fatalf("Expected DeleteAuditEntry to return true")
	}

	afterDelete := ps.GetDeletedMessagesForChat(chat1, 10)
	if len(afterDelete) != 0 {
		t.Fatalf("Expected 0 messages after deletion, got %d", len(afterDelete))
	}
}

func TestRealDataInit(t *testing.T) {
	ps := InitPersistentStore("../../data")
	if ps == nil {
		t.Fatalf("Failed to init data persistent store")
	}
	t.Logf("Loaded %d deleted messages into persistent store & SQLite", len(ps.deletedMsgs))
}

func TestPersistentStoreChatMessages_RestartAndFiltering(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "chat_msg_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ps := InitPersistentStore(tempDir)
	if ps == nil {
		t.Fatalf("InitPersistentStore returned nil")
	}

	chatA := "11111@s.whatsapp.net"
	chatB := "22222@s.whatsapp.net"

	// 1. Add user messages and bot messages in Chat A
	ps.SaveChatMessage(&ChatMessageRecord{
		ID:         "msg-a-1",
		ChatJID:    chatA,
		SenderJID:  "11111@s.whatsapp.net",
		SenderName: "Me",
		Text:       "yo bro wya rn",
		IsFromMe:   true,
		IsBot:      false,
		Timestamp:  time.Now().Add(-10 * time.Minute),
	})

	ps.SaveChatMessage(&ChatMessageRecord{
		ID:         "msg-a-2",
		ChatJID:    chatA,
		SenderJID:  "11111@s.whatsapp.net",
		SenderName: "Me",
		Text:       "lmao no way fr",
		IsFromMe:   true,
		IsBot:      false,
		Timestamp:  time.Now().Add(-8 * time.Minute),
	})

	// Bot command message (should be filtered out)
	ps.SaveChatMessage(&ChatMessageRecord{
		ID:         "msg-a-cmd",
		ChatJID:    chatA,
		SenderJID:  "11111@s.whatsapp.net",
		SenderName: "Me",
		Text:       "!style self",
		IsFromMe:   true,
		IsBot:      false,
		Timestamp:  time.Now().Add(-6 * time.Minute),
	})

	// Bot response message (should be filtered out)
	ps.RecordBotMessage("msg-a-bot-1", chatA, "👤 Persona Style Updated to SELF USER!")

	// 2. Add distinct messages in Chat B
	ps.SaveChatMessage(&ChatMessageRecord{
		ID:         "msg-b-1",
		ChatJID:    chatB,
		SenderJID:  "22222@s.whatsapp.net",
		SenderName: "Me",
		Text:       "Good morning, please find attached the report.",
		IsFromMe:   true,
		IsBot:      false,
		Timestamp:  time.Now().Add(-5 * time.Minute),
	})

	// Bot reply in Chat B
	ps.RecordBotMessage("msg-b-bot-1", chatB, "Understood. The report is logged.")

	// Verify IsBotMessage
	if !ps.IsBotMessage("msg-a-bot-1") {
		t.Errorf("expected msg-a-bot-1 to be recognized as bot message")
	}
	if ps.IsBotMessage("msg-a-1") {
		t.Errorf("expected msg-a-1 to NOT be recognized as bot message")
	}

	// 3. Query Chat A user messages
	userMsgsA := ps.GetChatUserMessages(chatA, 50, "msg-exclude")
	if len(userMsgsA) != 2 {
		t.Fatalf("expected 2 user messages for Chat A, got %d", len(userMsgsA))
	}
	if userMsgsA[0].Text != "yo bro wya rn" || userMsgsA[1].Text != "lmao no way fr" {
		t.Errorf("unexpected messages for Chat A: %v, %v", userMsgsA[0].Text, userMsgsA[1].Text)
	}

	// 4. Query Chat B user messages (different persona per chat!)
	userMsgsB := ps.GetChatUserMessages(chatB, 50, "")
	if len(userMsgsB) != 1 {
		t.Fatalf("expected 1 user message for Chat B, got %d", len(userMsgsB))
	}
	if userMsgsB[0].Text != "Good morning, please find attached the report." {
		t.Errorf("unexpected message for Chat B: %s", userMsgsB[0].Text)
	}

	// 5. Test persistence across restart: Reopen persistent store with same tempDir
	if ps.db != nil {
		_ = ps.db.Close()
	}

	psRestarted := InitPersistentStore(tempDir)
	if psRestarted == nil {
		t.Fatalf("failed to re-init persistent store")
	}

	restoredMsgsA := psRestarted.GetChatUserMessages(chatA, 50, "")
	if len(restoredMsgsA) != 2 {
		t.Fatalf("expected 2 restored messages for Chat A after restart, got %d", len(restoredMsgsA))
	}
	if restoredMsgsA[0].Text != "yo bro wya rn" || restoredMsgsA[1].Text != "lmao no way fr" {
		t.Errorf("restored messages mismatch: %v, %v", restoredMsgsA[0].Text, restoredMsgsA[1].Text)
	}

	restoredMsgsB := psRestarted.GetChatUserMessages(chatB, 50, "")
	if len(restoredMsgsB) != 1 {
		t.Fatalf("expected 1 restored message for Chat B after restart, got %d", len(restoredMsgsB))
	}
	if restoredMsgsB[0].Text != "Good morning, please find attached the report." {
		t.Errorf("restored message B mismatch: %s", restoredMsgsB[0].Text)
	}
}
