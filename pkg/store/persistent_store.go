package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type NoteItem struct {
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"createdAt"`
}

type AutoReplyRule struct {
	Trigger  string `json:"trigger"`
	Response string `json:"response"`
	Exact    bool   `json:"exact"`
}

type DeletedMessageAudit struct {
	ID            string    `json:"id"`
	ChatJID       string    `json:"chatJid"`
	ChatName      string    `json:"chatName"`
	SenderJID     string    `json:"senderJid"`
	SenderName    string    `json:"senderName"`
	PushName      string    `json:"pushName"`
	Text          string    `json:"text"`
	MediaType     string    `json:"mediaType"`
	LocalFilePath string    `json:"localFilePath,omitempty"`
	DeletedAt     time.Time `json:"deletedAt"`
}

type ChatMessageRecord struct {
	ID         string    `json:"id"`
	ChatJID    string    `json:"chatJid"`
	SenderJID  string    `json:"senderJid"`
	SenderName string    `json:"senderName"`
	Text       string    `json:"text"`
	IsFromMe   bool      `json:"isFromMe"`
	IsBot      bool      `json:"isBot"`
	Timestamp  time.Time `json:"timestamp"`
}

type PersistentStore struct {
	mu          sync.RWMutex
	dataDir     string
	db          *sql.DB
	notes       map[string]map[string]NoteItem
	autoReplies []AutoReplyRule
	deletedMsgs []DeletedMessageAudit
	notesFile   string
	autoRepFile string
	deletedFile string
	botMsgIDs   map[string]bool
	chatHistory map[string][]*ChatMessageRecord
}

var GlobalPersistent *PersistentStore

func InitPersistentStore(dataDir string) *PersistentStore {
	if dataDir == "" {
		dataDir = "data"
	}
	_ = os.MkdirAll(dataDir, 0755)

	dbPath := filepath.Join(dataDir, "prismbot.db")
	dbURI := fmt.Sprintf("file:%s?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)", filepath.ToSlash(dbPath))
	db, err := sql.Open("sqlite", dbURI)
	if err != nil {
		log.Printf("[PersistentStore] WARN: Failed to open sqlite DB at %s: %v", dbPath, err)
	} else {
		initSchema(db)
	}

	ps := &PersistentStore{
		dataDir:     dataDir,
		db:          db,
		notes:       make(map[string]map[string]NoteItem),
		autoReplies: make([]AutoReplyRule, 0),
		deletedMsgs: make([]DeletedMessageAudit, 0),
		notesFile:   filepath.Join(dataDir, "notes.json"),
		autoRepFile: filepath.Join(dataDir, "autoreply.json"),
		deletedFile: filepath.Join(dataDir, "deleted_messages.json"),
		botMsgIDs:   make(map[string]bool),
		chatHistory: make(map[string][]*ChatMessageRecord),
	}

	ps.loadAll()
	GlobalPersistent = ps
	return ps
}

func initSchema(db *sql.DB) {
	query := `
	CREATE TABLE IF NOT EXISTS deleted_messages (
		id TEXT PRIMARY KEY,
		chat_jid TEXT,
		chat_name TEXT,
		sender_jid TEXT,
		sender_name TEXT,
		push_name TEXT,
		text TEXT,
		media_type TEXT,
		local_file_path TEXT,
		deleted_at DATETIME
	);
	CREATE INDEX IF NOT EXISTS idx_del_chat_jid ON deleted_messages(chat_jid);
	CREATE INDEX IF NOT EXISTS idx_del_deleted_at ON deleted_messages(deleted_at);

	CREATE TABLE IF NOT EXISTS chat_messages (
		id TEXT PRIMARY KEY,
		chat_jid TEXT NOT NULL,
		sender_jid TEXT NOT NULL,
		sender_name TEXT,
		text TEXT NOT NULL,
		is_from_me INTEGER NOT NULL DEFAULT 0,
		is_bot INTEGER NOT NULL DEFAULT 0,
		timestamp DATETIME NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_chat_msg_chat_time ON chat_messages(chat_jid, timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_chat_msg_sender ON chat_messages(sender_jid);
	`
	if _, err := db.Exec(query); err != nil {
		log.Printf("[PersistentStore] WARN: Failed to initialize database schema: %v", err)
	}
}

func (ps *PersistentStore) loadAll() {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// 1. Load notes
	if data, err := os.ReadFile(ps.notesFile); err == nil {
		_ = json.Unmarshal(data, &ps.notes)
	}

	// 2. Load autoreply
	if data, err := os.ReadFile(ps.autoRepFile); err == nil {
		_ = json.Unmarshal(data, &ps.autoReplies)
	}

	// 3. Load deleted messages from JSON (support both flat array and map-by-JID)
	loadedJSON := make(map[string]DeletedMessageAudit)
	if data, err := os.ReadFile(ps.deletedFile); err == nil && len(data) > 0 {
		var list []DeletedMessageAudit
		if err := json.Unmarshal(data, &list); err == nil {
			for _, m := range list {
				loadedJSON[m.ID] = m
			}
		} else {
			var grouped map[string][]DeletedMessageAudit
			if err := json.Unmarshal(data, &grouped); err == nil {
				for _, items := range grouped {
					for _, m := range items {
						loadedJSON[m.ID] = m
					}
				}
			}
		}
	}

	// 4. Load or sync with SQLite database
	if ps.db != nil {
		rows, err := ps.db.Query(`
			SELECT id, chat_jid, chat_name, sender_jid, sender_name, push_name, text, media_type, local_file_path, deleted_at 
			FROM deleted_messages 
			ORDER BY deleted_at ASC 
			LIMIT 1000
		`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var a DeletedMessageAudit
				var delAt string
				var cName, sName, pName, mType, lPath sql.NullString
				if scanErr := rows.Scan(&a.ID, &a.ChatJID, &cName, &a.SenderJID, &sName, &pName, &a.Text, &mType, &lPath, &delAt); scanErr == nil {
					a.ChatName = cName.String
					a.SenderName = sName.String
					a.PushName = pName.String
					a.MediaType = mType.String
					a.LocalFilePath = lPath.String
					if t, tErr := time.Parse(time.RFC3339, delAt); tErr == nil {
						a.DeletedAt = t
					} else if t2, tErr2 := time.Parse("2006-01-02 15:04:05.999999999-07:00", delAt); tErr2 == nil {
						a.DeletedAt = t2
					} else if t3, tErr3 := time.Parse("2006-01-02 15:04:05", delAt); tErr3 == nil {
						a.DeletedAt = t3
					} else {
						a.DeletedAt = time.Now()
					}
					loadedJSON[a.ID] = a
				}
			}
		}

		// Ensure existing JSON records are also present in SQLite using a single batch transaction
		if len(loadedJSON) > 0 {
			tx, txErr := ps.db.Begin()
			if txErr == nil {
				stmt, prepErr := tx.Prepare(`
					INSERT INTO deleted_messages (id, chat_jid, chat_name, sender_jid, sender_name, push_name, text, media_type, local_file_path, deleted_at)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
					ON CONFLICT(id) DO UPDATE SET
						chat_name = excluded.chat_name,
						sender_name = excluded.sender_name,
						push_name = excluded.push_name,
						text = excluded.text,
						media_type = excluded.media_type,
						local_file_path = excluded.local_file_path,
						deleted_at = excluded.deleted_at;
				`)
				if prepErr == nil {
					for _, item := range loadedJSON {
						delAtStr := item.DeletedAt.Format(time.RFC3339)
						_, _ = stmt.Exec(
							item.ID,
							item.ChatJID,
							item.ChatName,
							item.SenderJID,
							item.SenderName,
							item.PushName,
							item.Text,
							item.MediaType,
							item.LocalFilePath,
							delAtStr,
						)
					}
					_ = stmt.Close()
				}
				_ = tx.Commit()
			}
		}
	}

	// Convert loaded map into chronological slice
	ps.deletedMsgs = make([]DeletedMessageAudit, 0, len(loadedJSON))
	for _, v := range loadedJSON {
		ps.deletedMsgs = append(ps.deletedMsgs, v)
	}

	// Save cleanly formatted JSON
	ps.flushDeletedJSONLocked()
}

func (ps *PersistentStore) upsertSQLite(audit DeletedMessageAudit) {
	if ps.db == nil {
		return
	}
	query := `
	INSERT INTO deleted_messages (id, chat_jid, chat_name, sender_jid, sender_name, push_name, text, media_type, local_file_path, deleted_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		chat_name = excluded.chat_name,
		sender_name = excluded.sender_name,
		push_name = excluded.push_name,
		text = excluded.text,
		media_type = excluded.media_type,
		local_file_path = excluded.local_file_path,
		deleted_at = excluded.deleted_at;
	`
	delAtStr := audit.DeletedAt.Format(time.RFC3339)
	_, _ = ps.db.Exec(query,
		audit.ID,
		audit.ChatJID,
		audit.ChatName,
		audit.SenderJID,
		audit.SenderName,
		audit.PushName,
		audit.Text,
		audit.MediaType,
		audit.LocalFilePath,
		delAtStr,
	)
}

func (ps *PersistentStore) flushDeletedJSONLocked() {
	grouped := make(map[string][]DeletedMessageAudit)
	for _, m := range ps.deletedMsgs {
		chat := m.ChatJID
		if chat == "" {
			chat = "unknown"
		}
		grouped[chat] = append(grouped[chat], m)
	}

	if data, err := json.MarshalIndent(grouped, "", "  "); err == nil {
		_ = os.WriteFile(ps.deletedFile, data, 0644)
	}
}

func (ps *PersistentStore) flushDeletedJSON() {
	ps.mu.RLock()
	grouped := make(map[string][]DeletedMessageAudit)
	for _, m := range ps.deletedMsgs {
		chat := m.ChatJID
		if chat == "" {
			chat = "unknown"
		}
		grouped[chat] = append(grouped[chat], m)
	}
	ps.mu.RUnlock()

	if data, err := json.MarshalIndent(grouped, "", "  "); err == nil {
		_ = os.WriteFile(ps.deletedFile, data, 0644)
	}
}

// Notes methods
func (ps *PersistentStore) SaveNote(chatJID, name, content, author string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if _, ok := ps.notes[chatJID]; !ok {
		ps.notes[chatJID] = make(map[string]NoteItem)
	}

	ps.notes[chatJID][name] = NoteItem{
		Name:      name,
		Content:   content,
		Author:    author,
		CreatedAt: time.Now(),
	}

	ps.saveNotesFile()
}

func (ps *PersistentStore) GetNote(chatJID, name string) (NoteItem, bool) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if chatNotes, ok := ps.notes[chatJID]; ok {
		item, found := chatNotes[name]
		return item, found
	}
	return NoteItem{}, false
}

func (ps *PersistentStore) ListNotes(chatJID string) []string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	keys := make([]string, 0)
	if chatNotes, ok := ps.notes[chatJID]; ok {
		for k := range chatNotes {
			keys = append(keys, k)
		}
	}
	return keys
}

func (ps *PersistentStore) DeleteNote(chatJID, name string) bool {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if chatNotes, ok := ps.notes[chatJID]; ok {
		if _, exists := chatNotes[name]; exists {
			delete(chatNotes, name)
			ps.saveNotesFile()
			return true
		}
	}
	return false
}

func (ps *PersistentStore) saveNotesFile() {
	if data, err := json.MarshalIndent(ps.notes, "", "  "); err == nil {
		_ = os.WriteFile(ps.notesFile, data, 0644)
	}
}

// Auto-Reply methods
func (ps *PersistentStore) AddAutoReply(rule AutoReplyRule) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.autoReplies = append(ps.autoReplies, rule)
	ps.saveAutoReplyFile()
}

func (ps *PersistentStore) GetAutoReplies() []AutoReplyRule {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	result := make([]AutoReplyRule, len(ps.autoReplies))
	copy(result, ps.autoReplies)
	return result
}

// MatchAutoReply tests text against all auto-reply rules in-memory with zero slice allocations
func (ps *PersistentStore) MatchAutoReply(text string) (string, bool) {
	if text == "" {
		return "", false
	}
	lower := strings.ToLower(strings.TrimSpace(text))

	ps.mu.RLock()
	defer ps.mu.RUnlock()

	for _, rule := range ps.autoReplies {
		trig := strings.ToLower(strings.TrimSpace(rule.Trigger))
		if trig == "" {
			continue
		}
		if rule.Exact {
			if lower == trig {
				return rule.Response, true
			}
		} else {
			if strings.Contains(lower, trig) {
				return rule.Response, true
			}
		}
	}
	return "", false
}

func (ps *PersistentStore) saveAutoReplyFile() {
	if data, err := json.MarshalIndent(ps.autoReplies, "", "  "); err == nil {
		_ = os.WriteFile(ps.autoRepFile, data, 0644)
	}
}

// Deleted messages methods
func (ps *PersistentStore) RecordDeleted(audit DeletedMessageAudit) {
	ps.mu.Lock()

	// 1. Update in-memory slice
	updated := false
	for i, m := range ps.deletedMsgs {
		if m.ID == audit.ID {
			ps.deletedMsgs[i] = audit
			updated = true
			break
		}
	}
	if !updated {
		ps.deletedMsgs = append(ps.deletedMsgs, audit)
	}

	if len(ps.deletedMsgs) > 1000 {
		ps.deletedMsgs = ps.deletedMsgs[len(ps.deletedMsgs)-1000:]
	}
	ps.mu.Unlock()

	// 2. Persist to SQLite DB table outside memory mutex
	ps.upsertSQLite(audit)

	// 3. Persist to JSON file outside memory mutex
	ps.flushDeletedJSON()
}

func (ps *PersistentStore) GetDeletedMessages(limit int) []DeletedMessageAudit {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if limit <= 0 || limit > len(ps.deletedMsgs) {
		limit = len(ps.deletedMsgs)
	}

	start := len(ps.deletedMsgs) - limit
	result := make([]DeletedMessageAudit, limit)
	copy(result, ps.deletedMsgs[start:])
	return result
}

func (ps *PersistentStore) GetDeletedMessagesForChat(chatJID string, limit int) []DeletedMessageAudit {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	var matching []DeletedMessageAudit
	for _, m := range ps.deletedMsgs {
		if m.ChatJID == chatJID {
			matching = append(matching, m)
		}
	}

	if limit <= 0 || limit > len(matching) {
		limit = len(matching)
	}

	start := len(matching) - limit
	result := make([]DeletedMessageAudit, limit)
	copy(result, matching[start:])
	return result
}

func (ps *PersistentStore) GetAllDeletedGrouped() map[string][]DeletedMessageAudit {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	grouped := make(map[string][]DeletedMessageAudit)
	for _, m := range ps.deletedMsgs {
		chat := m.ChatJID
		if chat == "" {
			chat = "unknown"
		}
		grouped[chat] = append(grouped[chat], m)
	}
	return grouped
}

func (ps *PersistentStore) DeleteAuditEntry(chatJID, targetID string) bool {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	foundIdx := -1
	for i, m := range ps.deletedMsgs {
		if m.ID == targetID && (chatJID == "" || m.ChatJID == chatJID) {
			foundIdx = i
			break
		}
	}

	if foundIdx >= 0 {
		removed := ps.deletedMsgs[foundIdx]
		ps.deletedMsgs = append(ps.deletedMsgs[:foundIdx], ps.deletedMsgs[foundIdx+1:]...)

		if removed.LocalFilePath != "" {
			_ = os.Remove(removed.LocalFilePath)
		}

		if ps.db != nil {
			_, _ = ps.db.Exec("DELETE FROM deleted_messages WHERE id = ?", targetID)
		}

		ps.flushDeletedJSONLocked()
		return true
	}

	return false
}

func (ps *PersistentStore) DeleteAllAuditForChat(chatJID string) bool {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	remaining := make([]DeletedMessageAudit, 0, len(ps.deletedMsgs))
	deletedCount := 0
	for _, m := range ps.deletedMsgs {
		if m.ChatJID == chatJID {
			deletedCount++
			if m.LocalFilePath != "" {
				_ = os.Remove(m.LocalFilePath)
			}
		} else {
			remaining = append(remaining, m)
		}
	}

	if deletedCount > 0 {
		ps.deletedMsgs = remaining
		if ps.db != nil {
			_, _ = ps.db.Exec("DELETE FROM deleted_messages WHERE chat_jid = ?", chatJID)
		}
		ps.flushDeletedJSONLocked()
		return true
	}

	return false
}

// ClearAllAudit purges all stored deleted messages and associated temp media
func (ps *PersistentStore) ClearAllAudit() {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	for _, m := range ps.deletedMsgs {
		if m.LocalFilePath != "" {
			_ = os.Remove(m.LocalFilePath)
		}
	}
	ps.deletedMsgs = make([]DeletedMessageAudit, 0)
	if ps.db != nil {
		_, _ = ps.db.Exec("DELETE FROM deleted_messages")
	}
	ps.flushDeletedJSONLocked()
}

func cleanJID(jid string) string {
	jid = strings.TrimSpace(jid)
	if at := strings.Index(jid, "@"); at != -1 {
		user := jid[:at]
		domain := jid[at:]
		if colon := strings.Index(user, ":"); colon != -1 {
			user = user[:colon]
		}
		if colon := strings.Index(domain, ":"); colon != -1 {
			domain = domain[:colon]
		}
		return user + domain
	}
	if colon := strings.Index(jid, ":"); colon != -1 {
		return jid[:colon]
	}
	return jid
}

func (ps *PersistentStore) SaveChatMessage(msg *ChatMessageRecord) error {
	if msg == nil || msg.ID == "" {
		return nil
	}

	cleanedChat := cleanJID(msg.ChatJID)
	cleanedSender := cleanJID(msg.SenderJID)
	msg.ChatJID = cleanedChat
	msg.SenderJID = cleanedSender

	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	fromMeInt := 0
	if msg.IsFromMe {
		fromMeInt = 1
	}

	ps.mu.Lock()
	if ps.botMsgIDs[msg.ID] {
		msg.IsBot = true
	}
	isBotInt := 0
	if msg.IsBot {
		isBotInt = 1
		ps.botMsgIDs[msg.ID] = true
	}

	// Update in-memory chat history cache
	list := ps.chatHistory[cleanedChat]
	found := false
	for _, existing := range list {
		if existing.ID == msg.ID {
			existing.Text = msg.Text
			existing.IsFromMe = msg.IsFromMe
			if msg.IsBot {
				existing.IsBot = true
			}
			found = true
			break
		}
	}
	if !found {
		if len(list) >= 200 {
			list = list[1:]
		}
		ps.chatHistory[cleanedChat] = append(list, msg)
	}
	db := ps.db
	ps.mu.Unlock()

	// Persist to SQLite outside memory mutex to eliminate lock contention on hot message paths
	if db != nil {
		_, err := db.Exec(`
			INSERT INTO chat_messages (id, chat_jid, sender_jid, sender_name, text, is_from_me, is_bot, timestamp)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				text = excluded.text,
				is_from_me = excluded.is_from_me,
				is_bot = CASE WHEN chat_messages.is_bot = 1 THEN 1 ELSE excluded.is_bot END,
				timestamp = excluded.timestamp
		`, msg.ID, cleanedChat, cleanedSender, msg.SenderName, msg.Text, fromMeInt, isBotInt, msg.Timestamp)
		if err != nil {
			log.Printf("[PersistentStore] Error saving chat message %s: %v", msg.ID, err)
		}
	}

	return nil
}

func (ps *PersistentStore) RecordBotMessage(id, chatJID, text string) error {
	if id == "" {
		return nil
	}
	cleanedChat := cleanJID(chatJID)

	ps.mu.Lock()
	ps.botMsgIDs[id] = true
	if list, ok := ps.chatHistory[cleanedChat]; ok {
		for _, m := range list {
			if m.ID == id {
				m.IsBot = true
				break
			}
		}
	}
	db := ps.db
	ps.mu.Unlock()

	// Persist to SQLite outside memory mutex
	if db != nil {
		_, _ = db.Exec(`
			INSERT INTO chat_messages (id, chat_jid, sender_jid, sender_name, text, is_from_me, is_bot, timestamp)
			VALUES (?, ?, 'bot', 'PrismBot', ?, 1, 1, ?)
			ON CONFLICT(id) DO UPDATE SET is_bot = 1
		`, id, cleanedChat, text, time.Now())
	}

	return nil
}

func (ps *PersistentStore) IsBotMessage(id string) bool {
	if id == "" {
		return false
	}
	ps.mu.RLock()
	if ps.botMsgIDs[id] {
		ps.mu.RUnlock()
		return true
	}
	ps.mu.RUnlock()

	if ps.db != nil {
		var isBot int
		err := ps.db.QueryRow("SELECT is_bot FROM chat_messages WHERE id = ?", id).Scan(&isBot)
		if err == nil && isBot == 1 {
			ps.mu.Lock()
			ps.botMsgIDs[id] = true
			ps.mu.Unlock()
			return true
		}
	}
	return false
}

func (ps *PersistentStore) GetChatUserMessages(chatJID string, limit int, excludeID string) []*ChatMessageRecord {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}

	cleanedChat := cleanJID(chatJID)
	var candidates []*ChatMessageRecord

	if ps.db != nil {
		rows, err := ps.db.Query(`
			SELECT id, chat_jid, sender_jid, sender_name, text, is_from_me, is_bot, timestamp
			FROM chat_messages
			WHERE (chat_jid = ? OR chat_jid = ?) AND is_bot = 0
			ORDER BY timestamp DESC
			LIMIT 60
		`, cleanedChat, chatJID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var r ChatMessageRecord
				var fromMe, isBot int
				if scanErr := rows.Scan(&r.ID, &r.ChatJID, &r.SenderJID, &r.SenderName, &r.Text, &fromMe, &isBot, &r.Timestamp); scanErr == nil {
					r.IsFromMe = (fromMe == 1)
					r.IsBot = (isBot == 1)
					candidates = append(candidates, &r)
				}
			}
		}
	}

	if len(candidates) == 0 {
		if list, ok := ps.chatHistory[cleanedChat]; ok {
			for i := len(list) - 1; i >= 0; i-- {
				candidates = append(candidates, list[i])
			}
		}
	}

	cleanUserText := func(raw string) string {
		t := strings.TrimSpace(raw)
		if t == "" {
			return ""
		}
		// Strip bot command invocations
		if strings.HasPrefix(t, "!") || strings.HasPrefix(t, ".") || strings.HasPrefix(t, "/") || strings.HasPrefix(t, "#") {
			return ""
		}
		// Strip @prism and @bot mentions
		for _, tag := range []string{"@prism", "@bot"} {
			lower := strings.ToLower(t)
			for idx := strings.Index(lower, tag); idx != -1; idx = strings.Index(lower, tag) {
				t = strings.TrimSpace(t[:idx] + t[idx+len(tag):])
				lower = strings.ToLower(t)
			}
		}
		t = strings.TrimLeft(strings.TrimSpace(t), ":,-? ")
		return t
	}

	var fromMeFiltered []*ChatMessageRecord
	var generalFiltered []*ChatMessageRecord

	for _, m := range candidates {
		if m == nil || m.ID == excludeID || m.IsBot {
			continue
		}
		cleaned := cleanUserText(m.Text)
		if cleaned == "" {
			continue
		}
		mCopy := *m
		mCopy.Text = cleaned

		if m.IsFromMe {
			fromMeFiltered = append(fromMeFiltered, &mCopy)
		}
		generalFiltered = append(generalFiltered, &mCopy)
	}

	// If this chat has NO messages sent by user, supplement with user's messages across other chats
	if len(fromMeFiltered) == 0 && len(generalFiltered) == 0 && ps.db != nil {
		rows, err := ps.db.Query(`
			SELECT id, chat_jid, sender_jid, sender_name, text, is_from_me, is_bot, timestamp
			FROM chat_messages
			WHERE is_from_me = 1 AND is_bot = 0 AND (chat_jid != ? AND chat_jid != ?)
			ORDER BY timestamp DESC
			LIMIT ?
		`, cleanedChat, chatJID, limit*2)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var r ChatMessageRecord
				var fromMe, isBot int
				if scanErr := rows.Scan(&r.ID, &r.ChatJID, &r.SenderJID, &r.SenderName, &r.Text, &fromMe, &isBot, &r.Timestamp); scanErr == nil {
					r.IsFromMe = (fromMe == 1)
					r.IsBot = (isBot == 1)
					cleaned := cleanUserText(r.Text)
					if cleaned == "" {
						continue
					}
					r.Text = cleaned
					fromMeFiltered = append(fromMeFiltered, &r)
					if len(fromMeFiltered) >= limit {
						break
					}
				}
			}
		}
	}

	// Prioritize user's own sent messages (IsFromMe = true) if present
	selected := fromMeFiltered
	if len(selected) == 0 {
		selected = generalFiltered
	}

	if len(selected) > limit {
		selected = selected[:limit]
	}

	// Reverse to chronological order (oldest to newest)
	result := make([]*ChatMessageRecord, len(selected))
	for i, j := 0, len(selected)-1; i < len(selected); i, j = i+1, j-1 {
		result[i] = selected[j]
	}

	return result
}

// GetRecentChatHistoryPersistent retrieves up to `limit` recent messages for this chat in chronological order,
// querying SQLite so history persists across bot restarts, and filtering out bot replies.
func (ps *PersistentStore) GetRecentChatHistoryPersistent(chatJID string, limit int, excludeID string) []*ChatMessageRecord {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}

	cleanedChat := cleanJID(chatJID)
	var records []*ChatMessageRecord

	if ps.db != nil {
		rows, err := ps.db.Query(`
			SELECT id, chat_jid, sender_jid, sender_name, text, is_from_me, is_bot, timestamp
			FROM chat_messages
			WHERE (chat_jid = ? OR chat_jid = ?) AND is_bot = 0
			ORDER BY timestamp DESC
			LIMIT ?
		`, cleanedChat, chatJID, limit*2)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var r ChatMessageRecord
				var fromMe, isBot int
				if scanErr := rows.Scan(&r.ID, &r.ChatJID, &r.SenderJID, &r.SenderName, &r.Text, &fromMe, &isBot, &r.Timestamp); scanErr == nil {
					r.IsFromMe = (fromMe == 1)
					r.IsBot = (isBot == 1)
					records = append(records, &r)
				}
			}
		}
	}

	// Fallback to in-memory if DB is empty
	if len(records) == 0 {
		if list, ok := ps.chatHistory[cleanedChat]; ok {
			for i := len(list) - 1; i >= 0; i-- {
				records = append(records, list[i])
			}
		}
	}

	var filtered []*ChatMessageRecord
	for _, m := range records {
		if m == nil || m.ID == excludeID || m.IsBot {
			continue
		}
		text := strings.TrimSpace(m.Text)
		if text == "" {
			continue
		}
		// Skip administrative commands
		if strings.HasPrefix(text, "!") || strings.HasPrefix(text, ".") || strings.HasPrefix(text, "/") || strings.HasPrefix(text, "#") {
			continue
		}
		// Clean bot mention if message was a tag
		cleaned := text
		for _, tag := range []string{"@prism", "@bot"} {
			lower := strings.ToLower(cleaned)
			for idx := strings.Index(lower, tag); idx != -1; idx = strings.Index(lower, tag) {
				cleaned = strings.TrimSpace(cleaned[:idx] + cleaned[idx+len(tag):])
				lower = strings.ToLower(cleaned)
			}
		}
		cleaned = strings.TrimLeft(strings.TrimSpace(cleaned), ":,-? ")
		if cleaned == "" {
			continue
		}
		mCopy := *m
		mCopy.Text = cleaned
		filtered = append(filtered, &mCopy)
		if len(filtered) >= limit {
			break
		}
	}

	// Reverse to chronological order (oldest to newest)
	result := make([]*ChatMessageRecord, len(filtered))
	for i, j := 0, len(filtered)-1; i < len(filtered); i, j = i+1, j-1 {
		result[i] = filtered[j]
	}
	return result
}

// GetUserName returns the push name of the user (account owner) recorded from is_from_me messages.
func (ps *PersistentStore) GetUserName() string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if ps.db != nil {
		var name string
		err := ps.db.QueryRow(`
			SELECT sender_name 
			FROM chat_messages 
			WHERE is_from_me = 1 AND sender_name IS NOT NULL AND sender_name != '' AND sender_name != 'User'
			ORDER BY timestamp DESC 
			LIMIT 1
		`).Scan(&name)
		if err == nil && strings.TrimSpace(name) != "" {
			return strings.TrimSpace(name)
		}
	}
	return ""
}

// Package-level accessors
func SaveChatMessage(msg *ChatMessageRecord) error {
	if GlobalPersistent == nil {
		return nil
	}
	return GlobalPersistent.SaveChatMessage(msg)
}

func RecordBotMessage(id, chatJID, text string) error {
	if GlobalPersistent == nil {
		return nil
	}
	return GlobalPersistent.RecordBotMessage(id, chatJID, text)
}

func IsBotMessage(id string) bool {
	if GlobalPersistent == nil {
		return false
	}
	return GlobalPersistent.IsBotMessage(id)
}

func GetChatUserMessages(chatJID string, limit int, excludeID string) []*ChatMessageRecord {
	if GlobalPersistent == nil {
		return nil
	}
	return GlobalPersistent.GetChatUserMessages(chatJID, limit, excludeID)
}

func GetRecentChatHistoryPersistent(chatJID string, limit int, excludeID string) []*ChatMessageRecord {
	if GlobalPersistent == nil {
		return nil
	}
	return GlobalPersistent.GetRecentChatHistoryPersistent(chatJID, limit, excludeID)
}

func GetUserName() string {
	if GlobalPersistent == nil {
		return ""
	}
	return GlobalPersistent.GetUserName()
}

func (ps *PersistentStore) Close() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.db != nil {
		err := ps.db.Close()
		ps.db = nil
		return err
	}
	return nil
}
