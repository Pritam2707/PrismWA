package dashboard

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"whatsapp-bot/pkg/meowcaller"
)

func TestDashboardMeowCallerEndpoints(t *testing.T) {
	// 1. Test Status
	reqStatus := httptest.NewRequest(http.MethodGet, "/api/meowcaller/status", nil)
	rrStatus := httptest.NewRecorder()
	handleMeowCallerStatus(rrStatus, reqStatus)
	if rrStatus.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from status endpoint, got %d", rrStatus.Code)
	}

	// 2. Test Toggle (Enable for 45 minutes)
	toggleBody, _ := json.Marshal(map[string]interface{}{
		"enabled":         true,
		"durationMinutes": 45,
	})
	reqToggle := httptest.NewRequest(http.MethodPost, "/api/meowcaller/toggle", bytes.NewBuffer(toggleBody))
	rrToggle := httptest.NewRecorder()
	handleMeowCallerToggle(rrToggle, reqToggle)
	if rrToggle.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from toggle endpoint, got %d", rrToggle.Code)
	}

	if !meowcaller.GlobalManager.IsEnabled() {
		t.Errorf("expected MeowCaller to be enabled after toggle POST")
	}

	// 3. Test AutoAccept
	autoAcceptBody, _ := json.Marshal(map[string]interface{}{
		"autoAccept": true,
	})
	reqAutoAccept := httptest.NewRequest(http.MethodPost, "/api/meowcaller/auto-accept", bytes.NewBuffer(autoAcceptBody))
	rrAutoAccept := httptest.NewRecorder()
	handleMeowCallerAutoAccept(rrAutoAccept, reqAutoAccept)
	if rrAutoAccept.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from auto-accept endpoint, got %d", rrAutoAccept.Code)
	}

	status := meowcaller.GlobalManager.GetStatus()
	if !status.AutoAcceptCalls {
		t.Errorf("expected autoAcceptCalls to be true")
	}

	// 4. Test Party Control: Stop
	ctrlBody, _ := json.Marshal(map[string]interface{}{
		"action": "stop",
	})
	reqCtrl := httptest.NewRequest(http.MethodPost, "/api/meowcaller/party/control", bytes.NewBuffer(ctrlBody))
	rrCtrl := httptest.NewRecorder()
	handleMeowCallerPartyControl(rrCtrl, reqCtrl)
	if rrCtrl.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from control endpoint, got %d", rrCtrl.Code)
	}

	// 5. Test Disable
	toggleOffBody, _ := json.Marshal(map[string]interface{}{
		"enabled":         false,
		"durationMinutes": 0,
	})
	reqToggleOff := httptest.NewRequest(http.MethodPost, "/api/meowcaller/toggle", bytes.NewBuffer(toggleOffBody))
	rrToggleOff := httptest.NewRecorder()
	handleMeowCallerToggle(rrToggleOff, reqToggleOff)
	if rrToggleOff.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from toggle off, got %d", rrToggleOff.Code)
	}
	if meowcaller.GlobalManager.IsEnabled() {
		t.Errorf("expected MeowCaller to be disabled")
	}
}
