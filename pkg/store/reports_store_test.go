package store

import (
	"os"
	"testing"

	"whatsapp-bot/pkg/config"
)

func TestReportsStore(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "reports_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	rs := InitReportsStore(tempDir)
	if rs == nil {
		t.Fatalf("InitReportsStore returned nil")
	}

	// 1. Submit First Report
	userJID := "1234567890@s.whatsapp.net"
	name := "Alice"
	content := "Completed Module 1 & solved 10 algorithm problems"

	entry, stats, points, err := rs.SubmitReport(userJID, name, "", content)
	if err != nil {
		t.Fatalf("SubmitReport failed: %v", err)
	}

	if points != 10 {
		t.Errorf("Expected 10 base points, got %d", points)
	}
	if stats.CurrentStreak != 1 {
		t.Errorf("Expected streak 1, got %d", stats.CurrentStreak)
	}
	if entry.Content != content {
		t.Errorf("Expected content '%s', got '%s'", content, entry.Content)
	}

	// Test Concatenation on Second Submission Today
	secondContent := "Built new dashboard analytics view"
	entry2, stats2, points2, err2 := rs.SubmitReport(userJID, name, "", secondContent)
	if err2 != nil {
		t.Fatalf("Second SubmitReport failed: %v", err2)
	}
	if points2 != 2 {
		t.Errorf("Expected 2 update bonus points, got %d", points2)
	}
	expectedConcat := content + "\n• " + secondContent
	if entry2.Content != expectedConcat {
		t.Errorf("Expected concatenated content:\n%q\ngot:\n%q", expectedConcat, entry2.Content)
	}
	if stats2.Points != 12 {
		t.Errorf("Expected total 12 points, got %d", stats2.Points)
	}

	// 2. Query Reports
	todayStr := config.Now().Format("2006-01-02")
	reports := rs.GetReportsForDate(todayStr)
	if len(reports) != 1 {
		t.Errorf("Expected 1 consolidated report for today, got %d", len(reports))
	}

	// 3. Leaderboard
	leaderboard := rs.GetLeaderboard()
	if len(leaderboard) != 1 || leaderboard[0].Points != 12 {
		t.Errorf("Leaderboard mismatch: %+v", leaderboard)
	}

	// 4. Target Groups
	groupJID := "120363000000000000@g.us"
	rs.AddReportGroup(groupJID)
	groups := rs.GetReportGroups()
	if len(groups) != 1 || groups[0] != groupJID {
		t.Errorf("Expected group '%s', got %+v", groupJID, groups)
	}

	rs.RemoveReportGroup(groupJID)
	if len(rs.GetReportGroups()) != 0 {
		t.Errorf("Expected 0 groups after removal")
	}

	// 5. Deadlines
	targetDate := config.Now().AddDate(0, 0, 14).Format("2006-01-02")
	rs.SetGroupDeadline(groupJID, targetDate, "Project Launch")
	deadline, ok := rs.GetGroupDeadline(groupJID)
	if !ok || deadline.Title != "Project Launch" {
		t.Errorf("Failed to retrieve deadline: %+v", deadline)
	}

	rs.RemoveGroupDeadline(groupJID)
	if _, ok := rs.GetGroupDeadline(groupJID); ok {
		t.Errorf("Expected deadline to be removed")
	}

	// Verify persistence reload
	rs2 := InitReportsStore(tempDir)
	stats2, found := rs2.GetUserStats(userJID)
	if !found || stats2.Points != 12 {
		t.Errorf("Persistence failed on reload: %+v", stats2)
	}
}
