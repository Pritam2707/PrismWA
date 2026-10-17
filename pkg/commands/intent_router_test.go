package commands

import (
	"testing"
)

func TestFastPathIntent(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedIntent string
		expectedParam  string
		expectedVal    string
	}{
		{
			name:           "Sticker simple",
			input:          "sticker",
			expectedIntent: "sticker",
		},
		{
			name:           "Make sticker phrase",
			input:          "turn this into a sticker",
			expectedIntent: "sticker",
		},
		{
			name:           "Catchup default",
			input:          "catchup",
			expectedIntent: "catchup",
		},
		{
			name:           "Catchup with count",
			input:          "catchup 20",
			expectedIntent: "catchup",
			expectedParam:  "count",
			expectedVal:    "20",
		},
		{
			name:           "Remind in format",
			input:          "remind me in 15m to check the oven",
			expectedIntent: "remind",
			expectedParam:  "duration",
			expectedVal:    "15m",
		},
		{
			name:           "Remind direct format",
			input:          "remind 2h study physics",
			expectedIntent: "remind",
			expectedParam:  "duration",
			expectedVal:    "2h",
		},
		{
			name:           "Weather in city",
			input:          "weather in Tokyo",
			expectedIntent: "weather",
			expectedParam:  "city",
			expectedVal:    "Tokyo",
		},
		{
			name:           "OCR trigger",
			input:          "extract text",
			expectedIntent: "ocr",
		},
		{
			name:           "View-once trigger",
			input:          "reveal view once",
			expectedIntent: "viewonce",
		},
		{
			name:           "Calc trigger",
			input:          "calc 50 * 20 + 4",
			expectedIntent: "calc",
			expectedParam:  "expression",
			expectedVal:    "50 * 20 + 4",
		},
		{
			name:           "Define trigger",
			input:          "define serendipity",
			expectedIntent: "define",
			expectedParam:  "word",
			expectedVal:    "serendipity",
		},
		{
			name:           "Translate to trigger",
			input:          "translate to Spanish: Good morning",
			expectedIntent: "translate",
			expectedParam:  "target_lang",
			expectedVal:    "Spanish",
		},
		{
			name:           "Deleted messages trigger",
			input:          "show deleted messages",
			expectedIntent: "deleted",
		},
		{
			name:           "Status ping",
			input:          "ping",
			expectedIntent: "status",
		},
		{
			name:           "Play song simple",
			input:          "play fade",
			expectedIntent: "play",
			expectedParam:  "query",
			expectedVal:    "fade",
		},
		{
			name:           "Play video query",
			input:          "play video alan walker",
			expectedIntent: "video",
			expectedParam:  "query",
			expectedVal:    "alan walker",
		},
		{
			name:           "Roast trigger",
			input:          "roast me",
			expectedIntent: "roast",
		},
		{
			name:           "Brainrot trigger",
			input:          "brainrot subway surfers",
			expectedIntent: "brainrot",
			expectedParam:  "topic",
			expectedVal:    "subway surfers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intent := FastPathIntent(tt.input, false, false, false)
			if intent == nil {
				t.Fatalf("expected intent for '%s', got nil", tt.input)
			}
			if intent.Intent != tt.expectedIntent {
				t.Errorf("expected intent '%s', got '%s'", tt.expectedIntent, intent.Intent)
			}
			if tt.expectedParam != "" {
				val, ok := intent.Parameters[tt.expectedParam]
				if !ok {
					t.Errorf("expected param '%s' to be present", tt.expectedParam)
				} else if val != tt.expectedVal {
					t.Errorf("expected param '%s' = '%s', got '%s'", tt.expectedParam, tt.expectedVal, val)
				}
			}
		})
	}
}

func TestFastPathConversationalFallthrough(t *testing.T) {
	conversationalInputs := []string{
		"Hello, how are you today?",
		"Can you explain quantum computing to me?",
		"Write a python script to parse CSV files",
		"Tell me a joke about programming",
		"What is the best way to learn Go?",
	}

	for _, input := range conversationalInputs {
		t.Run(input, func(t *testing.T) {
			intent := FastPathIntent(input, false, false, false)
			if intent != nil && intent.Intent != "chat" {
				t.Errorf("expected conversational query '%s' to not fast-path to a command, got intent '%s'", input, intent.Intent)
			}
		})
	}
}
