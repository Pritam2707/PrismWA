package commands

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/polymorfa/hypermeow"
	waProto "github.com/polymorfa/hypermeow/binary/proto"
	"github.com/polymorfa/hypermeow/types"
	"google.golang.org/protobuf/proto"

	"whatsapp-bot/pkg/config"
)

// NoteEntry matches the structure stored in notes.json
type NoteEntry struct {
	Content   string `json:"content"`
	MediaType string `json:"mediaType"` // "TXT", "IMA", "VID", "AUD", "STK", "DOC", "PDF", "XLS", "PPT"
	Mimetype  string `json:"mimetype"`
	FileName  string `json:"fileName,omitempty"`
	MediaData string `json:"mediaData,omitempty"`
	IsPrivate bool   `json:"isPrivate"`
	ScopeJID  string `json:"scopeJid,omitempty"`
	Timestamp string `json:"timestamp"`
}

// IndexedItem maps a display number [ 1 ], [ 2 ] to its full path and type
type IndexedItem struct {
	Num      int
	Name     string
	IsFolder bool
	FullPath string
}

// NotesManager manages thread-safe access to notes.json
type NotesManager struct {
	mu       sync.RWMutex
	filePath string
	notes    map[string]*NoteEntry
}

var GlobalNotesManager = &NotesManager{
	filePath: "notes.json",
	notes:    make(map[string]*NoteEntry),
}

// Active working directory state per chat JID
var userCurrentPaths sync.Map // chatJID string -> string (e.g. "root/", "root/public/")

func init() {
	if err := GlobalNotesManager.Load(); err != nil {
		log.Printf("[Notes] WARN: Failed to load notes.json on startup: %v", err)
	}
}

func (m *NotesManager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	targetPath := m.filePath
	if _, err := os.Stat(targetPath); err != nil {
		// Try relative to executable
		if exe, exeErr := os.Executable(); exeErr == nil {
			cand := filepath.Join(filepath.Dir(exe), "notes.json")
			if _, statErr := os.Stat(cand); statErr == nil {
				targetPath = cand
				m.filePath = cand
			}
		}
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			m.notes = make(map[string]*NoteEntry)
			return nil
		}
		return err
	}

	var loaded map[string]*NoteEntry
	if err := json.Unmarshal(data, &loaded); err != nil {
		return fmt.Errorf("malformed notes.json: %w", err)
	}

	m.notes = loaded
	log.Printf("[Notes] Loaded %d notes from %s", len(m.notes), targetPath)
	return nil
}

func (m *NotesManager) Save() error {
	m.mu.RLock()
	data, err := json.MarshalIndent(m.notes, "", "  ")
	m.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("failed to marshal notes: %w", err)
	}

	tmpFile := fmt.Sprintf("%s.tmp-%d", m.filePath, time.Now().UnixNano())
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write tmp notes file: %w", err)
	}

	if err := os.Rename(tmpFile, m.filePath); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to atomically replace notes file: %w", err)
	}

	return nil
}

func cleanJID(jid string) string {
	clean := strings.TrimSpace(jid)
	if clean == "" {
		return ""
	}
	if atIdx := strings.Index(clean, "@"); atIdx != -1 {
		userPart := clean[:atIdx]
		serverPart := clean[atIdx:]
		if colonIdx := strings.Index(userPart, ":"); colonIdx != -1 {
			userPart = userPart[:colonIdx]
		}
		return userPart + serverPart
	}
	if colonIdx := strings.Index(clean, ":"); colonIdx != -1 {
		clean = clean[:colonIdx]
	}
	return clean
}

func isScopeMatch(scopeJID, chatJID string) bool {
	if scopeJID == "" || chatJID == "" {
		return false
	}
	return strings.EqualFold(cleanJID(scopeJID), cleanJID(chatJID))
}

func (m *NotesManager) GetAllAccessible(chatJID string, isOwner bool) map[string]*NoteEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	res := make(map[string]*NoteEntry, len(m.notes))
	for notePath, note := range m.notes {
		isPrivate := note.IsPrivate || strings.HasPrefix(notePath, "root/private/")

		if isPrivate {
			// Private notes are strictly bound to the specific group or DM where they were created.
			// They are never accessible in other groups or DMs.
			if isScopeMatch(note.ScopeJID, chatJID) {
				res[notePath] = note
			}
		} else {
			// Public notes are accessible across all chats
			res[notePath] = note
		}
	}
	return res
}

// GetAllNotesForDashboard returns all notes stripped of heavy binary mediaData for fast dashboard transmission
func (m *NotesManager) GetAllNotesForDashboard() map[string]map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	res := make(map[string]map[string]interface{}, len(m.notes))
	for path, note := range m.notes {
		hasMedia := len(note.MediaData) > 0
		res[path] = map[string]interface{}{
			"content":   note.Content,
			"mediaType": note.MediaType,
			"mimetype":  note.Mimetype,
			"fileName":  note.FileName,
			"isPrivate": note.IsPrivate,
			"scopeJid":  note.ScopeJID,
			"timestamp": note.Timestamp,
			"hasMedia":  hasMedia,
		}
	}
	return res
}

// AddNoteFromDashboard adds a note and persists it to notes.json
func (m *NotesManager) AddNoteFromDashboard(rawPath, content string) error {
	cleanPath := resolveNoteFullPath(rawPath, "root/public/")
	m.mu.Lock()
	m.notes[cleanPath] = &NoteEntry{
		Content:   content,
		MediaType: "TXT",
		Mimetype:  "text/plain",
		FileName:  filepath.Base(cleanPath),
		Timestamp: time.Now().Format(time.RFC3339),
	}
	m.mu.Unlock()
	return m.Save()
}

// DeleteNoteFromDashboard deletes a note or folder and persists changes
func (m *NotesManager) DeleteNoteFromDashboard(path string) error {
	m.mu.Lock()
	delete(m.notes, path)
	prefix := path
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	for k := range m.notes {
		if strings.HasPrefix(k, prefix) {
			delete(m.notes, k)
		}
	}
	m.mu.Unlock()
	return m.Save()
}

func getUserPath(chatJID string) string {
	val, ok := userCurrentPaths.Load(chatJID)
	if !ok || val == nil {
		return "root/"
	}
	p := val.(string)
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	if !strings.HasPrefix(p, "root/") {
		p = "root/"
	}
	return p
}

func setUserPath(chatJID string, p string) {
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	if !strings.HasPrefix(p, "root/") {
		p = "root/"
	}
	userCurrentPaths.Store(chatJID, p)
}

func isNoteAuthorizedOwner(c *CommandContext) bool {
	if c.RawMsg != nil && c.RawMsg.Info.IsFromMe {
		return true
	}
	if c.Client != nil && c.Client.Store != nil && c.Client.Store.ID != nil {
		botUser := c.Client.Store.ID.User
		if c.SenderJID.User == botUser {
			return true
		}
	}
	for _, owner := range config.AppConfig.OwnerJIDs {
		if strings.TrimSpace(owner) == "" {
			continue
		}
		if strings.Contains(owner, c.SenderJID.User) || strings.Contains(c.SenderJID.String(), owner) {
			return true
		}
	}
	return false
}

func resolveNoteFullPath(rawPath, currentPath string) string {
	cleaned := strings.Trim(rawPath, "/")
	for strings.Contains(cleaned, "//") {
		cleaned = strings.ReplaceAll(cleaned, "//", "/")
	}

	if strings.HasPrefix(cleaned, "root/") {
		return cleaned
	}
	if strings.HasPrefix(cleaned, "public/") || strings.HasPrefix(cleaned, "private/") {
		return "root/" + cleaned
	}
	if strings.HasPrefix(currentPath, "root/private") {
		p := currentPath + cleaned
		for strings.Contains(p, "//") {
			p = strings.ReplaceAll(p, "//", "/")
		}
		return p
	}
	if currentPath == "root/" {
		return "root/public/" + cleaned
	}

	p := currentPath + cleaned
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	return p
}

type treeNode struct {
	fileData *NoteEntry
	fullPath string
	children map[string]*treeNode
}

func buildTreeAndIndexMap(currentPath string, accessible map[string]*NoteEntry) (string, []IndexedItem) {
	normalizedCurrent := currentPath
	if !strings.HasSuffix(normalizedCurrent, "/") {
		normalizedCurrent += "/"
	}

	var outputLines []string
	var itemIndexMap []IndexedItem
	counter := 1

	if normalizedCurrent == "root/" {
		// Collect all top-level subdirectories from accessible notes
		dirSet := map[string]bool{"public": true, "private": true}
		for p := range accessible {
			if strings.HasPrefix(p, "root/") {
				rel := strings.TrimPrefix(p, "root/")
				parts := strings.Split(rel, "/")
				if len(parts) > 1 {
					dirSet[parts[0]] = true
				}
			}
		}

		dirs := make([]string, 0, len(dirSet))
		for d := range dirSet {
			dirs = append(dirs, d)
		}
		sort.Strings(dirs)

		for i, dir := range dirs {
			isLast := i == len(dirs)-1
			connector := "├── "
			if isLast {
				connector = "└── "
			}
			num := counter
			counter++
			outputLines = append(outputLines, fmt.Sprintf("%s📂 [ %d ] %s/", connector, num, dir))
			itemIndexMap = append(itemIndexMap, IndexedItem{
				Num:      num,
				Name:     dir,
				IsFolder: true,
				FullPath: fmt.Sprintf("root/%s/", dir),
			})
		}

		return strings.Join(outputLines, "\n"), itemIndexMap
	}

	// Filter notes belonging to current subtree
	relevantPaths := make([]string, 0)
	for p := range accessible {
		if strings.HasPrefix(p, normalizedCurrent) {
			relevantPaths = append(relevantPaths, p)
		}
	}

	if len(relevantPaths) == 0 {
		return "( empty directory )", itemIndexMap
	}

	// Group paths into a recursive tree structure
	rootNode := &treeNode{children: make(map[string]*treeNode)}

	for _, fullNotePath := range relevantPaths {
		rel := fullNotePath[len(normalizedCurrent):]
		parts := strings.Split(rel, "/")

		curr := rootNode
		for i, part := range parts {
			isLast := i == len(parts)-1
			if curr.children[part] == nil {
				curr.children[part] = &treeNode{children: make(map[string]*treeNode)}
			}
			if isLast {
				curr.children[part].fileData = accessible[fullNotePath]
				curr.children[part].fullPath = fullNotePath
			} else {
				curr = curr.children[part]
			}
		}
	}

	// Recursive traversal
	var traverse func(node *treeNode, prefix string, currFullPath string)
	traverse = func(node *treeNode, prefix string, currFullPath string) {
		keys := make([]string, 0, len(node.children))
		for k := range node.children {
			keys = append(keys, k)
		}

		sort.Slice(keys, func(i, j int) bool {
			aChild := node.children[keys[i]]
			bChild := node.children[keys[j]]
			aIsFile := aChild.fileData != nil
			bIsFile := bChild.fileData != nil
			if aIsFile == bIsFile {
				return keys[i] < keys[j]
			}
			return !aIsFile // folders first
		})

		for i, key := range keys {
			isLast := i == len(keys)-1
			connector := "├── "
			childPrefix := prefix + "│   "
			if isLast {
				connector = "└── "
				childPrefix = prefix + "    "
			}

			child := node.children[key]
			num := counter
			counter++

			if child.fileData != nil {
				mType := child.fileData.MediaType
				if mType == "" {
					mType = "TXT"
				}
				outputLines = append(outputLines, fmt.Sprintf("%s%s📄 [ %d ] %s (%s)", prefix, connector, num, key, mType))
				itemIndexMap = append(itemIndexMap, IndexedItem{
					Num:      num,
					Name:     key,
					IsFolder: false,
					FullPath: child.fullPath,
				})
			} else {
				folderFullPath := fmt.Sprintf("%s%s/", currFullPath, key)
				outputLines = append(outputLines, fmt.Sprintf("%s%s📂 [ %d ] %s/", prefix, connector, num, key))
				itemIndexMap = append(itemIndexMap, IndexedItem{
					Num:      num,
					Name:     key,
					IsFolder: true,
					FullPath: folderFullPath,
				})
				traverse(child, childPrefix, folderFullPath)
			}
		}
	}

	traverse(rootNode, "", normalizedCurrent)

	return strings.Join(outputLines, "\n"), itemIndexMap
}

func renderNestedDashboard(currentPath string, accessible map[string]*NoteEntry) (string, []IndexedItem) {
	treeText, itemIndexMap := buildTreeAndIndexMap(currentPath, accessible)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📁 *%s*\n==================================\n", currentPath))
	sb.WriteString(treeText + "\n")
	sb.WriteString("==================================\n")
	sb.WriteString("👉 *Open:* `!cd <num|name>` | *Back:* `!cd ..`\n")
	sb.WriteString("👉 *Get file:* `!getnote <num|path>`\n")
	sb.WriteString("❌ *Delete (Owner Only):* `!delnote <num|path>`")

	return sb.String(), itemIndexMap
}

// -----------------------------------------------------------------------------
// Command Handlers
// -----------------------------------------------------------------------------

var lastDashboardMsgID sync.Map // chatKey string -> types.MessageID

func sendOrEditDashboard(c *CommandContext, textContent string) error {
	chatKey := c.ChatJID.String()
	lastIDVal, exists := lastDashboardMsgID.Load(chatKey)

	if exists && lastIDVal != nil {
		lastID, ok := lastIDVal.(types.MessageID)
		if ok && lastID != "" {
			editMsg := c.Client.BuildEdit(c.ChatJID, lastID, &waProto.Message{
				Conversation: proto.String(textContent),
			})
			resp, err := c.Client.SendMessage(c.Ctx, c.ChatJID, editMsg)
			if err == nil && resp.ID != "" {
				return nil
			}
		}
	}

	// Fallback or first send: send new message
	msg := &waProto.Message{
		Conversation: proto.String(textContent),
	}
	resp, err := c.Client.SendMessage(c.Ctx, c.ChatJID, msg)
	if err == nil && resp.ID != "" {
		lastDashboardMsgID.Store(chatKey, resp.ID)
	}
	return err
}

func handleNotesCommand(c *CommandContext) (bool, error) {
	chatKey := c.ChatJID.String()
	isOwner := isNoteAuthorizedOwner(c)
	currentPath := "root/"
	setUserPath(chatKey, currentPath)

	accessible := GlobalNotesManager.GetAllAccessible(chatKey, isOwner)
	viewText, _ := renderNestedDashboard(currentPath, accessible)
	err := sendOrEditDashboard(c, viewText)
	return true, err
}

func handleCdCommand(c *CommandContext) (bool, error) {
	args := strings.Fields(strings.TrimSpace(c.RawText))
	if len(args) < 2 {
		return true, c.ReplyText("⚠️ Usage: `!cd <number | folder_name | ..>`")
	}

	target := args[1]
	chatKey := c.ChatJID.String()
	isOwner := isNoteAuthorizedOwner(c)
	currentPath := getUserPath(chatKey)
	accessible := GlobalNotesManager.GetAllAccessible(chatKey, isOwner)

	if target == ".." {
		if currentPath == "root/" {
			viewText, _ := renderNestedDashboard("root/", accessible)
			return true, c.ReplyText(viewText + "\n\n⚠️ *Already at root/*")
		}

		trimmed := strings.TrimSuffix(currentPath, "/")
		idx := strings.LastIndex(trimmed, "/")
		if idx <= 0 {
			currentPath = "root/"
		} else {
			currentPath = trimmed[:idx+1]
		}
		if !strings.HasPrefix(currentPath, "root/") {
			currentPath = "root/"
		}

		setUserPath(chatKey, currentPath)
		viewText, _ := renderNestedDashboard(currentPath, accessible)
		err := sendOrEditDashboard(c, viewText)
		return true, err
	}

	_, itemIndexMap := buildTreeAndIndexMap(currentPath, accessible)
	numIndex, errNum := strconv.Atoi(target)

	var selectedChild *IndexedItem
	for i := range itemIndexMap {
		item := &itemIndexMap[i]
		if errNum == nil && item.Num == numIndex {
			selectedChild = item
			break
		} else if strings.EqualFold(item.Name, target) {
			selectedChild = item
			break
		}
	}

	if selectedChild == nil {
		viewText, _ := renderNestedDashboard(currentPath, accessible)
		return true, c.ReplyText(fmt.Sprintf("%s\n\n❌ *Item \"%s\" not found!*", viewText, target))
	}

	if !selectedChild.IsFolder {
		viewText, _ := renderNestedDashboard(currentPath, accessible)
		return true, c.ReplyText(fmt.Sprintf("%s\n\n⚠️ *\"%s\" is a file. Use !getnote %s*", viewText, selectedChild.Name, selectedChild.Name))
	}

	currentPath = selectedChild.FullPath
	setUserPath(chatKey, currentPath)

	viewText, _ := renderNestedDashboard(currentPath, accessible)
	err := sendOrEditDashboard(c, viewText)
	return true, err
}

func handlePwdCommand(c *CommandContext) (bool, error) {
	currentPath := getUserPath(c.ChatJID.String())
	return true, c.ReplyText(fmt.Sprintf("📂 *Current directory:* `%s`", currentPath))
}

func handleGetNoteCommand(c *CommandContext) (bool, error) {
	args := strings.Fields(strings.TrimSpace(c.RawText))
	if len(args) < 2 {
		return true, c.ReplyText("⚠️ Usage: `!getnote <num | name | path>`")
	}

	target := args[1]
	chatKey := c.ChatJID.String()
	isOwner := isNoteAuthorizedOwner(c)
	currentPath := getUserPath(chatKey)
	accessible := GlobalNotesManager.GetAllAccessible(chatKey, isOwner)

	var targetPath string
	numIndex, errNum := strconv.Atoi(target)

	if errNum == nil {
		_, itemIndexMap := buildTreeAndIndexMap(currentPath, accessible)
		for i := range itemIndexMap {
			if itemIndexMap[i].Num == numIndex && !itemIndexMap[i].IsFolder {
				targetPath = itemIndexMap[i].FullPath
				break
			}
		}
	}

	if targetPath == "" {
		possibleRel := currentPath + target
		if accessible[possibleRel] != nil {
			targetPath = possibleRel
		}
	}

	if targetPath == "" {
		resolved := resolveNoteFullPath(target, currentPath)
		if accessible[resolved] != nil {
			targetPath = resolved
		}
	}

	if targetPath == "" || accessible[targetPath] == nil {
		return true, c.ReplyText(fmt.Sprintf("❌ Note `%s` not found or permission denied.", target))
	}

	note := accessible[targetPath]
	savedTime := note.Timestamp
	if t, parseErr := time.Parse(time.RFC3339, note.Timestamp); parseErr == nil {
		savedTime = t.Format("02/01/2006, 15:04:05")
	}

	caption := fmt.Sprintf("📄 *Note:* `%s`\n🏷️ *Type:* `%s`\n📅 *Saved:* %s\n\n%s",
		targetPath, note.MediaType, savedTime, note.Content)
	caption = strings.TrimSpace(caption)

	if note.MediaType == "TXT" || note.MediaData == "" {
		return true, c.ReplyText(caption)
	}

	mediaBytes, decErr := base64.StdEncoding.DecodeString(note.MediaData)
	if decErr != nil || len(mediaBytes) == 0 {
		return true, c.ReplyText(caption)
	}

	fallbackName := note.FileName
	if fallbackName == "" {
		fallbackName = path.Base(targetPath)
	}

	switch note.MediaType {
	case "IMA":
		uploaded, upErr := c.Client.Upload(context.Background(), mediaBytes, whatsmeow.MediaImage)
		if upErr != nil {
			return true, c.ReplyText(fmt.Sprintf("Failed to upload image: %v", upErr))
		}
		msg := &waProto.Message{
			ImageMessage: &waProto.ImageMessage{
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      proto.String(note.Mimetype),
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(mediaBytes))),
				Caption:       proto.String(caption),
			},
		}
		_, err := c.Client.SendMessage(c.Ctx, c.ChatJID, msg)
		return true, err

	case "VID":
		uploaded, upErr := c.Client.Upload(context.Background(), mediaBytes, whatsmeow.MediaVideo)
		if upErr != nil {
			return true, c.ReplyText(fmt.Sprintf("Failed to upload video: %v", upErr))
		}
		msg := &waProto.Message{
			VideoMessage: &waProto.VideoMessage{
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      proto.String(note.Mimetype),
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(mediaBytes))),
				Caption:       proto.String(caption),
			},
		}
		_, err := c.Client.SendMessage(c.Ctx, c.ChatJID, msg)
		return true, err

	case "AUD":
		uploaded, upErr := c.Client.Upload(context.Background(), mediaBytes, whatsmeow.MediaAudio)
		if upErr != nil {
			return true, c.ReplyText(fmt.Sprintf("Failed to upload audio: %v", upErr))
		}
		mime := note.Mimetype
		if mime == "" {
			mime = "audio/mp4"
		}
		msg := &waProto.Message{
			AudioMessage: &waProto.AudioMessage{
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      proto.String(mime),
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(mediaBytes))),
			},
		}
		_, err := c.Client.SendMessage(c.Ctx, c.ChatJID, msg)
		return true, err

	default: // DOC, PDF, XLS, PPT
		uploaded, upErr := c.Client.Upload(context.Background(), mediaBytes, whatsmeow.MediaDocument)
		if upErr != nil {
			return true, c.ReplyText(fmt.Sprintf("Failed to upload document: %v", upErr))
		}
		mime := note.Mimetype
		if mime == "" {
			mime = "application/octet-stream"
		}
		msg := &waProto.Message{
			DocumentMessage: &waProto.DocumentMessage{
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      proto.String(mime),
				FileName:      proto.String(fallbackName),
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(mediaBytes))),
				Caption:       proto.String(caption),
			},
		}
		_, err := c.Client.SendMessage(c.Ctx, c.ChatJID, msg)
		return true, err
	}
}

func handleAddNoteCommand(c *CommandContext) (bool, error) {
	raw := strings.TrimSpace(c.RawText)
	args := strings.Fields(raw)

	// If invoked via !note add <path> <text>
	if len(args) > 2 && strings.EqualFold(args[1], "add") {
		args = append([]string{args[0]}, args[2:]...)
	}

	// Support !note private <path> <text> or !note -p <path> <text>
	isExplicitPrivate := false
	if len(args) > 2 && (strings.EqualFold(args[1], "private") || args[1] == "-p") {
		isExplicitPrivate = true
		args = append([]string{args[0]}, args[2:]...)
	}

	if len(args) < 2 {
		return true, c.ReplyText("⚠️ Usage:\n• `!addnote <path> <text>`\n• `!addnote private/<path> <text>` (Private note bound to this chat)")
	}

	notePathRaw := args[1]
	contentText := strings.TrimSpace(strings.Join(args[2:], " "))

	if isExplicitPrivate && !strings.HasPrefix(notePathRaw, "private/") && !strings.HasPrefix(notePathRaw, "root/private/") {
		notePathRaw = "private/" + notePathRaw
	}

	chatKey := c.ChatJID.String()
	isOwner := isNoteAuthorizedOwner(c)
	currentPath := getUserPath(chatKey)
	fullPath := resolveNoteFullPath(notePathRaw, currentPath)
	isPrivate := strings.HasPrefix(fullPath, "root/private/") || isExplicitPrivate

	// Extract attached or quoted media
	mediaInfo := extractNoteMediaInfo(c)
	if contentText == "" && mediaInfo.QuotedText != "" {
		contentText = mediaInfo.QuotedText
	}

	var base64Media string
	if mediaInfo.DownloadTarget != nil {
		data, err := c.Client.Download(c.Ctx, mediaInfo.DownloadTarget)
		if err == nil && len(data) > 0 {
			base64Media = base64.StdEncoding.EncodeToString(data)
		} else {
			log.Printf("[Notes] WARN: Failed to download note media: %v", err)
		}
	}

	entry := &NoteEntry{
		Content:   contentText,
		MediaType: mediaInfo.Type,
		Mimetype:  mediaInfo.Mimetype,
		FileName:  mediaInfo.FileName,
		MediaData: base64Media,
		IsPrivate: isPrivate,
		ScopeJID:  chatKey,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	if entry.FileName == "" {
		entry.FileName = path.Base(fullPath)
	}

	GlobalNotesManager.mu.Lock()
	GlobalNotesManager.notes[fullPath] = entry
	GlobalNotesManager.mu.Unlock()

	if err := GlobalNotesManager.Save(); err != nil {
		return true, c.ReplyText(fmt.Sprintf("Failed to save note to disk: %v", err))
	}

	accessible := GlobalNotesManager.GetAllAccessible(chatKey, isOwner)
	viewText, _ := renderNestedDashboard(currentPath, accessible)
	return true, c.ReplyText(fmt.Sprintf("%s\n\n✅ *Note saved:* `%s`", viewText, fullPath))
}

func handleDelNoteCommand(c *CommandContext) (bool, error) {
	if !isNoteAuthorizedOwner(c) {
		return true, c.ReplyText("⛔ *Access Denied:* Only the bot owner/host is permitted to delete notes or directories.")
	}

	args := strings.Fields(strings.TrimSpace(c.RawText))
	if len(args) < 2 {
		return true, c.ReplyText("⚠️ Usage: `!delnote <num | name | path>`")
	}

	target := args[1]
	chatKey := c.ChatJID.String()
	isOwner := true
	currentPath := getUserPath(chatKey)
	accessible := GlobalNotesManager.GetAllAccessible(chatKey, isOwner)

	var targetPath string
	var isFolder bool

	numIndex, errNum := strconv.Atoi(target)
	if errNum == nil {
		_, itemIndexMap := buildTreeAndIndexMap(currentPath, accessible)
		for i := range itemIndexMap {
			if itemIndexMap[i].Num == numIndex {
				targetPath = itemIndexMap[i].FullPath
				isFolder = itemIndexMap[i].IsFolder
				break
			}
		}
	}

	if targetPath == "" {
		possibleRel := currentPath + target
		if accessible[possibleRel] != nil {
			targetPath = possibleRel
		} else {
			asFolder := possibleRel
			if !strings.HasSuffix(asFolder, "/") {
				asFolder += "/"
			}
			for p := range accessible {
				if strings.HasPrefix(p, asFolder) {
					targetPath = asFolder
					isFolder = true
					break
				}
			}
		}
	}

	if targetPath == "" {
		resolved := resolveNoteFullPath(target, currentPath)
		if accessible[resolved] != nil {
			targetPath = resolved
		} else {
			asFolder := resolved
			if !strings.HasSuffix(asFolder, "/") {
				asFolder += "/"
			}
			for p := range accessible {
				if strings.HasPrefix(p, asFolder) {
					targetPath = asFolder
					isFolder = true
					break
				}
			}
		}
	}

	if targetPath == "" {
		return true, c.ReplyText(fmt.Sprintf("❌ Note or directory `%s` not found.", target))
	}

	if targetPath == "root/" || targetPath == "root/public/" || targetPath == "root/private/" {
		return true, c.ReplyText("⚠️ *Cannot delete root directories!*")
	}

	GlobalNotesManager.mu.Lock()
	deletedCount := 0
	if isFolder {
		folderPrefix := targetPath
		if !strings.HasSuffix(folderPrefix, "/") {
			folderPrefix += "/"
		}
		for k := range GlobalNotesManager.notes {
			if strings.HasPrefix(k, folderPrefix) {
				delete(GlobalNotesManager.notes, k)
				deletedCount++
			}
		}
	} else {
		delete(GlobalNotesManager.notes, targetPath)
	}
	GlobalNotesManager.mu.Unlock()

	_ = GlobalNotesManager.Save()

	updatedAccessible := GlobalNotesManager.GetAllAccessible(chatKey, isOwner)
	viewText, _ := renderNestedDashboard(currentPath, updatedAccessible)

	if isFolder {
		return true, c.ReplyText(fmt.Sprintf("%s\n\n🗑️ *Deleted folder `%s` (%d notes removed)*", viewText, targetPath, deletedCount))
	}
	return true, c.ReplyText(fmt.Sprintf("%s\n\n🗑️ *Deleted note:* `%s`", viewText, targetPath))
}

// -----------------------------------------------------------------------------
// Media Extraction Helper
// -----------------------------------------------------------------------------

type extractedNoteMedia struct {
	Type           string
	Mimetype       string
	FileName       string
	QuotedText     string
	DownloadTarget whatsmeow.DownloadableMessage
}

func extractNoteMediaInfo(c *CommandContext) extractedNoteMedia {
	res := extractedNoteMedia{
		Type:     "TXT",
		Mimetype: "text/plain",
	}

	if c.RawMsg == nil || c.RawMsg.Message == nil {
		return res
	}

	raw := c.RawMsg.Message
	var contextInfo *waProto.ContextInfo
	if ext := raw.GetExtendedTextMessage(); ext != nil {
		contextInfo = ext.GetContextInfo()
	} else if img := raw.GetImageMessage(); img != nil {
		contextInfo = img.GetContextInfo()
	} else if doc := raw.GetDocumentMessage(); doc != nil {
		contextInfo = doc.GetContextInfo()
	} else if aud := raw.GetAudioMessage(); aud != nil {
		contextInfo = aud.GetContextInfo()
	}

	targetMsg := raw
	if contextInfo != nil && contextInfo.GetQuotedMessage() != nil {
		targetMsg = contextInfo.GetQuotedMessage()
		if targetMsg.Conversation != nil {
			res.QuotedText = *targetMsg.Conversation
		} else if targetMsg.ExtendedTextMessage != nil && targetMsg.ExtendedTextMessage.Text != nil {
			res.QuotedText = *targetMsg.ExtendedTextMessage.Text
		}
	}

	if img := targetMsg.GetImageMessage(); img != nil {
		res.Type = "IMA"
		res.Mimetype = img.GetMimetype()
		if res.Mimetype == "" {
			res.Mimetype = "image/jpeg"
		}
		res.DownloadTarget = img
		return res
	}

	if vid := targetMsg.GetVideoMessage(); vid != nil {
		res.Type = "VID"
		res.Mimetype = vid.GetMimetype()
		if res.Mimetype == "" {
			res.Mimetype = "video/mp4"
		}
		res.DownloadTarget = vid
		return res
	}

	if aud := targetMsg.GetAudioMessage(); aud != nil {
		res.Type = "AUD"
		res.Mimetype = aud.GetMimetype()
		if res.Mimetype == "" {
			res.Mimetype = "audio/mp4"
		}
		res.DownloadTarget = aud
		return res
	}

	if doc := targetMsg.GetDocumentMessage(); doc != nil {
		fName := doc.GetFileName()
		if fName == "" {
			fName = "document"
		}
		res.FileName = fName
		ext := strings.ToLower(filepath.Ext(fName))
		mime := strings.ToLower(doc.GetMimetype())
		res.Mimetype = mime

		switch {
		case ext == ".xls" || ext == ".xlsx" || ext == ".csv" || strings.Contains(mime, "excel") || strings.Contains(mime, "spreadsheetml") || strings.Contains(mime, "csv"):
			res.Type = "XLS"
		case ext == ".ppt" || ext == ".pptx" || strings.Contains(mime, "powerpoint") || strings.Contains(mime, "presentationml"):
			res.Type = "PPT"
		case ext == ".pdf" || strings.Contains(mime, "pdf"):
			res.Type = "PDF"
		case ext == ".doc" || ext == ".docx" || strings.Contains(mime, "word") || strings.Contains(mime, "wordprocessingml"):
			res.Type = "DOC"
		default:
			res.Type = "DOC"
		}
		res.DownloadTarget = doc
		return res
	}

	return res
}
