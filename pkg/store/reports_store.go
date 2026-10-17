package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"whatsapp-bot/pkg/config"
)

// ReportEntry represents a single user's daily achievement submission
type ReportEntry struct {
	UserJID    string    `json:"userJid"`
	SenderName string    `json:"senderName"`
	GroupJID   string    `json:"groupJid,omitempty"`
	Date       string    `json:"date"` // YYYY-MM-DD
	Content    string    `json:"content"`
	Timestamp  time.Time `json:"timestamp"`
}

// UserReportStats represents gamified streaks and points for a user
type UserReportStats struct {
	UserJID          string    `json:"userJid"`
	Name             string    `json:"name"`
	Points           int       `json:"points"`
	CurrentStreak    int       `json:"currentStreak"`
	BestStreak       int       `json:"bestStreak"`
	LastReportDate   string    `json:"lastReportDate"` // YYYY-MM-DD
	TotalSubmissions int       `json:"totalSubmissions"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// DeadlineConfig holds the group countdown configuration
type DeadlineConfig struct {
	GroupJID    string    `json:"groupJid"`
	TargetDate  string    `json:"targetDate"` // YYYY-MM-DD
	Title       string    `json:"title"`
	Enabled     bool      `json:"enabled"`
	LastUpdated time.Time `json:"lastUpdated"`
}

// ReportsStore manages persistent status reports, streaks, and group configurations
type ReportsStore struct {
	mu           sync.RWMutex
	dataFile     string
	deadlinesFile string
	TargetGroups map[string]bool                 `json:"targetGroups"`
	Reports      []ReportEntry                   `json:"reports"`
	Stats        map[string]*UserReportStats     `json:"stats"`
	Deadlines    map[string]*DeadlineConfig      `json:"deadlines"`
}

var GlobalReportsStore *ReportsStore

// InitReportsStore initializes the reports storage
func InitReportsStore(dataDir string) *ReportsStore {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		fmt.Printf("[ReportsStore] Failed to create data dir: %v\n", err)
	}

	rs := &ReportsStore{
		dataFile:      filepath.Join(dataDir, "status_reports.json"),
		deadlinesFile: filepath.Join(dataDir, "group_deadlines.json"),
		TargetGroups:  make(map[string]bool),
		Reports:       make([]ReportEntry, 0),
		Stats:         make(map[string]*UserReportStats),
		Deadlines:     make(map[string]*DeadlineConfig),
	}

	rs.load()
	GlobalReportsStore = rs
	return rs
}

func (s *ReportsStore) load() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Load reports and stats
	if data, err := os.ReadFile(s.dataFile); err == nil && len(data) > 0 {
		var wrapper struct {
			TargetGroups map[string]bool             `json:"targetGroups"`
			Reports      []ReportEntry               `json:"reports"`
			Stats        map[string]*UserReportStats `json:"stats"`
		}
		if err := json.Unmarshal(data, &wrapper); err == nil {
			if wrapper.TargetGroups != nil {
				s.TargetGroups = wrapper.TargetGroups
			}
			if wrapper.Reports != nil {
				s.Reports = wrapper.Reports
			}
			if wrapper.Stats != nil {
				s.Stats = wrapper.Stats
			}
		}
	}

	// 2. Load deadlines
	if data, err := os.ReadFile(s.deadlinesFile); err == nil && len(data) > 0 {
		var deadlines map[string]*DeadlineConfig
		if err := json.Unmarshal(data, &deadlines); err == nil && deadlines != nil {
			s.Deadlines = deadlines
		}
	}
}

func (s *ReportsStore) saveReportsLocked() error {
	wrapper := struct {
		TargetGroups map[string]bool             `json:"targetGroups"`
		Reports      []ReportEntry               `json:"reports"`
		Stats        map[string]*UserReportStats `json:"stats"`
	}{
		TargetGroups: s.TargetGroups,
		Reports:      s.Reports,
		Stats:        s.Stats,
	}

	data, err := json.MarshalIndent(wrapper, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.dataFile, data, 0644)
}

func (s *ReportsStore) saveDeadlinesLocked() error {
	data, err := json.MarshalIndent(s.Deadlines, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.deadlinesFile, data, 0644)
}

// SubmitReport logs an achievement for today, updates streaks, and awards points.
// If the user submits multiple reports on the same day, they are concatenated together.
func (s *ReportsStore) SubmitReport(userJID, senderName, groupJID, content string) (ReportEntry, *UserReportStats, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cleanUser := strings.TrimSpace(userJID)
	cleanContent := strings.TrimSpace(content)
	now := config.Now()
	todayStr := now.Format("2006-01-02")
	yesterdayStr := now.AddDate(0, 0, -1).Format("2006-01-02")

	// 1. Record or concatenate entry
	var entry ReportEntry
	foundIdx := -1
	for i, r := range s.Reports {
		if r.Date == todayStr && (r.UserJID == cleanUser || (cleanUser != "" && strings.Split(r.UserJID, "@")[0] == strings.Split(cleanUser, "@")[0])) {
			foundIdx = i
			break
		}
	}

	if foundIdx >= 0 {
		// Concatenate multiple submissions on the same day
		existingContent := strings.TrimSpace(s.Reports[foundIdx].Content)
		s.Reports[foundIdx].Content = existingContent + "\n• " + cleanContent
		s.Reports[foundIdx].Timestamp = now
		if senderName != "" {
			s.Reports[foundIdx].SenderName = senderName
		}
		if groupJID != "" {
			s.Reports[foundIdx].GroupJID = groupJID
		}
		entry = s.Reports[foundIdx]
	} else {
		entry = ReportEntry{
			UserJID:    cleanUser,
			SenderName: senderName,
			GroupJID:   groupJID,
			Date:       todayStr,
			Content:    cleanContent,
			Timestamp:  now,
		}
		s.Reports = append(s.Reports, entry)
	}

	// 2. Update stats and streaks
	stats, exists := s.Stats[cleanUser]
	if !exists {
		stats = &UserReportStats{
			UserJID:          cleanUser,
			Name:             senderName,
			Points:           0,
			CurrentStreak:    0,
			BestStreak:       0,
			TotalSubmissions: 0,
		}
		s.Stats[cleanUser] = stats
	}

	if senderName != "" {
		stats.Name = senderName
	}
	stats.TotalSubmissions++
	stats.UpdatedAt = now

	pointsAwarded := 10 // Base 10 points per submission
	streakBonus := 0

	if foundIdx >= 0 || stats.LastReportDate == todayStr {
		// Already submitted earlier today; concatenated without streak increment
		pointsAwarded = 2 // small bonus for updating report
	} else if stats.LastReportDate == yesterdayStr {
		// Consecutive day report!
		stats.CurrentStreak++
		if stats.CurrentStreak > stats.BestStreak {
			stats.BestStreak = stats.CurrentStreak
		}
		if stats.CurrentStreak >= 7 {
			streakBonus = 10
		} else if stats.CurrentStreak >= 3 {
			streakBonus = 5
		}
		stats.LastReportDate = todayStr
	} else {
		// Missed one or more days, reset streak to 1
		stats.CurrentStreak = 1
		if stats.BestStreak < 1 {
			stats.BestStreak = 1
		}
		stats.LastReportDate = todayStr
	}

	totalPointsEarned := pointsAwarded + streakBonus
	stats.Points += totalPointsEarned

	_ = s.saveReportsLocked()
	return entry, stats, totalPointsEarned, nil
}

// GetReportsForDate retrieves reports for a given date (YYYY-MM-DD), consolidating by user
func (s *ReportsStore) GetReportsForDate(dateStr string) []ReportEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userMap := make(map[string]int)
	var result []ReportEntry
	for _, r := range s.Reports {
		if r.Date == dateStr {
			uKey := r.UserJID
			if idx, exists := userMap[uKey]; exists {
				result[idx].Content = strings.TrimSpace(result[idx].Content) + "\n• " + strings.TrimSpace(strings.TrimPrefix(r.Content, "• "))
				result[idx].Timestamp = r.Timestamp
			} else {
				userMap[uKey] = len(result)
				result = append(result, r)
			}
		}
	}
	return result
}

// GetLeaderboard returns users sorted by points and streak
func (s *ReportsStore) GetLeaderboard() []*UserReportStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*UserReportStats, 0, len(s.Stats))
	for _, st := range s.Stats {
		list = append(list, st)
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].Points != list[j].Points {
			return list[i].Points > list[j].Points
		}
		return list[i].CurrentStreak > list[j].CurrentStreak
	})

	return list
}

// GetUserStats retrieves a single user's stats
func (s *ReportsStore) GetUserStats(userJID string) (*UserReportStats, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	st, exists := s.Stats[strings.TrimSpace(userJID)]
	return st, exists
}

// Target Groups Management
func (s *ReportsStore) AddReportGroup(groupJID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.TargetGroups[strings.TrimSpace(groupJID)] = true
	_ = s.saveReportsLocked()
}

func (s *ReportsStore) RemoveReportGroup(groupJID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.TargetGroups, strings.TrimSpace(groupJID))
	_ = s.saveReportsLocked()
}

func (s *ReportsStore) GetReportGroups() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var list []string
	for jid, active := range s.TargetGroups {
		if active {
			list = append(list, jid)
		}
	}
	return list
}

// Group Deadlines Management
func (s *ReportsStore) SetGroupDeadline(groupJID, targetDate, title string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cleanJID := strings.TrimSpace(groupJID)
	s.Deadlines[cleanJID] = &DeadlineConfig{
		GroupJID:    cleanJID,
		TargetDate:  strings.TrimSpace(targetDate),
		Title:       strings.TrimSpace(title),
		Enabled:     true,
		LastUpdated: time.Now(),
	}
	_ = s.saveDeadlinesLocked()
}

func (s *ReportsStore) GetGroupDeadline(groupJID string) (*DeadlineConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	d, ok := s.Deadlines[strings.TrimSpace(groupJID)]
	return d, ok && d.Enabled
}

func (s *ReportsStore) GetAllGroupDeadlines() []*DeadlineConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var list []*DeadlineConfig
	for _, d := range s.Deadlines {
		if d.Enabled {
			list = append(list, d)
		}
	}
	return list
}

func (s *ReportsStore) RemoveGroupDeadline(groupJID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.Deadlines, strings.TrimSpace(groupJID))
	_ = s.saveDeadlinesLocked()
}
