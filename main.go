package main

import (
	_ "whatsapp-bot/pkg/envfix"

	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"whatsapp-bot/pkg/commands"
	"whatsapp-bot/pkg/config"
	"whatsapp-bot/pkg/dashboard"
	"whatsapp-bot/pkg/gemini"
	"whatsapp-bot/pkg/meowcaller"
	"whatsapp-bot/pkg/pool"
	"whatsapp-bot/pkg/scheduler"
	"whatsapp-bot/pkg/store"
	"whatsapp-bot/pkg/whatsapp"
)

func main() {
	log.Println("starting PrismBot")

	cfg := config.LoadConfig()

	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	msgStore := store.InitMessageStore(rootCtx, cfg.MaxCacheMessages)
	_ = store.InitPersistentStore("data")
	_ = store.InitReportsStore("data")

	aiClient := gemini.NewClient(cfg.GeminiAPIKey, cfg.GeminiModel)
	aiClient.ConfigureOpenRouter(cfg.OpenRouterAPIKey, cfg.OpenRouterModel, cfg.OpenRouterURL)
	aiClient.ConfigureGroq(cfg.GroqAPIKey, cfg.GroqModel, cfg.GroqURL)
	if err := aiClient.SetProvider(cfg.AIProvider); err != nil {
		log.Printf("AI Provider initialization note: %v (active provider: %s)", err, aiClient.GetProvider())
	} else {
		log.Printf("AI Provider initialized: %s (model: %s)", aiClient.GetProvider(), aiClient.GetActiveModel())
	}
	if cfg.GeminiAPIKey == "" && cfg.OpenRouterAPIKey == "" && cfg.GroqAPIKey == "" {
		log.Println("warning: No AI API keys configured in .env (GEMINI_API_KEY, OPENROUTER_API_KEY, or GROQ_API_KEY)")
	}

	workerPool := pool.NewWorkerPool(cfg.WorkerPoolSize, 1000)

	// Command Registrations
	registry := commands.InitRegistry()
	commands.RegisterSystemCommands(registry)
	commands.RegisterAICommands(registry)
	commands.RegisterToolsCommands(registry)
	commands.RegisterAdminCommands(registry)
	commands.RegisterSchedulerCommands(registry)
	commands.RegisterMediaCommands(registry)
	commands.RegisterDeletedCommands(registry)
	commands.RegisterStatusReportAndDeadlineCommands(registry)
	commands.RegisterPartyCommands(registry)

	if err := dashboard.StartDashboardServer(cfg.DashboardPort); err != nil {
		log.Printf("dashboard server failed to start: %v", err)
	}

	log.Println("Initializing WhatsApp service...")
	whatsapp.SetCallerLogHandler(dashboard.LogEvent)
	waService, err := whatsapp.NewWhatsAppService("data/prismbot.db")
	if err != nil {
		log.Fatalf("failed to initialize WhatsApp service: %v", err)
	}
	log.Println("WhatsApp service created successfully")

	meowcaller.GlobalManager.Init(waService.CallerClient, waService.Client)
	meowcaller.GlobalManager.SetLogHandler(dashboard.LogEvent)

	waService.RegisterEventHandler(registry)

	log.Println("Starting WhatsApp connection...")
	if err := waService.Start(rootCtx); err != nil {
		log.Fatalf("failed to start WhatsApp connection: %v", err)
	}
	log.Println("WhatsApp Start completed successfully")

	// Restore active reminder timers
	commands.RestoreReminders(waService.Client)

	// Start daily midnight cron for status reports and PFP countdown updates
	scheduler.StartDailyMidnightCron(waService.Client)

	dashboard.LogEvent("INFO", "PrismBot started successfully with all commands")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	log.Println("termination signal received, stopping...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	rootCancel()
	waService.Disconnect()
	workerPool.Shutdown()
	msgStore.Close()
	_ = dashboard.StopDashboardServer(shutdownCtx)

	log.Println("shutdown complete")
}
