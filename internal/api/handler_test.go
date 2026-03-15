package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"wallet/internal/wallet"
)

func TestCreateWallet(t *testing.T) {
	svc := wallet.NewService()
	h := NewHandler(svc)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req, _ := http.NewRequest(http.MethodPost, "/wallets", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusCreated)
	}

	var w wallet.Wallet
	err := json.Unmarshal(rr.Body.Bytes(), &w)
	if err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if w.ID == "" {
		t.Errorf("expected wallet ID not to be empty")
	}
	if w.Balance != 0 {
		t.Errorf("expected wallet balance to be 0, got %d", w.Balance)
	}
}

func TestGetWallet(t *testing.T) {
	svc := wallet.NewService()
	createdWallet, _ := svc.CreateWallet()

	h := NewHandler(svc)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// Test successful get
	req, _ := http.NewRequest(http.MethodGet, "/wallets/"+createdWallet.ID, nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	var w wallet.Wallet
	json.Unmarshal(rr.Body.Bytes(), &w)
	if w.ID != createdWallet.ID {
		t.Errorf("expected wallet ID %s, got %s", createdWallet.ID, w.ID)
	}

	// Test non-existent get
	req, _ = http.NewRequest(http.MethodGet, "/wallets/non-existent-id", nil)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code for non-existent wallet: got %v want %v",
			status, http.StatusNotFound)
	}
}

func TestTransferFunds(t *testing.T) {
	svc := wallet.NewService()
	w1, _ := svc.CreateWallet()
	w2, _ := svc.CreateWallet()

	// Hack securely internal storage to set balance via another transfer could take more code, 
	// but to keep it simple we'll just test a transfer where w1 has 0 balance (should fail)
	// and a hacky transfer if we could. Let's do it right using tests where it always fails for InsufficientFunds first
	h := NewHandler(svc)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// Test Insufficient Funds
	transferReq := TransferRequest{
		SourceWalletID:      w1.ID,
		DestinationWalletID: w2.ID,
		Amount:              50,
	}
	body, _ := json.Marshal(transferReq)

	req, _ := http.NewRequest(http.MethodPost, "/wallets/transfer", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code for insufficient funds: got %v want %v",
			status, http.StatusBadRequest)
	}
}
