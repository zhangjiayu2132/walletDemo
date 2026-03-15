package wallet

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

var ErrWalletNotFound = errors.New("wallet not found")

var ErrInsufficientFunds = errors.New("insufficient funds")

var ErrIdempotencyConflict = errors.New("request is currently processing")

var ErrIdempotencyMismatch = errors.New("idempotency key already used with different parameters")

type TransferStatus string

const (
	StatusProcessing TransferStatus = "processing"
	StatusSuccess    TransferStatus = "success"
)

type idempotencyRecord struct {
	status  TransferStatus
	reqHash string
}

type Wallet struct {
	ID      string
	Balance int64
	mu      sync.Mutex
}

type Service interface {
	CreateWallet() (*Wallet, error)
	GetWallet(id string) (*Wallet, error)
	TransferFunds(sourceID, destID string, amount int64, idempotencyKey string) error
}

type inMemoryService struct {
	mu                 sync.RWMutex
	wallets            map[string]*Wallet
	processedTransfers map[string]*idempotencyRecord
}

func NewService() Service {
	return &inMemoryService{
		wallets:            make(map[string]*Wallet),
		processedTransfers: make(map[string]*idempotencyRecord),
	}
}

func computeRequestHash(sourceID, destID string, amount int64) string {
	data := fmt.Sprintf("%s:%s:%d", sourceID, destID, amount)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
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

	reqHash := computeRequestHash(sourceID, destID, amount)

	s.mu.Lock()
	if idempotencyKey != "" {
		if record, exists := s.processedTransfers[idempotencyKey]; exists {
			s.mu.Unlock()
			if record.status == StatusProcessing {
				return ErrIdempotencyConflict
			}
			if record.reqHash != reqHash {
				return ErrIdempotencyMismatch
			}
			return nil
		}
		s.processedTransfers[idempotencyKey] = &idempotencyRecord{
			status:  StatusProcessing,
			reqHash: reqHash,
		}
	}

	defer func() {
		if idempotencyKey != "" {
			s.mu.Lock()
			if err != nil {
				delete(s.processedTransfers, idempotencyKey)
			} else {
				s.processedTransfers[idempotencyKey].status = StatusSuccess
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

	source.Balance -= amount
	dest.Balance += amount

	return nil
}
