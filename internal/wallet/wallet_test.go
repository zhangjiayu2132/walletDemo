package wallet

import (
	"sync"
	"testing"
	"github.com/google/uuid"
)

func TestCreateWallet(t *testing.T) {
	s := NewService()
	w, err := s.CreateWallet()

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if w.ID == "" {
		t.Errorf("expected non-empty wallet ID")
	}

	// verify UUID parsing
	if _, parseErr := uuid.Parse(w.ID); parseErr != nil {
		t.Errorf("expected valid UUID, got error: %v", parseErr)
	}

	if w.Balance != 0 {
		t.Errorf("expected initial balance of 0, got %d", w.Balance)
	}
}

func TestGetWallet(t *testing.T) {
	s := NewService()
	createdWallet, _ := s.CreateWallet()

	retrievedWallet, err := s.GetWallet(createdWallet.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if retrievedWallet.ID != createdWallet.ID {
		t.Errorf("expected ID %s, got %s", createdWallet.ID, retrievedWallet.ID)
	}

	_, err = s.GetWallet("non-existent-id")
	if err != ErrWalletNotFound {
		t.Errorf("expected ErrWalletNotFound, got %v", err)
	}
}

func TestTransferFunds(t *testing.T) {
	s := NewService()

	// Setup
	w1, _ := s.CreateWallet()
	w2, _ := s.CreateWallet()

	// Manually set balance for testing
	svc := s.(*inMemoryService)
	svc.wallets[w1.ID].Balance = 100

	// Test successful transfer
	err := s.TransferFunds(w1.ID, w2.ID, 50, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	getW1, _ := s.GetWallet(w1.ID)
	getW2, _ := s.GetWallet(w2.ID)

	if getW1.Balance != 50 {
		t.Errorf("expected source balance 50, got %d", getW1.Balance)
	}
	if getW2.Balance != 50 {
		t.Errorf("expected destination balance 50, got %d", getW2.Balance)
	}

	// Test insufficient funds
	err = s.TransferFunds(w1.ID, w2.ID, 100, "")
	if err != ErrInsufficientFunds {
		t.Errorf("expected ErrInsufficientFunds, got %v", err)
	}

	// Test negative amount
	err = s.TransferFunds(w1.ID, w2.ID, -10, "")
	if err == nil {
		t.Errorf("expected error for negative amount, got nil")
	}

	// Test same wallet transfer
	err = s.TransferFunds(w1.ID, w1.ID, 10, "")
	if err == nil {
		t.Errorf("expected error for transferring to same wallet, got nil")
	}

	// Test non-existent wallet
	err = s.TransferFunds(w1.ID, "non-existent", 10, "")
	if err != ErrWalletNotFound {
		t.Errorf("expected ErrWalletNotFound, got %v", err)
	}
}

func TestIdempotency(t *testing.T) {
	s := NewService()
	w1, _ := s.CreateWallet()
	w2, _ := s.CreateWallet()

	svc := s.(*inMemoryService)
	svc.wallets[w1.ID].Balance = 100

	idemKey := "tx-12345"
	err1 := s.TransferFunds(w1.ID, w2.ID, 50, idemKey)
	if err1 != nil {
		t.Fatalf("expected no error, got %v", err1)
	}

	// Second transfer with same idempotency key should succeed without changing balance
	err2 := s.TransferFunds(w1.ID, w2.ID, 20, idemKey)
	if err2 != nil {
		t.Fatalf("expected idempotency nil error, got %v", err2)
	}

	getW1, _ := s.GetWallet(w1.ID)
	getW2, _ := s.GetWallet(w2.ID)

	if getW1.Balance != 50 {
		t.Errorf("expected w1 balance 50, got %d", getW1.Balance)
	}
	if getW2.Balance != 50 {
		t.Errorf("expected w2 balance 50, got %d", getW2.Balance)
	}
}

func TestConcurrentTransfers(t *testing.T) {
	s := NewService()

	// Setup
	w1, _ := s.CreateWallet()
	w2, _ := s.CreateWallet()
	w3, _ := s.CreateWallet()

	svc := s.(*inMemoryService)
	svc.wallets[w1.ID].Balance = 1000
	svc.wallets[w2.ID].Balance = 1000
	svc.wallets[w3.ID].Balance = 1000

	var wg sync.WaitGroup
	numTransfers := 100

	// Transfer from w1 to w2
	for i := 0; i < numTransfers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.TransferFunds(w1.ID, w2.ID, 2, "")
		}()
	}

	// Transfer from w2 to w3
	for i := 0; i < numTransfers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.TransferFunds(w2.ID, w3.ID, 1, "")
		}()
	}

	wg.Wait()

	getW1, _ := s.GetWallet(w1.ID)
	getW2, _ := s.GetWallet(w2.ID)
	getW3, _ := s.GetWallet(w3.ID)

	// w1 sent 100*2 = 200 => 800
	if getW1.Balance != 800 {
		t.Errorf("expected w1 balance 800, got %d", getW1.Balance)
	}
	// w2 received 200, sent 100*1 = 100 => 1000 + 200 - 100 = 1100
	if getW2.Balance != 1100 {
		t.Errorf("expected w2 balance 1100, got %d", getW2.Balance)
	}
	// w3 received 100 => 1100
	if getW3.Balance != 1100 {
		t.Errorf("expected w3 balance 1100, got %d", getW3.Balance)
	}
}
