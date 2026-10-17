package commands

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/polymorfa/hypermeow/types"

	"whatsapp-bot/pkg/store"
)

func TestIsProblemSolvingIntent(t *testing.T) {
	tests := []struct {
		name        string
		rawText     string
		cleanPrompt string
		extCtx      *ExtractedContext
		want        bool
	}{
		{
			name:        "solve prefix",
			rawText:     "!solve 2x + 4 = 10",
			cleanPrompt: "2x + 4 = 10",
			want:        true,
		},
		{
			name:        "math prefix",
			rawText:     "!math integral of x^2 dx",
			cleanPrompt: "integral of x^2 dx",
			want:        true,
		},
		{
			name:        "calc prefix",
			rawText:     "!calc (15 * 3) / 2",
			cleanPrompt: "(15 * 3) / 2",
			want:        true,
		},
		{
			name:        "solve in clean prompt",
			rawText:     "@prism can you solve this puzzle for me?",
			cleanPrompt: "can you solve this puzzle for me?",
			want:        true,
		},
		{
			name:        "solve equation prompt",
			rawText:     "@prism solve equation 3x - 9 = 0",
			cleanPrompt: "solve equation 3x - 9 = 0",
			want:        true,
		},
		{
			name:        "find value prompt",
			rawText:     "@prism find the value of x when 2x=8",
			cleanPrompt: "find the value of x when 2x=8",
			want:        true,
		},
		{
			name:        "step by step solution",
			rawText:     "@bot provide step by step solution for this",
			cleanPrompt: "provide step by step solution for this",
			want:        true,
		},
		{
			name:        "image with empty prompt",
			rawText:     "",
			cleanPrompt: "",
			extCtx: &ExtractedContext{
				MediaData: []byte("fake-image-bytes"),
				IsImage:   true,
			},
			want: true,
		},
		{
			name:        "image with solve prompt",
			rawText:     "solve this question",
			cleanPrompt: "solve this question",
			extCtx: &ExtractedContext{
				MediaData: []byte("fake-image-bytes"),
				IsImage:   true,
			},
			want: true,
		},
		{
			name:        "conversational question - not solve",
			rawText:     "@prism how are you today?",
			cleanPrompt: "how are you today?",
			want:        false,
		},
		{
			name:        "group discussion inquiry - not solve",
			rawText:     "@prism what did Alice say about the meeting?",
			cleanPrompt: "what did Alice say about the meeting?",
			want:        false,
		},
		{
			name:        "creative prompt - not solve",
			rawText:     "!ai write a poem about autumn leaves",
			cleanPrompt: "write a poem about autumn leaves",
			want:        false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &CommandContext{
				RawText: tc.rawText,
			}
			got := isProblemSolvingIntent(c, tc.cleanPrompt, tc.extCtx)
			if got != tc.want {
				t.Errorf("isProblemSolvingIntent() = %v; want %v for prompt %q", got, tc.want, tc.cleanPrompt)
			}
		})
	}
}

func TestBuildRecentChatContext(t *testing.T) {
	// Initialize store
	s := store.InitMessageStore(context.Background(), 200)

	chat1 := "1111111111@s.whatsapp.net"
	chat2 := "2222222222@s.whatsapp.net"

	senderA := types.NewJID("userA", "s.whatsapp.net")
	senderB := types.NewJID("userB", "s.whatsapp.net")

	now := time.Now()

	// Insert messages for Chat 1
	s.Put(&store.CachedMessage{
		ID:        "msg-1",
		ChatJID:   types.NewJID("1111111111", "s.whatsapp.net"),
		SenderJID: senderA,
		PushName:  "Alice",
		Text:      "Hey everyone, are we still meeting?",
		Timestamp: now.Add(-10 * time.Minute),
	})
	s.Put(&store.CachedMessage{
		ID:        "msg-2",
		ChatJID:   types.NewJID("1111111111", "s.whatsapp.net"),
		SenderJID: senderB,
		PushName:  "Bob",
		Text:      "Yes, let's meet at 4 PM in room 3.",
		Timestamp: now.Add(-5 * time.Minute),
	})

	// Insert message for Chat 2 (must be isolated and never appear in Chat 1 context)
	s.Put(&store.CachedMessage{
		ID:        "msg-3",
		ChatJID:   types.NewJID("2222222222", "s.whatsapp.net"),
		SenderJID: senderA,
		PushName:  "Alice",
		Text:      "Secret message from other chat!",
		Timestamp: now.Add(-2 * time.Minute),
	})

	// Insert a very long message in Chat 1 to test truncation
	longText := strings.Repeat("A", 300)
	s.Put(&store.CachedMessage{
		ID:        "msg-4",
		ChatJID:   types.NewJID("1111111111", "s.whatsapp.net"),
		SenderJID: senderB,
		PushName:  "Bob",
		Text:      longText,
		Timestamp: now.Add(-1 * time.Minute),
	})

	// Test Chat 1 context
	chatCtx := buildRecentChatContext(chat1, "msg-exclude", 50)
	if !strings.Contains(chatCtx, "Alice: Hey everyone, are we still meeting?") {
		t.Errorf("expected Chat 1 context to contain Alice's message, got:\n%s", chatCtx)
	}
	if !strings.Contains(chatCtx, "Bob: Yes, let's meet at 4 PM in room 3.") {
		t.Errorf("expected Chat 1 context to contain Bob's message, got:\n%s", chatCtx)
	}
	if strings.Contains(chatCtx, "Secret message from other chat!") {
		t.Errorf("chat context isolation violated: found Chat 2 message in Chat 1 context:\n%s", chatCtx)
	}

	// Verify long message is truncated to prevent context overload
	if strings.Contains(chatCtx, longText) {
		t.Errorf("expected long message to be truncated to avoid token overload")
	}
	if !strings.Contains(chatCtx, "...") {
		t.Errorf("expected truncated message to have ellipsis")
	}

	// Test Chat 2 context
	chat2Ctx := buildRecentChatContext(chat2, "msg-exclude", 50)
	if !strings.Contains(chat2Ctx, "Secret message from other chat!") {
		t.Errorf("expected Chat 2 context to contain Chat 2 message, got:\n%s", chat2Ctx)
	}
	if strings.Contains(chat2Ctx, "Hey everyone, are we still meeting?") {
		t.Errorf("chat context isolation violated: found Chat 1 message in Chat 2 context")
	}
}

func TestBuildSelfPersonaInstruction_PerChatIsolation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "persona_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ps := store.InitPersistentStore(tempDir)
	if ps == nil {
		t.Fatalf("InitPersistentStore returned nil")
	}
	defer ps.Close()

	chatA := "1111111111@s.whatsapp.net"
	chatB := "2222222222@s.whatsapp.net"
	chatEmpty := "9999999999@s.whatsapp.net"

	// 1. Test empty chat fallback
	emptyInstruction := buildSelfPersonaInstruction(chatEmpty, "")
	if !strings.Contains(emptyInstruction, "Persona: Self User") {
		t.Errorf("expected empty chat to contain fallback Self User instruction, got:\n%s", emptyInstruction)
	}

	// 2. Populate Chat A with casual user messages, bot replies, and commands
	ps.SaveChatMessage(&store.ChatMessageRecord{
		ID:         "msg-a-1",
		ChatJID:    chatA,
		SenderJID:  chatA,
		SenderName: "Me",
		Text:       "yo bro wya rn",
		IsFromMe:   true,
		IsBot:      false,
		Timestamp:  time.Now().Add(-15 * time.Minute),
	})
	ps.SaveChatMessage(&store.ChatMessageRecord{
		ID:         "msg-a-2",
		ChatJID:    chatA,
		SenderJID:  chatA,
		SenderName: "Me",
		Text:       "lmao dead fr",
		IsFromMe:   true,
		IsBot:      false,
		Timestamp:  time.Now().Add(-10 * time.Minute),
	})
	// Bot command (should be filtered out)
	ps.SaveChatMessage(&store.ChatMessageRecord{
		ID:         "msg-a-cmd",
		ChatJID:    chatA,
		SenderJID:  chatA,
		SenderName: "Me",
		Text:       "!style self",
		IsFromMe:   true,
		IsBot:      false,
		Timestamp:  time.Now().Add(-8 * time.Minute),
	})
	// Bot response (should be filtered out)
	ps.RecordBotMessage("msg-a-bot", chatA, "👤 Persona Style Updated to SELF USER!")

	// 3. Populate Chat B with formal user messages
	ps.SaveChatMessage(&store.ChatMessageRecord{
		ID:         "msg-b-1",
		ChatJID:    chatB,
		SenderJID:  chatB,
		SenderName: "Me",
		Text:       "Greetings, please find attached the revised budget report.",
		IsFromMe:   true,
		IsBot:      false,
		Timestamp:  time.Now().Add(-5 * time.Minute),
	})
	ps.RecordBotMessage("msg-b-bot", chatB, "Understood, message received.")

	// 4. Test Chat A persona instruction
	instrA := buildSelfPersonaInstruction(chatA, "msg-exclude")
	if !strings.Contains(instrA, "yo bro wya rn") || !strings.Contains(instrA, "lmao dead fr") {
		t.Errorf("expected Chat A instruction to contain Chat A user samples, got:\n%s", instrA)
	}
	// Verify bot messages and commands are filtered out
	if strings.Contains(instrA, "!style self") {
		t.Errorf("Chat A instruction should NOT contain command invocations")
	}
	if strings.Contains(instrA, "Persona Style Updated to SELF USER") {
		t.Errorf("Chat A instruction should NOT contain bot sent messages")
	}
	// Verify Chat B messages are NOT in Chat A
	if strings.Contains(instrA, "revised budget report") {
		t.Errorf("Chat isolation failed: Chat B message found in Chat A persona!")
	}

	// 5. Test Chat B persona instruction
	instrB := buildSelfPersonaInstruction(chatB, "")
	if !strings.Contains(instrB, "revised budget report") {
		t.Errorf("expected Chat B instruction to contain Chat B user sample, got:\n%s", instrB)
	}
	if strings.Contains(instrB, "yo bro wya rn") || strings.Contains(instrB, "lmao dead fr") {
		t.Errorf("Chat isolation failed: Chat A message found in Chat B persona!")
	}
	if strings.Contains(instrB, "message received") {
		t.Errorf("Chat B instruction should NOT contain bot messages")
	}
}

func TestBuildRecentChatContext_PersistentSQLite(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "history_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ps := store.InitPersistentStore(tempDir)
	if ps == nil {
		t.Fatalf("InitPersistentStore returned nil")
	}
	defer ps.Close()

	chatJID := "5555555555@s.whatsapp.net"
	ps.SaveChatMessage(&store.ChatMessageRecord{
		ID:         "msg-h-1",
		ChatJID:    chatJID,
		SenderJID:  "123@s.whatsapp.net",
		SenderName: "Alice",
		Text:       "hey are you free today?",
		IsFromMe:   false,
		IsBot:      false,
		Timestamp:  time.Now().Add(-2 * time.Minute),
	})
	ps.SaveChatMessage(&store.ChatMessageRecord{
		ID:         "msg-h-2",
		ChatJID:    chatJID,
		SenderJID:  "456@s.whatsapp.net",
		SenderName: "GlitchyLime",
		Text:       "yeah finishing some work rn",
		IsFromMe:   true,
		IsBot:      false,
		Timestamp:  time.Now().Add(-1 * time.Minute),
	})
	ps.RecordBotMessage("msg-h-bot", chatJID, "bot automated notification")

	contextLog := buildRecentChatContext(chatJID, "", 10)
	if !strings.Contains(contextLog, "Alice: hey are you free today?") {
		t.Errorf("expected contextLog to contain Alice's message, got:\n%s", contextLog)
	}
	if !strings.Contains(contextLog, "GlitchyLime: yeah finishing some work rn") && !strings.Contains(contextLog, "You: yeah finishing some work rn") {
		t.Errorf("expected contextLog to contain user's message, got:\n%s", contextLog)
	}
	if strings.Contains(contextLog, "bot automated notification") {
		t.Errorf("contextLog must NOT contain bot automated messages")
	}
}

