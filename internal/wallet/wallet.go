package wallet

import (
	"errors"
	"sync"

	"github.com/google/uuid"
)

// ErrWalletNotFound is returned when a wallet is not found.
var ErrWalletNotFound = errors.New("wallet not found")

// ErrInsufficientFunds is returned when a wallet doesn't have enough balance for a transfer.
var ErrInsufficientFunds = errors.New("insufficient funds")

// ErrIdempotencyConflict is returned when a concurrent request with the same idempotency key is processing.
var ErrIdempotencyConflict = errors.New("request is currently processing")

type TransferStatus string

const (
	StatusProcessing TransferStatus = "processing"
	StatusSuccess    TransferStatus = "success"
)

// Wallet represents a digital wallet.
type Wallet struct {
	ID      string
	Balance int64
	mu      sync.Mutex
}

// Service defines the operations for the wallet service.
type Service interface {
	CreateWallet() (*Wallet, error)
	GetWallet(id string) (*Wallet, error)
	TransferFunds(sourceID, destID string, amount int64, idempotencyKey string) error
}

// inMemoryService implements the Service interface using a map and a RWMutex for thread safety.
type inMemoryService struct {
	mu                 sync.RWMutex
	wallets            map[string]*Wallet
	processedTransfers map[string]TransferStatus
}

// NewService creates a new instance of the wallet service.
func NewService() Service {
	return &inMemoryService{
		wallets:            make(map[string]*Wallet),
		processedTransfers: make(map[string]TransferStatus),
	}
}

// CreateWallet creates a new wallet with an initial balance of 0.
func (s *inMemoryService) CreateWallet() (*Wallet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := uuid.New().String()
	wallet := &Wallet{
		ID:      id,
		Balance: 0,
	}
	s.wallets[id] = wallet
	return wallet, nil
}

// GetWallet retrieves a wallet by its ID.
func (s *inMemoryService) GetWallet(id string) (*Wallet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	wallet, exists := s.wallets[id]
	if !exists {
		return nil, ErrWalletNotFound
	}

	wallet.mu.Lock()
	balance := wallet.Balance
	wallet.mu.Unlock()

	return &Wallet{
		ID:      wallet.ID,
		Balance: balance,
	}, nil
}

// TransferFunds transfers a specified amount from one wallet to another.
func (s *inMemoryService) TransferFunds(sourceID, destID string, amount int64, idempotencyKey string) (err error) {
	if sourceID == destID {
		return errors.New("cannot transfer to the same wallet")
	}
	if amount <= 0 {
		return errors.New("transfer amount must be positive")
	}

	s.mu.Lock()
	if idempotencyKey != "" {
		if status, exists := s.processedTransfers[idempotencyKey]; exists {
			s.mu.Unlock()
			if status == StatusProcessing {
				return ErrIdempotencyConflict
			}
			return nil // Idempotency check: already processed successfully
		}
		s.processedTransfers[idempotencyKey] = StatusProcessing
	}

	defer func() {
		if idempotencyKey != "" {
			s.mu.Lock()
			if err != nil {
				delete(s.processedTransfers, idempotencyKey)
			} else {
				s.processedTransfers[idempotencyKey] = StatusSuccess
			}
			s.mu.Unlock()
		}
	}()

	source, ok := s.wallets[sourceID]
	if !ok {
		s.mu.Unlock()
		return ErrWalletNotFound
	}

	dest, ok := s.wallets[destID]
	if !ok {
		s.mu.Unlock()
		return ErrWalletNotFound
	}
	s.mu.Unlock()

	// Ordered locking to prevent deadlocks
	if sourceID < destID {
		source.mu.Lock()
		dest.mu.Lock()
	} else {
		dest.mu.Lock()
		source.mu.Lock()
	}
	defer func() {
		source.mu.Unlock()
		dest.mu.Unlock()
	}()

	if source.Balance < amount {
		return ErrInsufficientFunds
	}

	// Perform the transfer
	source.Balance -= amount
	dest.Balance += amount

	return nil
}
