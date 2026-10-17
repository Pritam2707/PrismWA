package dashboard

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/polymorfa/hypermeow/types"
	waProto "github.com/polymorfa/hypermeow/binary/proto"
	"google.golang.org/protobuf/proto"

	"whatsapp-bot/pkg/commands"
	"whatsapp-bot/pkg/config"
	"whatsapp-bot/pkg/gemini"
	"whatsapp-bot/pkg/meowcaller"
	"whatsapp-bot/pkg/pfp"
	"whatsapp-bot/pkg/pool"
	"whatsapp-bot/pkg/store"
	"whatsapp-bot/pkg/tts"
	"whatsapp-bot/pkg/whatsapp"
)

type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
}

var (
	logsMu       sync.RWMutex
	recentLogs   = make([]LogEntry, 0, 200)
	startEpoch   = time.Now()
	activeServer *http.Server
)

func LogEvent(level, message string) {
	logsMu.Lock()
	defer logsMu.Unlock()

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
	}
	recentLogs = append(recentLogs, entry)
	if len(recentLogs) > 200 {
		recentLogs = recentLogs[1:]
	}
}

func StartDashboardServer(port string) error {
	if port == "" {
		port = "3000"
	}

	mux := http.NewServeMux()

	publicDir := filepath.Join("dashboard", "public")
	fs := http.FileServer(http.Dir(publicDir))
	mux.Handle("/", fs)

	_ = os.MkdirAll(filepath.Join("data", "media"), 0755)
	_ = os.MkdirAll("stickers", 0755)
	mux.Handle("/data/media/", http.StripPrefix("/data/media/", http.FileServer(http.Dir(filepath.Join("data", "media")))))
	mux.Handle("/stickers/", http.StripPrefix("/stickers/", http.FileServer(http.Dir("stickers"))))

	// Status & Telemetry
	mux.HandleFunc("/api/status", handleStatus)
	mux.HandleFunc("/api/control/toggle-sleep", handleToggleSleep)
	mux.HandleFunc("/api/control/toggle-antidelete", handleToggleAntiDelete)
	mux.HandleFunc("/api/control/toggle-features", handleToggleFeatures)

	// Chats & Contacts
	mux.HandleFunc("/api/chats", handleChats)
	mux.HandleFunc("/api/contacts", handleContacts)
	mux.HandleFunc("/api/contacts/rename", handleRenameContact)

	// Stats & Analytics
	mux.HandleFunc("/api/stats", handleStats)
	mux.HandleFunc("/api/stats/", handleStats)

	// Storage & Disk Cleaner
	mux.HandleFunc("/api/storage", handleStorage)
	mux.HandleFunc("/api/storage/clean", handleStorageClean)

	// Notes
	mux.HandleFunc("/api/notes", handleNotes)

	// Reminders
	mux.HandleFunc("/api/reminders", handleReminders)
	mux.HandleFunc("/api/reminders/", handleReminders)

	// AI Personas & Interactive Test Playground
	mux.HandleFunc("/api/personas", handlePersonas)
	mux.HandleFunc("/api/personas/test", handlePersonasTest)

	// AI Provider & Modes Management
	mux.HandleFunc("/api/ai/config", handleAIConfig)
	mux.HandleFunc("/api/ai/provider", handleAISetProvider)
	mux.HandleFunc("/api/ai/model", handleAISetModel)
	mux.HandleFunc("/api/ai/style", handleAISetStyle)
	mux.HandleFunc("/api/ai/chat-override", handleAIChatOverride)

	// Welcome Studio
	mux.HandleFunc("/api/welcome", handleWelcome)

	// Moderation & Mutes
	mux.HandleFunc("/api/moderation/mutes", handleModerationMutes)
	mux.HandleFunc("/api/moderation/mute", handleModerationMute)
	mux.HandleFunc("/api/moderation/mute/", handleModerationMute)

	// Messaging & Broadcast
	mux.HandleFunc("/api/message/send", handleSendMessage)
	mux.HandleFunc("/api/message/broadcast", handleBroadcastMessage)

	// Auto-Reply
	mux.HandleFunc("/api/autoreply", handleAutoReply)
	mux.HandleFunc("/api/autoreply/flush", handleAutoReplyFlush)

	// Audit Vault & Deleted Messages
	mux.HandleFunc("/api/deleted-messages", handleDeletedMessages)
	mux.HandleFunc("/api/audit/deleted", handleAuditDeleted)
	mux.HandleFunc("/api/audit/deleted/", handleAuditDeleted)

	// Logs & Console
	mux.HandleFunc("/api/logs", handleLogs)

	// Status Reports & Streaks
	mux.HandleFunc("/api/reports", handleReports)
	mux.HandleFunc("/api/reports/groups", handleReportGroups)
	mux.HandleFunc("/api/reports/leaderboard", handleReportsLeaderboard)
	mux.HandleFunc("/api/reports/generate", handleReportsGenerate)

	// Group Deadlines & Aesthetic PFP
	mux.HandleFunc("/api/deadlines", handleDeadlines)
	mux.HandleFunc("/api/deadlines/preview", handleDeadlinesPreview)
	mux.HandleFunc("/api/deadlines/updatepfp", handleDeadlinesUpdatePFP)

	// MeowCaller VoIP & Song Party
	mux.HandleFunc("/api/meowcaller/status", handleMeowCallerStatus)
	mux.HandleFunc("/api/meowcaller/toggle", handleMeowCallerToggle)
	mux.HandleFunc("/api/meowcaller/auto-accept", handleMeowCallerAutoAccept)
	mux.HandleFunc("/api/meowcaller/call", handleMeowCallerCall)
	mux.HandleFunc("/api/meowcaller/answer", handleMeowCallerAnswer)
	mux.HandleFunc("/api/meowcaller/reject", handleMeowCallerReject)
	mux.HandleFunc("/api/meowcaller/hangup", handleMeowCallerHangup)
	mux.HandleFunc("/api/meowcaller/party/play", handleMeowCallerPartyPlay)
	mux.HandleFunc("/api/meowcaller/party/control", handleMeowCallerPartyControl)
	mux.HandleFunc("/api/meowcaller/party/queue/remove", handleMeowCallerPartyQueueRemove)
	mux.HandleFunc("/api/meowcaller/audio/ws", handleMeowCallerAudioWS)
	mux.HandleFunc("/api/meowcaller/audio/stream", handleMeowCallerAudioStream)
	mux.HandleFunc("/api/meowcaller/audio/listen.wav", handleMeowCallerAudioStream)
	mux.HandleFunc("/api/meowcaller/audio/status", handleMeowCallerAudioStatus)
	mux.HandleFunc("/api/meowcaller/radio/config", handleMeowCallerRadioConfig)
	mux.HandleFunc("/api/meowcaller/radio/status", handleMeowCallerRadioStatus)
	mux.HandleFunc("/api/meowcaller/voicebot/config", handleMeowCallerVoiceBotConfig)
	mux.HandleFunc("/api/meowcaller/voicebot/voices", handleMeowCallerVoiceBotVoices)
	mux.HandleFunc("/api/meowcaller/voicebot/connect", handleMeowCallerVoiceBotConnect)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	activeServer = server

	go func() {
		log.Printf("[Dashboard] Web server listening on http://localhost:%s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[Dashboard] Server error: %v", err)
		}
	}()

	return nil
}

func StopDashboardServer(ctx context.Context) error {
	if activeServer != nil {
		return activeServer.Shutdown(ctx)
	}
	return nil
}

// -----------------------------------------------------------------------------
// Handlers
// -----------------------------------------------------------------------------

func handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	uptimeSecs := int64(time.Since(startEpoch).Seconds())
	allocMB := float64(m.Alloc) / (1024 * 1024)
	sysMB := float64(m.Sys) / (1024 * 1024)

	connState := "disconnected"
	qrCode := ""
	var userObj interface{} = nil

	if whatsapp.GlobalWA != nil {
		connState = whatsapp.GlobalWA.GetConnectionState()
		qrCode = whatsapp.GlobalWA.GetQRCode()
		if whatsapp.GlobalWA.Client != nil && whatsapp.GlobalWA.Client.Store.ID != nil {
			id := whatsapp.GlobalWA.Client.Store.ID.String()
			userObj = map[string]string{
				"id":    id,
				"name":  "PrismBot",
				"phone": strings.Split(id, "@")[0],
			}
		}
	}

	resp := map[string]interface{}{
		"status": "ok",
		"bot": map[string]interface{}{
			"isSleeping":           store.GlobalStore.IsSleeping(),
			"connectionState":      connState,
			"qrCode":               qrCode,
			"user":                 userObj,
			"uptimeSecs":           uptimeSecs,
			"totalCachedMessages":  store.GlobalStore.Count(),
			"antiDeleteChatsCount": store.GlobalStore.AntiDeleteCount(),
			"featureChatsCount":    store.GlobalStore.FeatureCount(),
			"knownContactsCount":   store.GlobalStore.ContactsCount(),
			"activeWorkers":        pool.GlobalPool.ActiveWorkers(),
			"totalProcessed":       pool.GlobalPool.TotalProcessed(),
			"meowcaller":           meowcaller.GlobalManager.GetStatus(),
		},
		"system": map[string]interface{}{
			"platform":    runtime.GOOS,
			"arch":        runtime.GOARCH,
			"nodeVersion": fmt.Sprintf("go-%s", runtime.Version()),
			"cpuCount":    runtime.NumCPU(),
			"allocMB":     fmt.Sprintf("%.1f", allocMB),
			"sysMB":       fmt.Sprintf("%.1f", sysMB),
			"heapUsedMB":  fmt.Sprintf("%.1f", allocMB),
			"rssMB":       fmt.Sprintf("%.1f", sysMB),
			"goroutines":  runtime.NumGoroutine(),
		},
	}

	_ = json.NewEncoder(w).Encode(resp)
}

func handleToggleSleep(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	currentlySleeping := store.GlobalStore.IsSleeping()
	store.GlobalStore.SetSleeping(!currentlySleeping)

	LogEvent("SYSTEM", fmt.Sprintf("Sleep mode toggled to %v via dashboard", !currentlySleeping))

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"isSleeping": !currentlySleeping,
	})
}

func handleToggleAntiDelete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		JID    string `json:"jid"`
		Enable bool   `json:"enable"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.JID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	store.GlobalStore.ToggleAntiDelete(body.JID, body.Enable)
	LogEvent("SYSTEM", fmt.Sprintf("Anti-delete for %s set to %v via dashboard", body.JID, body.Enable))

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

func handleToggleFeatures(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		JID    string `json:"jid"`
		Enable bool   `json:"enable"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.JID == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	store.GlobalStore.ToggleFeatures(body.JID, body.Enable)
	LogEvent("SYSTEM", fmt.Sprintf("Features for %s set to %v via dashboard", body.JID, body.Enable))

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

func handleChats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	chatMap := make(map[string]map[string]interface{})
	contacts := store.GlobalStore.GetAllContacts()
	antiDeleteMap := store.GlobalStore.GetAllAntiDeleteChats()
	featuresMap := store.GlobalStore.GetAllFeatureChats()
	stats := commands.GetAllChatStats()

	// 1. Ingest from contacts
	for jid, name := range contacts {
		isGroup := strings.HasSuffix(jid, "@g.us")
		chatMap[jid] = map[string]interface{}{
			"jid":             jid,
			"name":            name,
			"phone":           strings.Split(jid, "@")[0],
			"isGroup":         isGroup,
			"antiDelete":      antiDeleteMap[jid],
			"featuresEnabled": !isGroup || featuresMap[jid],
			"totalMessages":   0,
		}
	}

	// 2. Ingest from stats
	for jid, userMap := range stats {
		tot := 0
		for _, u := range userMap {
			tot += u.Count
		}
		entry, exists := chatMap[jid]
		if !exists {
			name := store.GlobalStore.GetContact(jid)
			if name == "" {
				name = strings.Split(jid, "@")[0]
			}
			isGroup := strings.HasSuffix(jid, "@g.us")
			entry = map[string]interface{}{
				"jid":             jid,
				"name":            name,
				"phone":           strings.Split(jid, "@")[0],
				"isGroup":         isGroup,
				"antiDelete":      antiDeleteMap[jid],
				"featuresEnabled": !isGroup || featuresMap[jid],
			}
			chatMap[jid] = entry
		}
		entry["totalMessages"] = tot
	}

	// Convert to list sorted by activity
	list := make([]map[string]interface{}, 0, len(chatMap))
	for _, item := range chatMap {
		list = append(list, item)
	}

	sort.Slice(list, func(i, j int) bool {
		ti := list[i]["totalMessages"].(int)
		tj := list[j]["totalMessages"].(int)
		return ti > tj
	})

	_ = json.NewEncoder(w).Encode(list)
}

func handleContacts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	contacts := store.GlobalStore.GetAllContacts()
	list := make([]map[string]interface{}, 0, len(contacts))
	for jid, name := range contacts {
		list = append(list, map[string]interface{}{
			"jid":     jid,
			"name":    name,
			"phone":   strings.Split(jid, "@")[0],
			"isGroup": strings.HasSuffix(jid, "@g.us"),
		})
	}
	_ = json.NewEncoder(w).Encode(list)
}

func handleRenameContact(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		JID  string `json:"jid"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.JID == "" || body.Name == "" {
		http.Error(w, "Missing JID or name", http.StatusBadRequest)
		return
	}

	store.GlobalStore.SetContact(body.JID, strings.TrimSpace(body.Name))
	LogEvent("SUCCESS", fmt.Sprintf("Renamed contact %s to '%s' via dashboard", body.JID, body.Name))

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"jid":     body.JID,
		"name":    body.Name,
	})
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodDelete {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 3 {
			jid := parts[2]
			commands.ResetChatStatsForJID(jid)
			LogEvent("SYSTEM", fmt.Sprintf("Reset stats for %s via dashboard", jid))
		}
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
		return
	}

	statsData := commands.GetAllChatStats()
	enriched := make(map[string]map[string]interface{})

	for jid, userMap := range statsData {
		resolvedUsers := make(map[string]map[string]interface{})
		tot := 0
		for uid, stat := range userMap {
			uName := store.GlobalStore.GetContact(uid)
			if uName == "" {
				uName = strings.Split(uid, "@")[0]
			}
			resolvedUsers[uid] = map[string]interface{}{
				"count": stat.Count,
				"name":  uName,
				"phone": strings.Split(uid, "@")[0],
			}
			tot += stat.Count
		}

		cName := store.GlobalStore.GetContact(jid)
		if cName == "" {
			cName = strings.Split(jid, "@")[0]
		}

		enriched[jid] = map[string]interface{}{
			"chatName":      cName,
			"isGroup":       strings.HasSuffix(jid, "@g.us"),
			"phone":         strings.Split(jid, "@")[0],
			"totalMessages": tot,
			"users":         resolvedUsers,
		}
	}

	_ = json.NewEncoder(w).Encode(enriched)
}

func handleStorage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	mediaSize := getDirSize(filepath.Join("data", "media"))
	stickersSize := getDirSize("stickers")
	deletedMsgsSize := getFileSize(filepath.Join("data", "deleted_messages.json"))
	sqliteSize := getFileSize(filepath.Join("data", "prismbot.db"))
	notesSize := getFileSize("notes.json")
	if notesSize == 0 {
		notesSize = getFileSize(filepath.Join("data", "notes.json"))
	}
	statsSize := getFileSize(filepath.Join("data", "chat_stats.json"))
	total := mediaSize + stickersSize + deletedMsgsSize + sqliteSize + notesSize + statsSize

	resp := map[string]interface{}{
		"media":       map[string]interface{}{"size": mediaSize, "formatted": formatBytes(mediaSize)},
		"stickers":    map[string]interface{}{"size": stickersSize, "formatted": formatBytes(stickersSize)},
		"deletedMsgs": map[string]interface{}{"size": deletedMsgsSize, "formatted": formatBytes(deletedMsgsSize)},
		"sqlite":      map[string]interface{}{"size": sqliteSize, "formatted": formatBytes(sqliteSize)},
		"notes":       map[string]interface{}{"size": notesSize, "formatted": formatBytes(notesSize)},
		"stats":       map[string]interface{}{"size": statsSize, "formatted": formatBytes(statsSize)},
		"total":       map[string]interface{}{"size": total, "formatted": formatBytes(total)},
	}

	_ = json.NewEncoder(w).Encode(resp)
}

func handleStorageClean(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Target string `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Target == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	switch body.Target {
	case "media":
		_ = os.RemoveAll(filepath.Join("data", "media"))
		_ = os.MkdirAll(filepath.Join("data", "media"), 0755)
		LogEvent("SYSTEM", "Cleaned media cache via dashboard")
	case "stickers":
		_ = os.RemoveAll("stickers")
		_ = os.MkdirAll("stickers", 0755)
		LogEvent("SYSTEM", "Cleaned stickers cache via dashboard")
	case "deleted":
		_ = os.WriteFile(filepath.Join("data", "deleted_messages.json"), []byte("{}"), 0644)
		LogEvent("SYSTEM", "Cleaned deleted messages audit log via dashboard")
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"target":  body.Target,
	})
}

func handleNotes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		notes := commands.GlobalNotesManager.GetAllNotesForDashboard()
		_ = json.NewEncoder(w).Encode(notes)

	case http.MethodPost:
		var body struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Path == "" {
			http.Error(w, "Missing path", http.StatusBadRequest)
			return
		}

		if err := commands.GlobalNotesManager.AddNoteFromDashboard(body.Path, body.Content); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		LogEvent("SUCCESS", fmt.Sprintf("Saved note '%s' via dashboard", body.Path))
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})

	case http.MethodDelete:
		var body struct {
			Path string `json:"path"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Path == "" {
			http.Error(w, "Missing path", http.StatusBadRequest)
			return
		}

		if err := commands.GlobalNotesManager.DeleteNoteFromDashboard(body.Path); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		LogEvent("SYSTEM", fmt.Sprintf("Removed note '%s' via dashboard", body.Path))
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}
}

func handleReminders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		reminders := commands.GetAllReminders()
		enriched := make([]map[string]interface{}, 0, len(reminders))
		for _, rem := range reminders {
			cName := store.GlobalStore.GetContact(rem.ChatJID)
			if cName == "" {
				cName = strings.Split(rem.ChatJID, "@")[0]
			}
			sName := store.GlobalStore.GetContact(rem.SenderJID)
			if sName == "" {
				sName = strings.Split(rem.SenderJID, "@")[0]
			}

			enriched = append(enriched, map[string]interface{}{
				"id":         rem.ID,
				"jid":        rem.ChatJID,
				"senderId":   rem.SenderJID,
				"chatName":   cName,
				"senderName": sName,
				"text":       rem.Message,
				"targetTime": rem.TriggerAt.Unix() * 1000,
			})
		}
		_ = json.NewEncoder(w).Encode(enriched)

	case http.MethodPost:
		var body struct {
			JID     string `json:"jid"`
			Text    string `json:"text"`
			Minutes int    `json:"minutes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.JID == "" || body.Text == "" {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		if body.Minutes <= 0 {
			body.Minutes = 5
		}

		var cli = whatsapp.GlobalWA.Client
		targetTime := time.Now().Add(time.Duration(body.Minutes) * time.Minute)
		item := commands.CreateReminderFromDashboard(body.JID, body.JID, body.Text, targetTime, cli)

		LogEvent("SUCCESS", fmt.Sprintf("Scheduled reminder for %s in %dm via dashboard", body.JID, body.Minutes))
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "item": item})

	case http.MethodDelete:
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 3 {
			id := parts[2]
			commands.CancelReminderByID(id)
			LogEvent("SYSTEM", fmt.Sprintf("Cancelled reminder %s via dashboard", id))
		}
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}
}

func handlePersonas(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodPost {
		var body struct {
			Key    string `json:"key"`
			Label  string `json:"label"`
			Flavor string `json:"flavor"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Key == "" {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		cleanKey := strings.ToLower(strings.TrimPrefix(body.Key, "!"))
		label := body.Label
		if label == "" {
			label = strings.Title(cleanKey)
		}
		config.AppConfig.Personas[cleanKey] = config.PersonaConfig{
			Name:   label,
			Prefix: "!" + cleanKey,
			Prompt: body.Flavor,
		}
		LogEvent("SUCCESS", fmt.Sprintf("Added persona '%s' via dashboard", cleanKey))
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
		return
	}

	personas := make(map[string]map[string]interface{})
	for k, p := range config.AppConfig.Personas {
		personas[k] = map[string]interface{}{
			"key":      k,
			"name":     p.Name,
			"label":    p.Name,
			"prefix":   p.Prefix,
			"prompt":   p.Prompt,
			"flavor":   p.Prompt,
			"stickers": map[string]string{},
		}
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"personas": personas,
	})
}

func handlePersonasTest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		PersonaKey string `json:"personaKey"`
		Prompt     string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Prompt == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if gemini.GlobalClient == nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Gemini AI client is not initialized. Please verify GEMINI_API_KEY in .env file.",
		})
		return
	}

	instruction := config.AppConfig.BaseInstructionFormal
	if gemini.GlobalClient != nil && gemini.GlobalClient.GetGlobalStyle() == "genz" {
		instruction = config.AppConfig.BaseInstructionGenZ
	} else if gemini.GlobalClient != nil && gemini.GlobalClient.GetGlobalStyle() == "self" {
		instruction = config.AppConfig.BaseInstructionSelf
	}
	if cfg, ok := config.AppConfig.Personas[body.PersonaKey]; ok {
		instruction = cfg.Prompt
	}

	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()

	testChatKey := "dash_test_" + body.PersonaKey
	gemini.GlobalClient.ClearHistory(testChatKey)

	reply, err := gemini.GlobalClient.Chat(ctx, testChatKey, body.Prompt, instruction)
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"answer":  reply,
		"mood":    "neutral",
	})
}

func handleAIConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if gemini.GlobalClient == nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "AI service not initialized",
		})
		return
	}

	activeProvider := gemini.GlobalClient.GetProvider()
	activeModel := gemini.GlobalClient.GetActiveModel()
	status := gemini.GlobalClient.GetProviderStatus()

	overrides := gemini.GlobalClient.GetAllChatOverrides()
	type ChatOverrideItem struct {
		ChatJID  string `json:"chatJid"`
		ChatName string `json:"chatName"`
		Provider string `json:"provider"`
		Model    string `json:"model"`
		Style    string `json:"style,omitempty"`
	}

	overrideList := make([]ChatOverrideItem, 0, len(overrides))
	for jid, s := range overrides {
		cName := store.GlobalStore.GetContact(jid)
		if cName == "" {
			cName = strings.Split(jid, "@")[0]
		}
		overrideList = append(overrideList, ChatOverrideItem{
			ChatJID:  jid,
			ChatName: cName,
			Provider: s.Provider,
			Model:    s.Model,
			Style:    s.Style,
		})
	}
	sort.Slice(overrideList, func(i, j int) bool {
		return overrideList[i].ChatName < overrideList[j].ChatName
	})

	recommended := map[string][]string{
		"openrouter": {
			"cognitivecomputations/dolphin-mistral-24b-venice-edition",
			"nousresearch/hermes-3-llama-3.1-405b",
			"meta-llama/llama-3.3-70b-instruct:free",
			"mistralai/mistral-nemo",
		},
		"groq": {
			"llama-3.3-70b-versatile",
			"llama-3.1-8b-instant",
			"openai/gpt-oss-120b",
			"openai/gpt-oss-20b",
			"qwen/qwen3.8-27b",
			"minimaxai/minimax-m2.7",
		},
		"gemini": {
			"gemini-3.5-flash-lite",
			"gemini-2.5-pro",
		},
		"ollama": {
			"dolphin-llama3",
			"dolphin-mistral",
			"wizardlm-uncensored",
		},
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":           true,
		"activeProvider":    activeProvider,
		"activeModel":       activeModel,
		"activeStyle":       gemini.GlobalClient.GetGlobalStyle(),
		"availableStyles":   []string{"formal", "genz", "self"},
		"status":            status,
		"chatOverrides":     overrideList,
		"recommendedModels": recommended,
	})
}

func handleAISetProvider(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if gemini.GlobalClient == nil {
		http.Error(w, "AI service not initialized", http.StatusInternalServerError)
		return
	}

	var body struct {
		Provider string `json:"provider"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Provider == "" {
		http.Error(w, "Invalid provider payload", http.StatusBadRequest)
		return
	}

	if err := gemini.GlobalClient.SetProvider(body.Provider); err != nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	LogEvent("SUCCESS", fmt.Sprintf("AI Provider switched to %s via dashboard", body.Provider))
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"provider": gemini.GlobalClient.GetProvider(),
		"model":    gemini.GlobalClient.GetActiveModel(),
	})
}

func handleAISetModel(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if gemini.GlobalClient == nil {
		http.Error(w, "AI service not initialized", http.StatusInternalServerError)
		return
	}

	var body struct {
		Model string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Model == "" {
		http.Error(w, "Invalid model payload", http.StatusBadRequest)
		return
	}

	gemini.GlobalClient.SetActiveModel(body.Model)
	LogEvent("SUCCESS", fmt.Sprintf("AI Model updated to %s via dashboard", body.Model))
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"model":   gemini.GlobalClient.GetActiveModel(),
	})
}

func handleAISetStyle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if gemini.GlobalClient == nil {
		http.Error(w, "AI service not initialized", http.StatusInternalServerError)
		return
	}

	var body struct {
		Style string `json:"style"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Style == "" {
		http.Error(w, "Invalid style payload", http.StatusBadRequest)
		return
	}

	if err := gemini.GlobalClient.SetGlobalStyle(body.Style); err != nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	LogEvent("SUCCESS", fmt.Sprintf("AI Persona Style switched to %s via dashboard", body.Style))
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"style":   gemini.GlobalClient.GetGlobalStyle(),
	})
}

func handleAIChatOverride(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if gemini.GlobalClient == nil {
		http.Error(w, "AI service not initialized", http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case http.MethodPost:
		var body struct {
			ChatJID  string `json:"chatJid"`
			Provider string `json:"provider"`
			Model    string `json:"model"`
			Style    string `json:"style"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ChatJID == "" {
			http.Error(w, "Missing chatJid", http.StatusBadRequest)
			return
		}

		if body.Provider != "" {
			if err := gemini.GlobalClient.SetChatProvider(body.ChatJID, body.Provider); err != nil {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   err.Error(),
				})
				return
			}
		}
		if body.Model != "" {
			gemini.GlobalClient.SetChatModel(body.ChatJID, body.Model)
		}
		if body.Style != "" {
			if body.Style == "inherit" || body.Style == "default" {
				_ = gemini.GlobalClient.SetChatStyle(body.ChatJID, "")
			} else {
				_ = gemini.GlobalClient.SetChatStyle(body.ChatJID, body.Style)
			}
		}

		LogEvent("SUCCESS", fmt.Sprintf("Set AI overrides for chat %s (provider=%s, style=%s) via dashboard", body.ChatJID, body.Provider, body.Style))
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":  true,
			"chatJid":  body.ChatJID,
			"provider": gemini.GlobalClient.GetChatProvider(body.ChatJID),
			"model":    gemini.GlobalClient.GetChatModel(body.ChatJID),
			"style":    gemini.GlobalClient.GetChatStyle(body.ChatJID),
		})

	case http.MethodDelete:
		chatJID := r.URL.Query().Get("chatJid")
		if chatJID == "" {
			var body struct {
				ChatJID string `json:"chatJid"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			chatJID = body.ChatJID
		}
		if chatJID == "" {
			http.Error(w, "Missing chatJid", http.StatusBadRequest)
			return
		}

		gemini.GlobalClient.ClearChatOverride(chatJID)
		LogEvent("SYSTEM", fmt.Sprintf("Reset AI provider override for chat %s to default via dashboard", chatJID))
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}


func handleWelcome(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		settings := commands.GetAllWelcomeSettings()
		enriched := make(map[string]map[string]interface{})
		for jid, cfg := range settings {
			cName := store.GlobalStore.GetContact(jid)
			if cName == "" {
				cName = strings.Split(jid, "@")[0]
			}
			enriched[jid] = map[string]interface{}{
				"jid":       jid,
				"groupName": cName,
				"enabled":   cfg.Enabled,
				"text":      cfg.Message,
			}
		}
		_ = json.NewEncoder(w).Encode(enriched)

	case http.MethodPost:
		var body struct {
			JID     string `json:"jid"`
			Enabled bool   `json:"enabled"`
			Text    string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.JID == "" {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		commands.SetWelcomeConfigForJID(body.JID, body.Enabled, body.Text)
		LogEvent("SUCCESS", fmt.Sprintf("Welcome banner updated for %s via dashboard", body.JID))
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}
}

func handleModerationMutes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	mutes := commands.GetAllMutedUsers()
	list := make([]map[string]interface{}, 0)

	for chatJID, users := range mutes {
		cName := store.GlobalStore.GetContact(chatJID)
		if cName == "" {
			cName = strings.Split(chatJID, "@")[0]
		}
		for _, u := range users {
			uName := store.GlobalStore.GetContact(u)
			if uName == "" {
				uName = strings.Split(u, "@")[0]
			}
			list = append(list, map[string]interface{}{
				"jid":        chatJID,
				"chatName":   cName,
				"senderId":   u,
				"senderName": uName,
				"phone":      strings.Split(u, "@")[0],
				"isGroup":    strings.HasSuffix(chatJID, "@g.us"),
			})
		}
	}

	_ = json.NewEncoder(w).Encode(list)
}

func handleModerationMute(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodPost:
		var body struct {
			JID      string `json:"jid"`
			SenderID string `json:"senderId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.JID == "" || body.SenderID == "" {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		cleanUser := body.SenderID
		if !strings.Contains(cleanUser, "@") {
			cleanUser += "@s.whatsapp.net"
		}

		commands.AddMutedUser(body.JID, cleanUser)
		LogEvent("SYSTEM", fmt.Sprintf("Muted %s in %s via dashboard", cleanUser, body.JID))
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})

	case http.MethodDelete:
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		// api / moderation / mute / :jid / :senderId
		if len(parts) >= 5 {
			chatJID := parts[3]
			senderID := parts[4]
			commands.RemoveMutedUser(chatJID, senderID)
			LogEvent("SYSTEM", fmt.Sprintf("Unmuted %s in %s via dashboard", senderID, chatJID))
		}
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}
}

func handleSendMessage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if whatsapp.GlobalWA == nil || whatsapp.GlobalWA.Client == nil || !whatsapp.GlobalWA.Client.IsConnected() {
		http.Error(w, "WhatsApp client disconnected", http.StatusServiceUnavailable)
		return
	}

	var body struct {
		JID  string `json:"jid"`
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.JID == "" || body.Text == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	cleanJID := strings.TrimSpace(body.JID)
	if !strings.Contains(cleanJID, "@") {
		cleanJID = cleanJID + "@s.whatsapp.net"
	}
	parsedJID, err := types.ParseJID(cleanJID)
	if err != nil {
		http.Error(w, "Invalid JID", http.StatusBadRequest)
		return
	}

	msg := &waProto.Message{Conversation: proto.String(body.Text)}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resp, err := whatsapp.GlobalWA.Client.SendMessage(ctx, parsedJID, msg)
	if err != nil {
		LogEvent("ERROR", fmt.Sprintf("Failed to send message to %s: %v", cleanJID, err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if resp.ID != "" {
		_ = store.RecordBotMessage(resp.ID, parsedJID.String(), body.Text)
	}

	LogEvent("MSG", fmt.Sprintf("Direct message dispatched to %s via dashboard", cleanJID))
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func handleBroadcastMessage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if whatsapp.GlobalWA == nil || whatsapp.GlobalWA.Client == nil || !whatsapp.GlobalWA.Client.IsConnected() {
		http.Error(w, "WhatsApp client disconnected", http.StatusServiceUnavailable)
		return
	}

	var body struct {
		Text       string `json:"text"`
		TargetType string `json:"targetType"` // "groups", "dms", "all"
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Text == "" {
		http.Error(w, "Missing text", http.StatusBadRequest)
		return
	}

	contacts := store.GlobalStore.GetAllContacts()
	targets := make([]string, 0)

	for jid := range contacts {
		isGroup := strings.HasSuffix(jid, "@g.us")
		if body.TargetType == "groups" && isGroup {
			targets = append(targets, jid)
		} else if body.TargetType == "dms" && !isGroup {
			targets = append(targets, jid)
		} else if body.TargetType == "all" || body.TargetType == "" {
			targets = append(targets, jid)
		}
	}

	go func() {
		sentCount := 0
		for _, target := range targets {
			parsedJID, err := types.ParseJID(target)
			if err == nil {
				msg := &waProto.Message{Conversation: proto.String(body.Text)}
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				_, _ = whatsapp.GlobalWA.Client.SendMessage(ctx, parsedJID, msg)
				cancel()
				sentCount++
				time.Sleep(800 * time.Millisecond) // safe rate-limiting
			}
		}
		LogEvent("SUCCESS", fmt.Sprintf("Broadcast completed to %d/%d recipients via dashboard", sentCount, len(targets)))
	}()

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"queuedCount":  len(targets),
		"notification": fmt.Sprintf("Queued broadcast to %d chats", len(targets)),
	})
}

func handleAutoReply(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		rules := store.GlobalPersistent.GetAutoReplies()
		triggers := make(map[string]string)
		for _, rule := range rules {
			triggers[rule.Trigger] = rule.Response
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled":  true,
			"mode":     "keyword",
			"triggers": triggers,
			"rules":    rules,
		})

	case http.MethodPost:
		var body struct {
			AddTrigger *struct {
				Phrase string `json:"phrase"`
				Reply  string `json:"reply"`
			} `json:"addTrigger"`
			RemoveTrigger string `json:"removeTrigger"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)

		if body.AddTrigger != nil && body.AddTrigger.Phrase != "" {
			store.GlobalPersistent.AddAutoReply(store.AutoReplyRule{
				Trigger:  body.AddTrigger.Phrase,
				Response: body.AddTrigger.Reply,
				Exact:    false,
			})
			LogEvent("SUCCESS", fmt.Sprintf("Added auto-reply trigger '%s' via dashboard", body.AddTrigger.Phrase))
		}

		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}
}

func handleAutoReplyFlush(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"count":   0,
	})
}

func handleDeletedMessages(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	messages := store.GlobalPersistent.GetDeletedMessages(100)
	_ = json.NewEncoder(w).Encode(messages)
}

func handleAuditDeleted(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodDelete {
		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(pathParts) <= 3 {
			store.GlobalPersistent.ClearAllAudit()
			LogEvent("INFO", "Cleared all intercepted deleted messages from audit vault")
		} else if len(pathParts) == 4 {
			targetID := pathParts[3]
			store.GlobalPersistent.DeleteAuditEntry("", targetID)
		} else if len(pathParts) >= 5 {
			if pathParts[3] == "chat" {
				chatJID := pathParts[4]
				store.GlobalPersistent.DeleteAllAuditForChat(chatJID)
			} else {
				chatJID := pathParts[3]
				targetID := pathParts[4]
				store.GlobalPersistent.DeleteAuditEntry(chatJID, targetID)
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
		return
	}

	groupedAudit := store.GlobalPersistent.GetAllDeletedGrouped()
	outGrouped := make(map[string][]map[string]interface{})

	for chatJID, items := range groupedAudit {
		chatName := store.GlobalStore.GetContact(chatJID)
		if chatName == "" {
			chatName = strings.Split(chatJID, "@")[0]
		}

		list := make([]map[string]interface{}, 0, len(items))
		for _, m := range items {
			sName := m.SenderName
			if sName == "" {
				sName = m.PushName
			}
			if sName == "" {
				sName = store.GlobalStore.GetContact(m.SenderJID)
			}
			if sName == "" {
				sName = strings.Split(m.SenderJID, "@")[0]
			}

			cName := m.ChatName
			if cName == "" {
				cName = chatName
			}

			entry := map[string]interface{}{
				"id":            m.ID,
				"sender":        m.SenderJID,
				"senderJid":     m.SenderJID,
				"senderName":    sName,
				"chatName":      cName,
				"pushName":      m.PushName,
				"text":          m.Text,
				"mediaType":     m.MediaType,
				"localFilePath": m.LocalFilePath,
				"deletedAt":     m.DeletedAt.Format(time.RFC3339),
				"message": map[string]interface{}{
					"conversation": m.Text,
				},
			}
			list = append(list, entry)
		}
		outGrouped[chatJID] = list
	}

	resp := map[string]interface{}{
		"deletedMsgs": outGrouped,
		"deletedVo":   map[string]interface{}{},
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func handleLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodDelete {
		logsMu.Lock()
		recentLogs = make([]LogEntry, 0, 200)
		logsMu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
		return
	}

	logsMu.RLock()
	defer logsMu.RUnlock()
	_ = json.NewEncoder(w).Encode(recentLogs)
}

// -----------------------------------------------------------------------------
// Utilities
// -----------------------------------------------------------------------------

func getDirSize(path string) int64 {
	var size int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

func getFileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// -----------------------------------------------------------------------------
// Daily Status Reports & Gamified Streaks
// -----------------------------------------------------------------------------

func handleReports(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if store.GlobalReportsStore == nil {
		http.Error(w, `{"error":"Reports store not initialized"}`, http.StatusInternalServerError)
		return
	}

	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		dateStr = config.Now().Format("2006-01-02")
	}

	reports := store.GlobalReportsStore.GetReportsForDate(dateStr)
	if reports == nil {
		reports = make([]store.ReportEntry, 0)
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"date":    dateStr,
		"reports": reports,
		"count":   len(reports),
	})
}

func handleReportGroups(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if store.GlobalReportsStore == nil {
		http.Error(w, `{"error":"Reports store not initialized"}`, http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case http.MethodGet:
		groups := store.GlobalReportsStore.GetReportGroups()
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"groups": groups,
		})

	case http.MethodPost:
		var body struct {
			GroupJID string `json:"groupJid"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.GroupJID) == "" {
			http.Error(w, `{"error":"Valid groupJid required"}`, http.StatusBadRequest)
			return
		}
		store.GlobalReportsStore.AddReportGroup(body.GroupJID)
		LogEvent("INFO", fmt.Sprintf("Added status report group: %s", body.GroupJID))
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"groups":  store.GlobalReportsStore.GetReportGroups(),
		})

	case http.MethodDelete:
		var body struct {
			GroupJID string `json:"groupJid"`
		}
		body.GroupJID = r.URL.Query().Get("groupJid")
		if body.GroupJID == "" {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		if strings.TrimSpace(body.GroupJID) == "" {
			http.Error(w, `{"error":"Valid groupJid required"}`, http.StatusBadRequest)
			return
		}
		store.GlobalReportsStore.RemoveReportGroup(body.GroupJID)
		LogEvent("INFO", fmt.Sprintf("Removed status report group: %s", body.GroupJID))
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"groups":  store.GlobalReportsStore.GetReportGroups(),
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleReportsLeaderboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if store.GlobalReportsStore == nil {
		http.Error(w, `{"error":"Reports store not initialized"}`, http.StatusInternalServerError)
		return
	}

	leaderboard := store.GlobalReportsStore.GetLeaderboard()
	_ = json.NewEncoder(w).Encode(leaderboard)
}

func handleReportsGenerate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		var body struct {
			Date string `json:"date"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		dateStr = body.Date
	}
	if dateStr == "" {
		dateStr = config.Now().Format("2006-01-02")
	}

	summary, err := commands.GenerateDailyReportSummary(r.Context(), dateStr)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"date":    dateStr,
		"summary": summary,
	})
}

// -----------------------------------------------------------------------------
// Group Deadlines & Aesthetic PFP
// -----------------------------------------------------------------------------

func handleDeadlines(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if store.GlobalReportsStore == nil {
		http.Error(w, `{"error":"Reports store not initialized"}`, http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case http.MethodGet:
		deadlines := store.GlobalReportsStore.GetAllGroupDeadlines()
		_ = json.NewEncoder(w).Encode(deadlines)

	case http.MethodPost:
		var body struct {
			GroupJID   string `json:"groupJid"`
			TargetDate string `json:"targetDate"`
			Title      string `json:"title"`
			UpdateNow  bool   `json:"updateNow"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"error":"Invalid request payload"}`, http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(body.GroupJID) == "" || strings.TrimSpace(body.TargetDate) == "" {
			http.Error(w, `{"error":"groupJid and targetDate are required"}`, http.StatusBadRequest)
			return
		}
		if _, err := time.Parse("2006-01-02", strings.TrimSpace(body.TargetDate)); err != nil {
			http.Error(w, `{"error":"Invalid date format (YYYY-MM-DD required)"}`, http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(body.Title) == "" {
			body.Title = "Target Deadline"
		}

		store.GlobalReportsStore.SetGroupDeadline(body.GroupJID, body.TargetDate, body.Title)
		LogEvent("INFO", fmt.Sprintf("Configured group deadline: %s for %s (%s)", body.Title, body.GroupJID, body.TargetDate))

		var pfpErr string
		if body.UpdateNow && whatsapp.GlobalWA != nil && whatsapp.GlobalWA.Client != nil {
			jpegBytes, genErr := pfp.GenerateDeadlinePFP(body.Title, body.TargetDate)
			if genErr != nil {
				pfpErr = genErr.Error()
			} else if targetJID, parseErr := types.ParseJID(body.GroupJID); parseErr == nil {
				_, setErr := whatsapp.GlobalWA.Client.SetGroupPhoto(r.Context(), targetJID, jpegBytes)
				if setErr != nil {
					pfpErr = setErr.Error()
				}
			}
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":    true,
			"groupJid":   body.GroupJID,
			"title":      body.Title,
			"targetDate": body.TargetDate,
			"pfpError":   pfpErr,
		})

	case http.MethodDelete:
		groupJID := r.URL.Query().Get("groupJid")
		if groupJID == "" {
			var body struct {
				GroupJID string `json:"groupJid"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			groupJID = body.GroupJID
		}
		if strings.TrimSpace(groupJID) == "" {
			http.Error(w, `{"error":"groupJid required"}`, http.StatusBadRequest)
			return
		}
		store.GlobalReportsStore.RemoveGroupDeadline(groupJID)
		LogEvent("INFO", fmt.Sprintf("Removed deadline for group %s", groupJID))
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleDeadlinesPreview(w http.ResponseWriter, r *http.Request) {
	title := r.URL.Query().Get("title")
	if strings.TrimSpace(title) == "" {
		title = "Goal Deadline"
	}
	dateStr := r.URL.Query().Get("date")
	if strings.TrimSpace(dateStr) == "" {
		dateStr = config.Now().AddDate(0, 0, 14).Format("2006-01-02")
	}

	jpegBytes, err := pfp.GenerateDeadlinePFP(title, dateStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate preview: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(jpegBytes)))
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	_, _ = w.Write(jpegBytes)
}

func handleDeadlinesUpdatePFP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if whatsapp.GlobalWA == nil || whatsapp.GlobalWA.Client == nil {
		http.Error(w, `{"error":"WhatsApp client is not connected"}`, http.StatusServiceUnavailable)
		return
	}

	var body struct {
		GroupJID string `json:"groupJid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.GroupJID) == "" {
		http.Error(w, `{"error":"groupJid required"}`, http.StatusBadRequest)
		return
	}

	cfg, found := store.GlobalReportsStore.GetGroupDeadline(body.GroupJID)
	if !found {
		http.Error(w, `{"error":"No active deadline found for this group"}`, http.StatusNotFound)
		return
	}

	jpegBytes, err := pfp.GenerateDeadlinePFP(cfg.Title, cfg.TargetDate)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"PFP generation failed: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	targetJID, err := types.ParseJID(body.GroupJID)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Invalid group JID: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	picID, err := whatsapp.GlobalWA.Client.SetGroupPhoto(r.Context(), targetJID, jpegBytes)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"WhatsApp SetGroupPhoto failed: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	LogEvent("INFO", fmt.Sprintf("Manually updated group photo for %s (pictureID: %s)", body.GroupJID, picID))
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"pictureId": picID,
	})
}

// =============================================================================
// MeowCaller VoIP & Song Party Handlers
// =============================================================================

func handleMeowCallerStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	status := meowcaller.GlobalManager.GetStatus()
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"status":  status,
	})
}

func handleMeowCallerToggle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Enabled         bool `json:"enabled"`
		DurationMinutes int  `json:"durationMinutes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	meowcaller.GlobalManager.SetEnabled(body.Enabled, body.DurationMinutes)
	status := meowcaller.GlobalManager.GetStatus()

	LogEvent("INFO", fmt.Sprintf("MeowCaller state set to %v (duration: %d mins) via dashboard", body.Enabled, body.DurationMinutes))
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"status":  status,
	})
}

func handleMeowCallerAutoAccept(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		AutoAccept bool `json:"autoAccept"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	meowcaller.GlobalManager.SetAutoAcceptCalls(body.AutoAccept)
	status := meowcaller.GlobalManager.GetStatus()

	LogEvent("INFO", fmt.Sprintf("MeowCaller auto-accept set to %v via dashboard", body.AutoAccept))
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"status":  status,
	})
}

func handleMeowCallerCall(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Target string `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(body.Target) == "" {
		http.Error(w, `{"error":"Target phone or JID cannot be empty"}`, http.StatusBadRequest)
		return
	}

	call, err := meowcaller.GlobalManager.Dial(r.Context(), body.Target)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	LogEvent("INFO", fmt.Sprintf("Dialed %s via MeowCaller console", body.Target))
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"callId":  call.ID(),
		"peer":    body.Target,
	})
}

func handleMeowCallerHangup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	if err := meowcaller.GlobalManager.Hangup(); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	LogEvent("INFO", "Hung up active call via dashboard console")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

func handleMeowCallerAnswer(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	if err := meowcaller.GlobalManager.AnswerCall(); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	LogEvent("INFO", "Answered incoming call via dashboard console")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"status":  meowcaller.GlobalManager.GetStatus(),
	})
}

func handleMeowCallerReject(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	if err := meowcaller.GlobalManager.RejectCall(); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	LogEvent("INFO", "Rejected incoming call via dashboard console")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"status":  meowcaller.GlobalManager.GetStatus(),
	})
}

func handleMeowCallerPartyPlay(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Query  string `json:"query"`
		Target string `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	query := strings.TrimSpace(body.Query)
	if query == "" {
		http.Error(w, `{"error":"Song query cannot be empty"}`, http.StatusBadRequest)
		return
	}

	if !meowcaller.GlobalManager.IsEnabled() {
		http.Error(w, `{"error":"MeowCaller is disabled. Please enable it before playing songs."}`, http.StatusBadRequest)
		return
	}

	status := meowcaller.GlobalManager.GetStatus()
	var track *meowcaller.Track
	var err error

	if status.IsPartyActive || status.ActiveCall != nil {
		track, err = meowcaller.GlobalManager.AddToQueue(r.Context(), query, "Dashboard Admin")
	} else {
		target := strings.TrimSpace(body.Target)
		track, err = meowcaller.GlobalManager.StartParty(r.Context(), target, query, "Dashboard Admin", "")
	}

	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	LogEvent("INFO", fmt.Sprintf("Added track \"%s\" to Song Party from dashboard", track.Title))
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"track":   track,
		"status":  meowcaller.GlobalManager.GetStatus(),
	})
}

func handleMeowCallerPartyControl(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	var err error
	switch strings.ToLower(body.Action) {
	case "pause":
		err = meowcaller.GlobalManager.Pause()
	case "resume":
		err = meowcaller.GlobalManager.Resume()
	case "skip":
		_, err = meowcaller.GlobalManager.Skip()
	case "stop":
		meowcaller.GlobalManager.StopParty()
	default:
		http.Error(w, `{"error":"Unknown action"}`, http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	LogEvent("INFO", fmt.Sprintf("Song Party action '%s' executed from dashboard", body.Action))
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"status":  meowcaller.GlobalManager.GetStatus(),
	})
}

func handleMeowCallerPartyQueueRemove(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Index int `json:"index"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	removed, err := meowcaller.GlobalManager.RemoveFromQueue(body.Index)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	LogEvent("INFO", fmt.Sprintf("Removed track '%s' from Song Party queue", removed.Title))
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"removed": removed,
		"status":  meowcaller.GlobalManager.GetStatus(),
	})
}

func handleMeowCallerAudioStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	status := meowcaller.GlobalManager.GetAudioMonitorStatus()
	_ = json.NewEncoder(w).Encode(status)
}

func handleMeowCallerAudioWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "listener disconnected")

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	subID, audioCh, unsub := meowcaller.GlobalManager.SubscribeAudio(meowcaller.ChannelAll)
	defer unsub()

	LogEvent("INFO", fmt.Sprintf("Dashboard client started live call audio monitor (id: %s)", subID[:8]))

	// Read loop to detect client disconnection, incoming mic PCM frames, or control packets
	go func() {
		defer cancel()
		for {
			msgType, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			if msgType == websocket.MessageBinary && len(data) >= 3 {
				// Binary audio packet from dashboard mic: tag 0x03 followed by 16-bit LE PCM
				var pcmBytes []byte
				if data[0] == byte(meowcaller.ChannelMic) {
					pcmBytes = data[1:]
				} else if len(data)%2 == 0 {
					// Raw PCM without channel prefix
					pcmBytes = data
				}

				numSamples := len(pcmBytes) / 2
				if numSamples > 0 {
					samples := make([]float32, numSamples)
					for i := 0; i < numSamples; i++ {
						s16 := int16(binary.LittleEndian.Uint16(pcmBytes[i*2:]))
						samples[i] = float32(s16) / 32768.0
					}
					meowcaller.GlobalManager.PushMicFrame(samples)
				}
			} else if msgType == websocket.MessageText {
				var cmd struct {
					Action    string   `json:"action"`
					Active    *bool    `json:"active"`
					Mode      string   `json:"mode"`
					DuckLevel *float64 `json:"duckLevel"`
					MicGain   *float64 `json:"micGain"`
				}
				if json.Unmarshal(data, &cmd) == nil {
					if cmd.Action == "set_mic" && cmd.Active != nil {
						meowcaller.GlobalManager.SetRadioMicActive(*cmd.Active)
					}
					if cmd.Mode != "" {
						meowcaller.GlobalManager.SetRadioMode(cmd.Mode)
					}
					if cmd.DuckLevel != nil {
						meowcaller.GlobalManager.SetRadioDuckLevel(float32(*cmd.DuckLevel))
					}
					if cmd.MicGain != nil {
						meowcaller.GlobalManager.SetRadioMicGain(float32(*cmd.MicGain))
					}
				}
			}
		}
	}()

	var sentPackets uint64
	// Send loop for real-time binary audio frames
	for {
		select {
		case <-ctx.Done():
			return
		case packet, ok := <-audioCh:
			if !ok {
				return
			}
			writeCtx, writeCancel := context.WithTimeout(ctx, 3*time.Second)
			err := conn.Write(writeCtx, websocket.MessageBinary, packet)
			writeCancel()
			if err != nil {
				return
			}
			sent := atomic.AddUint64(&sentPackets, 1)
			if sent <= 5 || sent%100 == 0 {
				tag := "unknown"
				if len(packet) > 0 {
					if packet[0] == 2 {
						tag = "inbound caller voice"
					} else if packet[0] == 1 {
						tag = "outbound bot/music"
					}
				}
				if sent == 1 {
					LogEvent("SUCCESS", fmt.Sprintf("Live Audio Monitor streaming active: delivered first packet to browser (%s, %d bytes)", tag, len(packet)))
				} else {
					LogEvent("INFO", fmt.Sprintf("Live Audio streaming to browser: packet #%d (%s, %d bytes)", sent, tag, len(packet)))
				}
			}
		}
	}
}

func handleMeowCallerRadioConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Active    *bool    `json:"active"`
		Mode      string   `json:"mode"`
		DuckLevel *float64 `json:"duckLevel"`
		MicGain   *float64 `json:"micGain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if body.Active != nil {
		meowcaller.GlobalManager.SetRadioMicActive(*body.Active)
	}
	if body.Mode != "" {
		meowcaller.GlobalManager.SetRadioMode(body.Mode)
	}
	if body.DuckLevel != nil {
		meowcaller.GlobalManager.SetRadioDuckLevel(float32(*body.DuckLevel))
	}
	if body.MicGain != nil {
		meowcaller.GlobalManager.SetRadioMicGain(float32(*body.MicGain))
	}

	status := meowcaller.GlobalManager.GetRadioStatus()
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"radio":   status,
	})
}

func handleMeowCallerRadioStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	status := meowcaller.GlobalManager.GetRadioStatus()
	_ = json.NewEncoder(w).Encode(status)
}

func handleMeowCallerAudioStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Connection", "keep-alive")

	// 44-byte WAV header with continuous streaming placeholder length
	var h [44]byte
	copy(h[0:4], "RIFF")
	binary.LittleEndian.PutUint32(h[4:8], 0x7FFFFFFF)
	copy(h[8:12], "WAVE")
	copy(h[12:16], "fmt ")
	binary.LittleEndian.PutUint32(h[16:20], 16)
	binary.LittleEndian.PutUint16(h[20:22], 1)
	binary.LittleEndian.PutUint16(h[22:24], 1)
	binary.LittleEndian.PutUint32(h[24:28], 16000)
	binary.LittleEndian.PutUint32(h[28:32], 32000)
	binary.LittleEndian.PutUint16(h[32:34], 2)
	binary.LittleEndian.PutUint16(h[34:36], 16)
	copy(h[36:40], "data")
	binary.LittleEndian.PutUint32(h[40:44], 0x7FFFFFFF-36)

	if _, err := w.Write(h[:]); err != nil {
		return
	}
	flusher.Flush()

	filter := meowcaller.ChannelAll
	chParam := strings.ToLower(r.URL.Query().Get("channel"))
	if chParam == "incoming" || chParam == "callers" || chParam == "voices" {
		filter = meowcaller.ChannelIncoming
	} else if chParam == "outgoing" || chParam == "bot" || chParam == "music" {
		filter = meowcaller.ChannelOutgoing
	}

	_, audioCh, unsub := meowcaller.GlobalManager.SubscribeAudio(filter)
	defer unsub()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case packet, ok := <-audioCh:
			if !ok {
				return
			}
			if len(packet) > 1 {
				// packet[0] is tag, packet[1:] is 16-bit PCM samples
				if _, err := w.Write(packet[1:]); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	}
}

func handleMeowCallerVoiceBotConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodGet {
		status := meowcaller.GlobalManager.GetStatus()
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":  true,
			"voiceBot": status.VoiceBot,
			"voices":   tts.GetAvailableVoices(),
		})
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var body meowcaller.VoiceBotConfig
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	meowcaller.GlobalManager.UpdateVoiceBotConfig(body)
	status := meowcaller.GlobalManager.GetStatus()

	LogEvent("INFO", fmt.Sprintf("VoiceBot config updated: enabled=%v, voice=%s, server=%s", body.Enabled, body.Voice, body.ServerURL))
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"status":   status,
		"voiceBot": status.VoiceBot,
	})
}

func handleMeowCallerVoiceBotVoices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"voices":  tts.GetAvailableVoices(),
	})
}

func handleMeowCallerVoiceBotConnect(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ServerURL string `json:"serverUrl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	url := strings.TrimSpace(req.ServerURL)
	if url == "" {
		url = "ws://127.0.0.1:6006"
	}

	err := meowcaller.GlobalManager.ConnectVoiceBotWebSocket(url)
	status := meowcaller.GlobalManager.GetStatus()
	if err != nil {
		LogEvent("WARN", fmt.Sprintf("VoiceBot WebSocket connection to %s failed: %v", url, err))
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":   false,
			"connected": false,
			"error":     fmt.Sprintf("Failed to connect to %s: %v", url, err),
			"status":    status,
			"voiceBot":  status.VoiceBot,
		})
		return
	}

	LogEvent("SUCCESS", fmt.Sprintf("VoiceBot connected to WebSocket engine at %s", url))
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"connected": true,
		"message":   fmt.Sprintf("Successfully connected to WebSocket server at %s", url),
		"status":    status,
		"voiceBot":  status.VoiceBot,
	})
}



