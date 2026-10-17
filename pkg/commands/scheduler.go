package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/polymorfa/hypermeow"
	waProto "github.com/polymorfa/hypermeow/binary/proto"
	"github.com/polymorfa/hypermeow/types"
	"google.golang.org/protobuf/proto"
)

type ReminderItem struct {
	ID        string    `json:"id"`
	ChatJID   string    `json:"chatJid"`
	SenderJID string    `json:"senderJid"`
	Message   string    `json:"message"`
	TriggerAt time.Time `json:"triggerAt"`
}

var (
	schedMu       sync.RWMutex
	remindersList = make([]ReminderItem, 0)
	activeTimers  = make(map[string]*time.Timer)
	remindersFile = filepath.Join("data", "reminders.json")
	timeRegex     = regexp.MustCompile(`^(\d+)([smhd])$`)
)

func init() {
	_ = os.MkdirAll("data", 0755)
	loadReminders()
}

func loadReminders() {
	schedMu.Lock()
	defer schedMu.Unlock()

	if data, err := os.ReadFile(remindersFile); err == nil {
		_ = json.Unmarshal(data, &remindersList)
	}
}

func saveReminders() {
	if data, err := json.MarshalIndent(remindersList, "", "  "); err == nil {
		_ = os.WriteFile(remindersFile, data, 0644)
	}
}

func RegisterSchedulerCommands(r *CommandRegistry) {
	r.RegisterPrefix("!remind", handleRemind)
	r.RegisterPrefix("!reminders", handleListReminders)
	r.RegisterPrefix("!cancelremind", handleCancelRemind)
}

func handleRemind(c *CommandContext) (bool, error) {
	parts := strings.Fields(c.RawText)
	if len(parts) < 3 {
		return true, c.ReplyText("Usage: !remind <time> <message>\nExamples:\n- !remind 10m Take a break\n- !remind 2h Study physics\n- !remind 30s Check oven")
	}

	durationStr := strings.ToLower(parts[1])
	dur, err := parseDuration(durationStr)
	if err != nil {
		return true, c.ReplyText(fmt.Sprintf("Invalid duration: %s. Use format like 30s, 10m, 2h, 1d.", durationStr))
	}

	msgText := strings.TrimSpace(c.RawText[len(parts[0])+len(parts[1])+1:])
	id := fmt.Sprintf("%d", time.Now().UnixNano())
	triggerAt := time.Now().Add(dur)

	item := ReminderItem{
		ID:        id,
		ChatJID:   c.ChatJID.String(),
		SenderJID: c.SenderJID.String(),
		Message:   msgText,
		TriggerAt: triggerAt,
	}

	schedMu.Lock()
	remindersList = append(remindersList, item)
	saveReminders()
	armReminder(c.Client, item)
	schedMu.Unlock()

	return true, c.ReplyText(fmt.Sprintf("Reminder set for %s (in %s): \"%s\"", triggerAt.Format("15:04:05"), dur, msgText))
}

func handleListReminders(c *CommandContext) (bool, error) {
	chatKey := c.ChatJID.String()

	schedMu.RLock()
	chatReminders := make([]ReminderItem, 0)
	for _, r := range remindersList {
		if r.ChatJID == chatKey {
			chatReminders = append(chatReminders, r)
		}
	}
	schedMu.RUnlock()

	if len(chatReminders) == 0 {
		return true, c.ReplyText("No pending reminders for this chat.")
	}

	var sb strings.Builder
	sb.WriteString("*Pending Reminders:*\n")
	for i, r := range chatReminders {
		remaining := time.Until(r.TriggerAt).Round(time.Second)
		sb.WriteString(fmt.Sprintf("%d. [ID: %s] \"%s\" in %s\n", i+1, r.ID[:6], r.Message, remaining))
	}
	return true, c.ReplyText(strings.TrimSpace(sb.String()))
}

func handleCancelRemind(c *CommandContext) (bool, error) {
	parts := strings.Fields(c.RawText)
	if len(parts) < 2 {
		return true, c.ReplyText("Usage: !cancelremind <id>")
	}
	targetID := parts[1]

	schedMu.Lock()
	defer schedMu.Unlock()

	newList := make([]ReminderItem, 0, len(remindersList))
	found := false
	for _, r := range remindersList {
		if strings.HasPrefix(r.ID, targetID) {
			found = true
			if timer, ok := activeTimers[r.ID]; ok {
				timer.Stop()
				delete(activeTimers, r.ID)
			}
		} else {
			newList = append(newList, r)
		}
	}
	remindersList = newList
	saveReminders()

	if !found {
		return true, c.ReplyText(fmt.Sprintf("Reminder ID '%s' not found.", targetID))
	}
	return true, c.ReplyText("Reminder cancelled.")
}

func armReminder(client *whatsmeow.Client, r ReminderItem) {
	delay := time.Until(r.TriggerAt)
	if delay < 0 {
		delay = 100 * time.Millisecond
	}

	timer := time.AfterFunc(delay, func() {
		schedMu.Lock()
		delete(activeTimers, r.ID)
		newList := make([]ReminderItem, 0, len(remindersList))
		for _, item := range remindersList {
			if item.ID != r.ID {
				newList = append(newList, item)
			}
		}
		remindersList = newList
		saveReminders()
		schedMu.Unlock()

		targetChat, err := types.ParseJID(r.ChatJID)
		if err != nil || client == nil {
			return
		}

		userTag := "@" + strings.Split(r.SenderJID, "@")[0]
		text := fmt.Sprintf("Reminder for %s:\n\n%s", userTag, r.Message)
		msg := &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text: proto.String(text),
				ContextInfo: &waProto.ContextInfo{
					MentionedJID: []string{r.SenderJID},
				},
			},
		}
		_, _ = client.SendMessage(context.Background(), targetChat, msg)
	})

	activeTimers[r.ID] = timer
}

func RestoreReminders(client *whatsmeow.Client) {
	schedMu.Lock()
	defer schedMu.Unlock()

	for _, r := range remindersList {
		armReminder(client, r)
	}
}

func parseDuration(s string) (time.Duration, error) {
	match := timeRegex.FindStringSubmatch(s)
	if match == nil {
		return 0, fmt.Errorf("invalid time format")
	}
	val, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, err
	}
	unit := match[2]
	switch unit {
	case "s":
		return time.Duration(val) * time.Second, nil
	case "m":
		return time.Duration(val) * time.Minute, nil
	case "h":
		return time.Duration(val) * time.Hour, nil
	case "d":
		return time.Duration(val) * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown unit %s", unit)
	}
}

func GetAllReminders() []ReminderItem {
	schedMu.RLock()
	defer schedMu.RUnlock()
	res := make([]ReminderItem, len(remindersList))
	copy(res, remindersList)
	return res
}

func CreateReminderFromDashboard(chatJID, senderJID, message string, targetTime time.Time, client *whatsmeow.Client) ReminderItem {
	schedMu.Lock()
	defer schedMu.Unlock()

	item := ReminderItem{
		ID:        fmt.Sprintf("remind-%d", time.Now().UnixNano()),
		ChatJID:   chatJID,
		SenderJID: senderJID,
		Message:   message,
		TriggerAt: targetTime,
	}

	remindersList = append(remindersList, item)
	saveReminders()

	if client != nil {
		armReminder(client, item)
	}

	return item
}

func CancelReminderByID(id string) bool {
	schedMu.Lock()
	defer schedMu.Unlock()

	found := false
	newList := make([]ReminderItem, 0, len(remindersList))
	for _, r := range remindersList {
		if r.ID == id {
			found = true
			if t, ok := activeTimers[id]; ok {
				t.Stop()
				delete(activeTimers, id)
			}
		} else {
			newList = append(newList, r)
		}
	}

	if found {
		remindersList = newList
		saveReminders()
	}

	return found
}
