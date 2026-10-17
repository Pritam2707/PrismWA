package store

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/polymorfa/hypermeow/types"
	"github.com/polymorfa/hypermeow/types/events"
)

type CachedMessage struct {
	ID        string
	ChatJID   types.JID
	SenderJID types.JID
	PushName  string
	Text      string
	MediaData []byte
	MediaType string
	Timestamp time.Time
	Raw       *events.Message
}

type MessageStore struct {
	mu              sync.RWMutex
	messages        map[string]*CachedMessage
	order           []string
	maxCapacity     int
	isSleeping      atomic.Bool
	lastActivity    time.Time
	antiDeleteChats map[string]bool
	featureChats    map[string]bool
	contacts        map[string]string
	mutedUsers      map[string]time.Time
	ctx             context.Context
	cancel          context.CancelFunc
}

var GlobalStore *MessageStore

func InitMessageStore(ctx context.Context, maxCapacity int) *MessageStore {
	if maxCapacity <= 0 {
		maxCapacity = 2000
	}

	storeCtx, cancel := context.WithCancel(ctx)

	s := &MessageStore{
		messages:        make(map[string]*CachedMessage, maxCapacity),
		order:           make([]string, 0, maxCapacity),
		maxCapacity:     maxCapacity,
		lastActivity:    time.Now(),
		antiDeleteChats: make(map[string]bool),
		featureChats:    make(map[string]bool),
		contacts:        make(map[string]string),
		mutedUsers:      make(map[string]time.Time),
		ctx:             storeCtx,
		cancel:          cancel,
	}
	s.isSleeping.Store(false)

	GlobalStore = s

	// Periodic cleanup worker with context cancellation
	go s.startCleanupWorker(storeCtx, 10*time.Minute, 4*time.Hour)

	return s
}

func (s *MessageStore) Put(msg *CachedMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastActivity = time.Now()

	if _, exists := s.messages[msg.ID]; exists {
		s.messages[msg.ID] = msg
		return
	}

	if len(s.order) >= s.maxCapacity {
		oldestID := s.order[0]
		copy(s.order, s.order[1:])
		s.order[len(s.order)-1] = msg.ID
		delete(s.messages, oldestID)
		s.messages[msg.ID] = msg
		return
	}

	s.messages[msg.ID] = msg
	s.order = append(s.order, msg.ID)
}

func (s *MessageStore) Get(id string) (*CachedMessage, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	msg, found := s.messages[id]
	return msg, found
}

func (s *MessageStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.messages)
}

func (s *MessageStore) SetSleeping(sleeping bool) {
	s.isSleeping.Store(sleeping)
	s.mu.Lock()
	s.lastActivity = time.Now()
	s.mu.Unlock()
}

func (s *MessageStore) IsSleeping() bool {
	return s.isSleeping.Load()
}

func (s *MessageStore) TouchActivity() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastActivity = time.Now()
}

func (s *MessageStore) LastActivity() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastActivity
}

func (s *MessageStore) ToggleAntiDelete(chatJID string, enable bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if enable {
		s.antiDeleteChats[chatJID] = true
	} else {
		delete(s.antiDeleteChats, chatJID)
	}
}

func (s *MessageStore) IsAntiDeleteEnabled(chatJID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.antiDeleteChats[chatJID]
}

func (s *MessageStore) AntiDeleteCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.antiDeleteChats)
}

func (s *MessageStore) ToggleFeatures(chatJID string, enable bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if enable {
		s.featureChats[chatJID] = true
	} else {
		delete(s.featureChats, chatJID)
	}
}

func (s *MessageStore) IsFeaturesEnabled(chatJID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.featureChats[chatJID]
}

func (s *MessageStore) FeatureCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.featureChats)
}

func (s *MessageStore) SetContact(jid string, name string) {
	if jid == "" || name == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.contacts[jid] = name
}

func (s *MessageStore) GetContact(jid string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.contacts[jid]
}

func (s *MessageStore) ContactsCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.contacts)
}

func (s *MessageStore) GetAllContacts() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make(map[string]string)
	for k, v := range s.contacts {
		res[k] = v
	}
	return res
}

func (s *MessageStore) GetAllAntiDeleteChats() map[string]bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make(map[string]bool)
	for k, v := range s.antiDeleteChats {
		res[k] = v
	}
	return res
}

func (s *MessageStore) GetAllFeatureChats() map[string]bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make(map[string]bool)
	for k, v := range s.featureChats {
		res[k] = v
	}
	return res
}

func (s *MessageStore) MuteUser(userJID string, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mutedUsers[userJID] = time.Now().Add(duration)
}

func (s *MessageStore) IsMuted(userJID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	expiry, exists := s.mutedUsers[userJID]
	if !exists {
		return false
	}
	return time.Now().Before(expiry)
}

func (s *MessageStore) Close() {
	s.cancel()
}

func (s *MessageStore) startCleanupWorker(ctx context.Context, interval, maxAge time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.cleanupExpired(maxAge)
		}
	}
}

func (s *MessageStore) cleanupExpired(maxAge time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	newOrder := make([]string, 0, len(s.order))

	for _, id := range s.order {
		msg, ok := s.messages[id]
		if ok && msg.Timestamp.After(cutoff) {
			newOrder = append(newOrder, id)
		} else {
			delete(s.messages, id)
		}
	}
	s.order = newOrder

	now := time.Now()
	for user, expiry := range s.mutedUsers {
		if now.After(expiry) {
			delete(s.mutedUsers, user)
		}
	}
}

// extractJIDUser extracts the bare phone number / username part of a JID without string allocations
func extractJIDUser(jid string) string {
	if at := strings.IndexByte(jid, '@'); at != -1 {
		jid = jid[:at]
	}
	if colon := strings.IndexByte(jid, ':'); colon != -1 {
		jid = jid[:colon]
	}
	return jid
}

// GetChatHistorySinceUserLastMessage returns cached messages for chatJID that occurred
// after the senderJID's last message, capped between fallbackLimit and maxLimit (e.g. 50-100).
// If the user's previous message is not found within recent history, it returns up to fallbackLimit messages.
func (s *MessageStore) GetChatHistorySinceUserLastMessage(chatJID string, senderJID string, excludeID string, fallbackLimit int, maxLimit int) ([]*CachedMessage, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxLimit <= 0 {
		maxLimit = 100
	}
	if fallbackLimit <= 0 {
		fallbackLimit = 50
	}

	cleanChatUser := extractJIDUser(chatJID)

	// Collect recent messages for this chat in reverse chronological order (newest first)
	recent := make([]*CachedMessage, 0, 150)
	for i := len(s.order) - 1; i >= 0; i-- {
		id := s.order[i]
		if id == excludeID {
			continue
		}
		msg, ok := s.messages[id]
		if !ok || msg == nil {
			continue
		}
		if extractJIDUser(msg.ChatJID.User) == cleanChatUser {
			recent = append(recent, msg)
			if len(recent) >= 150 {
				break
			}
		}
	}

	cleanSenderUser := extractJIDUser(senderJID)

	lastUserIdx := -1
	for idx, msg := range recent {
		if extractJIDUser(msg.SenderJID.User) == cleanSenderUser {
			lastUserIdx = idx
			break
		}
	}

	var result []*CachedMessage
	foundLastUser := false

	if lastUserIdx > 0 {
		// All messages from 0 to lastUserIdx-1 were sent while the user was away
		foundLastUser = true
		awayCount := lastUserIdx
		if awayCount > maxLimit {
			awayCount = maxLimit
		}
		result = make([]*CachedMessage, awayCount)
		copy(result, recent[:awayCount])
	} else {
		// Fallback: take fallbackLimit recent messages
		count := fallbackLimit
		if count > len(recent) {
			count = len(recent)
		}
		result = make([]*CachedMessage, count)
		copy(result, recent[:count])
	}

	// Reverse result into chronological order (oldest to newest)
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result, foundLastUser
}

// GetRecentChatHistory returns up to `limit` cached messages for chatJID in chronological order.
func (s *MessageStore) GetRecentChatHistory(chatJID string, limit int, excludeID string) []*CachedMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	cleanChatUser := extractJIDUser(chatJID)
	matched := make([]*CachedMessage, 0, limit)

	for i := len(s.order) - 1; i >= 0; i-- {
		id := s.order[i]
		if id == excludeID {
			continue
		}
		msg, ok := s.messages[id]
		if !ok || msg == nil {
			continue
		}
		if extractJIDUser(msg.ChatJID.User) == cleanChatUser {
			matched = append(matched, msg)
			if len(matched) >= limit {
				break
			}
		}
	}

	// Reverse to chronological order (oldest to newest)
	for i, j := 0, len(matched)-1; i < j; i, j = i+1, j-1 {
		matched[i], matched[j] = matched[j], matched[i]
	}

	return matched
}
