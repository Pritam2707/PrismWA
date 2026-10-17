package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/polymorfa/hypermeow"
	waProto "github.com/polymorfa/hypermeow/binary/proto"
	"github.com/polymorfa/hypermeow/types"
	"google.golang.org/protobuf/proto"

	"whatsapp-bot/pkg/commands"
	"whatsapp-bot/pkg/config"
	"whatsapp-bot/pkg/pfp"
	"whatsapp-bot/pkg/store"
)

// NextMidnight returns the next upcoming 00:00:00 instant in the given timezone
func NextMidnight(t time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = config.GetLocation()
	}
	tInLoc := t.In(loc)
	return time.Date(tInLoc.Year(), tInLoc.Month(), tInLoc.Day()+1, 0, 0, 0, 0, loc)
}

// StartDailyMidnightCron starts the background scheduler triggering at 00:00:00 every night (IST / configured timezone)
func StartDailyMidnightCron(client *whatsmeow.Client) {
	go func() {
		loc := config.GetLocation()
		for {
			now := time.Now().In(loc)
			// Next midnight (00:00:00) in configured timezone (IST)
			nextMidnight := NextMidnight(now, loc)
			delay := time.Until(nextMidnight)

			fmt.Printf("[DailyCron] Next midnight run scheduled in %v (at %s)\n", delay.Round(time.Second), nextMidnight.Format("2006-01-02 15:04:05 MST"))

			timer := time.NewTimer(delay)
			<-timer.C

			// Run midnight tasks
			RunMidnightTasks(client)

			// Sleep a few seconds to avoid re-triggering within the same minute
			time.Sleep(5 * time.Second)
		}
	}()
}

// RunMidnightTasks executes both status report AI summarization and group PFP updates
func RunMidnightTasks(client *whatsmeow.Client) {
	nowInLoc := config.Now()
	fmt.Printf("[DailyCron] 🕛 Midnight (%s) reached! Executing daily summary & group PFP updates...\n", nowInLoc.Format("2006-01-02 15:04:05 MST"))
	ctx := context.Background()

	// 1. Status Report AI Summary
	yesterday := nowInLoc.AddDate(0, 0, -1).Format("2006-01-02")
	targetGroups := store.GlobalReportsStore.GetReportGroups()

	if len(targetGroups) > 0 {
		fmt.Printf("[DailyCron] Generating summary for date: %s for %d target group(s)...\n", yesterday, len(targetGroups))
		summary, err := commands.GenerateDailyReportSummary(ctx, yesterday)
		if err == nil && summary != "" {
			for _, groupStr := range targetGroups {
				groupJID, err := types.ParseJID(groupStr)
				if err != nil {
					fmt.Printf("[DailyCron] Invalid group JID '%s': %v\n", groupStr, err)
					continue
				}

				if client != nil {
					msg := &waProto.Message{
						ExtendedTextMessage: &waProto.ExtendedTextMessage{
							Text: proto.String(summary),
						},
					}
					_, sendErr := client.SendMessage(ctx, groupJID, msg)
					if sendErr != nil {
						fmt.Printf("[DailyCron] Failed to send status summary to %s: %v\n", groupStr, sendErr)
					} else {
						fmt.Printf("[DailyCron] Successfully sent status summary to %s\n", groupStr)
					}
				}
			}
		} else {
			fmt.Printf("[DailyCron] Summary generation skipped or error: %v\n", err)
		}
	} else {
		fmt.Println("[DailyCron] No report groups configured. Skipping report broadcast.")
	}

	// 2. Group PFP Countdown Updates
	deadlines := store.GlobalReportsStore.GetAllGroupDeadlines()
	if len(deadlines) > 0 && client != nil {
		fmt.Printf("[DailyCron] Updating group profile pictures for %d group(s)...\n", len(deadlines))
		for _, d := range deadlines {
			groupJID, err := types.ParseJID(d.GroupJID)
			if err != nil {
				fmt.Printf("[DailyCron] Invalid deadline group JID '%s': %v\n", d.GroupJID, err)
				continue
			}

			jpegBytes, err := pfp.GenerateDeadlinePFP(d.Title, d.TargetDate)
			if err != nil {
				fmt.Printf("[DailyCron] Failed to generate PFP for group %s: %v\n", d.GroupJID, err)
				continue
			}

			picID, err := client.SetGroupPhoto(ctx, groupJID, jpegBytes)
			if err != nil {
				fmt.Printf("[DailyCron] Failed to set group photo for %s: %v\n", d.GroupJID, err)
			} else {
				fmt.Printf("[DailyCron] Successfully updated group PFP for %s (picID: %s)\n", d.GroupJID, picID)
			}
		}
	}
}
