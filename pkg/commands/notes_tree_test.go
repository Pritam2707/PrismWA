package commands

import (
	"strings"
	"testing"
)

func TestNotesTreeBuilder(t *testing.T) {
	testNotes := map[string]*NoteEntry{
		"root/public/maths/calculus": {
			Content:   "Calculus notes",
			MediaType: "PDF",
		},
		"root/public/maths/algebra": {
			Content:   "Algebra formulas",
			MediaType: "TXT",
		},
		"root/public/physics/mechanics": {
			Content:   "Newton laws",
			MediaType: "DOC",
		},
		"root/private/personal/passwords": {
			Content:   "Secret",
			MediaType: "TXT",
			IsPrivate: true,
		},
	}

	// 1. Root directory test
	rootTree, rootItems := buildTreeAndIndexMap("root/", testNotes)
	if !strings.Contains(rootTree, "public/") || !strings.Contains(rootTree, "private/") {
		t.Fatalf("Root tree missing public or private: %s", rootTree)
	}
	if len(rootItems) < 2 {
		t.Fatalf("Expected at least 2 items at root, got %d", len(rootItems))
	}

	// 2. Subdirectory test
	pubTree, pubItems := buildTreeAndIndexMap("root/public/", testNotes)
	if !strings.Contains(pubTree, "maths/") || !strings.Contains(pubTree, "physics/") {
		t.Fatalf("Public subtree missing maths or physics: %s", pubTree)
	}
	if len(pubItems) == 0 {
		t.Fatalf("Expected indexed items in public subtree")
	}

	// 3. Nested folder test
	mathTree, mathItems := buildTreeAndIndexMap("root/public/maths/", testNotes)
	if !strings.Contains(mathTree, "calculus (PDF)") || !strings.Contains(mathTree, "algebra (TXT)") {
		t.Fatalf("Maths subtree missing files: %s", mathTree)
	}
	if len(mathItems) != 2 {
		t.Fatalf("Expected 2 math notes, got %d", len(mathItems))
	}
}

func TestResolveNoteFullPath(t *testing.T) {
	cases := []struct {
		raw      string
		current  string
		expected string
	}{
		{"math/calc", "root/", "root/public/math/calc"},
		{"private/my_note", "root/", "root/private/my_note"},
		{"formula", "root/public/math/", "root/public/math/formula"},
		{"root/anime/zero2", "root/", "root/anime/zero2"},
	}

	for _, c := range cases {
		actual := resolveNoteFullPath(c.raw, c.current)
		if actual != c.expected {
			t.Errorf("resolveNoteFullPath(%q, %q) = %q; want %q", c.raw, c.current, actual, c.expected)
		}
	}
}

func TestPrivateNotesScoping(t *testing.T) {
	mgr := &NotesManager{
		notes: map[string]*NoteEntry{
			"root/public/announcement": {
				Content:   "Public news",
				MediaType: "TXT",
				IsPrivate: false,
			},
			"root/private/groupA/secret": {
				Content:   "Group A secret info",
				MediaType: "TXT",
				IsPrivate: true,
				ScopeJID:  "120363424413794839@g.us",
			},
			"root/private/userB/journal": {
				Content:   "User B personal diary",
				MediaType: "TXT",
				IsPrivate: true,
				ScopeJID:  "57291306987527:1@s.whatsapp.net", // test device JID format
			},
			"root/private/orphaned/note": {
				Content:   "No scope defined",
				MediaType: "TXT",
				IsPrivate: true,
				ScopeJID:  "", // should never leak
			},
		},
	}

	// 1. Group A should only see its own private note and public notes
	groupANotes := mgr.GetAllAccessible("120363424413794839@g.us", true)
	if _, ok := groupANotes["root/public/announcement"]; !ok {
		t.Errorf("Group A should see public note")
	}
	if _, ok := groupANotes["root/private/groupA/secret"]; !ok {
		t.Errorf("Group A should see its own private note")
	}
	if _, ok := groupANotes["root/private/userB/journal"]; ok {
		t.Errorf("Group A MUST NOT see User B's private note!")
	}
	if _, ok := groupANotes["root/private/orphaned/note"]; ok {
		t.Errorf("Group A MUST NOT see orphaned private note without ScopeJID!")
	}

	// 2. User B (testing non-AD matching with 57291306987527@s.whatsapp.net)
	userBNotes := mgr.GetAllAccessible("57291306987527@s.whatsapp.net", true)
	if _, ok := userBNotes["root/public/announcement"]; !ok {
		t.Errorf("User B should see public note")
	}
	if _, ok := userBNotes["root/private/userB/journal"]; !ok {
		t.Errorf("User B should see its own private note even with device JID")
	}
	if _, ok := userBNotes["root/private/groupA/secret"]; ok {
		t.Errorf("User B MUST NOT see Group A's private note!")
	}

	// 3. Unrelated Group C should see ZERO private notes
	groupCNotes := mgr.GetAllAccessible("9999999999@g.us", true)
	if len(groupCNotes) != 1 {
		t.Errorf("Group C should only see 1 public note, got %d", len(groupCNotes))
	}
	if _, ok := groupCNotes["root/public/announcement"]; !ok {
		t.Errorf("Group C should see public note")
	}
	if _, ok := groupCNotes["root/private/groupA/secret"]; ok {
		t.Errorf("Group C MUST NOT see Group A's private note!")
	}
	if _, ok := groupCNotes["root/private/userB/journal"]; ok {
		t.Errorf("Group C MUST NOT see User B's private note!")
	}
}
