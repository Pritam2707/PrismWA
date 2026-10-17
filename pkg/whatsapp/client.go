package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/skip2/go-qrcode"
	"github.com/polymorfa/hypermeow"
	meowcallerLib "github.com/purpshell/meowcaller"
	"github.com/polymorfa/hypermeow/store/sqlstore"
	waLog "github.com/polymorfa/hypermeow/util/log"
	"github.com/rs/zerolog"
)

type WhatsAppService struct {
	Client          *whatsmeow.Client
	CallerClient    *meowcallerLib.Client
	container       *sqlstore.Container
	qrCodeStr       string
	connectionState string
	startTime       time.Time
	isCatchingUp    bool
	mu              sync.RWMutex
}

var (
	GlobalWA        *WhatsAppService
	callerLogMu     sync.RWMutex
	onCallerLogFunc func(level, msg string)
)

// SetCallerLogHandler registers a callback for low-level meowcaller VoIP events
func SetCallerLogHandler(fn func(level, msg string)) {
	callerLogMu.Lock()
	defer callerLogMu.Unlock()
	onCallerLogFunc = fn
}

type callerLogBridgeWriter struct{}

func (w *callerLogBridgeWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	var entry struct {
		Level   string `json:"level"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(p, &entry); err == nil && entry.Message != "" {
		lvl := strings.ToUpper(entry.Level)
		if lvl == "TRACE" || lvl == "DEBUG" {
			return len(p), nil
		}
		if lvl == "" {
			lvl = "INFO"
		}
		log.Printf("[VoIP-Core] %s: %s", lvl, entry.Message)

		callerLogMu.RLock()
		fn := onCallerLogFunc
		callerLogMu.RUnlock()
		if fn != nil {
			fn(lvl, "[VoIP-Core] "+entry.Message)
		}
	}
	return len(p), nil
}

func NewWhatsAppService(dbPath string) (*WhatsAppService, error) {
	if dbPath == "" {
		dbPath = filepath.Join("data", "prismbot.db")
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}

	dbLog := waLog.Stdout("Database", "WARN", true)
	dbURI := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(10000)", dbPath)

	log.Println("[WA] Opening SQLite container...")
	ctx := context.Background()
	container, err := sqlstore.New(ctx, "sqlite", dbURI, dbLog)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize sqlite container: %w", err)
	}

	log.Println("[WA] Getting first device...")
	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get device store: %w", err)
	}

	log.Println("[WA] Creating whatsmeow client...")
	clientLog := waLog.Stdout("WhatsApp", "WARN", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)

	log.Println("[WA] Creating meowcaller client...")
	bridge := &callerLogBridgeWriter{}
	callerLog := zerolog.New(bridge).Level(zerolog.InfoLevel).With().Timestamp().Str("subsystem", "meowcaller").Logger()
	callerClient := meowcallerLib.NewClient(client, meowcallerLib.WithLogger(callerLog))
	log.Println("[WA] meowcaller client created successfully")

	service := &WhatsAppService{
		Client:          client,
		CallerClient:    callerClient,
		container:       container,
		connectionState: "disconnected",
		startTime:       time.Now(),
		isCatchingUp:    false,
	}
	GlobalWA = service
	return service, nil
}

func (w *WhatsAppService) Start(ctx context.Context) error {
	w.mu.Lock()
	w.connectionState = "connecting"
	w.mu.Unlock()

	log.Printf("[WA] Checking device registration: Store.ID = %v", w.Client.Store.ID)
	if w.Client.Store.ID == nil {
		log.Println("[WA] No existing session found, requesting QR channel for pairing...")
		qrChan, _ := w.Client.GetQRChannel(ctx)
		log.Println("[WA] Connecting client for QR pairing...")
		if err := w.Client.Connect(); err != nil {
			return fmt.Errorf("failed to connect for QR pairing: %w", err)
		}
		log.Println("[WA] Connected for QR pairing, waiting for QR events...")

		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case evt, ok := <-qrChan:
					if !ok {
						return
					}
					if evt.Event == "code" {
						w.mu.Lock()
						w.qrCodeStr = evt.Code
						w.connectionState = "qr_pending"
						w.mu.Unlock()

						fmt.Println("WhatsApp pairing QR code:")
						if qrObj, err := qrcode.New(evt.Code, qrcode.Medium); err == nil {
							fmt.Println(qrObj.ToSmallString(false))
						}
					} else {
						if evt.Event == "success" {
							w.mu.Lock()
							w.qrCodeStr = ""
							w.connectionState = "open"
							w.mu.Unlock()
							fmt.Println("WhatsApp authentication successful.")
						}
					}
				}
			}
		}()
	} else {
		if err := w.Client.Connect(); err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		w.mu.Lock()
		w.connectionState = "open"
		w.mu.Unlock()
		fmt.Println("WhatsApp connected to existing session.")
	}

	return nil
}

func (w *WhatsAppService) GetQRCode() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.qrCodeStr
}

func (w *WhatsAppService) GetConnectionState() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.connectionState
}

func (w *WhatsAppService) SetConnectionState(state string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.connectionState = state
}

func (w *WhatsAppService) Disconnect() {
	if w.Client != nil {
		w.Client.Disconnect()
	}
}

func (w *WhatsAppService) GetStartTime() time.Time {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.startTime
}

func (w *WhatsAppService) IsCatchingUp() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.isCatchingUp
}

func (w *WhatsAppService) SetCatchingUp(catchingUp bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.isCatchingUp = catchingUp
}

