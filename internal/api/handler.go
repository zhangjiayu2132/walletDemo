package api

import (
	"encoding/json"
	"math"
	"net/http"
	"wallet/internal/wallet"
)

// Handler handles HTTP requests for the wallet service.
type Handler struct {
	service wallet.Service
}

// NewHandler creates a new Handler with the given wallet service.
func NewHandler(service wallet.Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers the HTTP routes to the given ServeMux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/wallets", h.handleWallets)
	mux.HandleFunc("/wallets/", h.handleWalletByID) // catches /wallets/{id}
	mux.HandleFunc("/wallets/transfer", h.handleTransfer)
}

// handleWallets delegates to specific methods based on the HTTP method.
func (h *Handler) handleWallets(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		h.createWallet(w, r)
		return
	}
	respondWithError(w, http.StatusMethodNotAllowed, "method not allowed")
}

// WalletResponse represents the API response for a wallet.
type WalletResponse struct {
	ID      string  `json:"id"`
	Balance float64 `json:"balance"` // output in Yuan
}

// handleWalletByID handles GET /wallets/{id}
func (h *Handler) handleWalletByID(w http.ResponseWriter, r *http.Request) {
	// Only allow GET
	if r.Method != http.MethodGet {
		respondWithError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract ID from path. Len("/wallets/") is 9
	path := r.URL.Path
	if len(path) <= 9 {
		respondWithError(w, http.StatusBadRequest, "wallet ID is required")
		return
	}
	id := path[9:]

	// Only process id path
	if id == "transfer" {
		respondWithError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	wlt, err := h.service.GetWallet(id)
	if err != nil {
		if err == wallet.ErrWalletNotFound {
			respondWithError(w, http.StatusNotFound, "wallet not found")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	resp := WalletResponse{
		ID:      wlt.ID,
		Balance: float64(wlt.Balance) / 100.0,
	}
	respondWithJSON(w, http.StatusOK, resp)
}

// createWallet handles POST /wallets
func (h *Handler) createWallet(w http.ResponseWriter, r *http.Request) {
	wlt, err := h.service.CreateWallet()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to create wallet")
		return
	}

	resp := WalletResponse{
		ID:      wlt.ID,
		Balance: float64(wlt.Balance) / 100.0,
	}
	respondWithJSON(w, http.StatusCreated, resp)
}

// TransferRequest represents the payload for transferring funds.
type TransferRequest struct {
	SourceWalletID      string  `json:"source_wallet"`
	DestinationWalletID string  `json:"destination_wallet"`
	Amount              float64 `json:"amount"` // input in Yuan
}

// handleTransfer handles POST /wallets/transfer
func (h *Handler) handleTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req TransferRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	if req.SourceWalletID == "" || req.DestinationWalletID == "" {
		respondWithError(w, http.StatusBadRequest, "source and destination wallets are required")
		return
	}
	if req.Amount <= 0 {
		respondWithError(w, http.StatusBadRequest, "transfer amount must be positive")
		return
	}

	// Safely convert floating point Yuan to integer representation in Cents
	amountCents := int64(math.Round(req.Amount * 100))

	if amountCents <= 0 {
		respondWithError(w, http.StatusBadRequest, "transfer amount must be at least 0.01")
		return
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")
	err := h.service.TransferFunds(req.SourceWalletID, req.DestinationWalletID, amountCents, idempotencyKey)
	if err != nil {
		switch err {
		case wallet.ErrWalletNotFound:
			respondWithError(w, http.StatusNotFound, "one or both wallets not found")
		case wallet.ErrInsufficientFunds:
			respondWithError(w, http.StatusBadRequest, "insufficient funds")
		case wallet.ErrIdempotencyConflict:
			respondWithError(w, http.StatusConflict, "request is already processing")
		default:
			if err.Error() == "cannot transfer to the same wallet" {
				respondWithError(w, http.StatusBadRequest, err.Error())
			} else {
				respondWithError(w, http.StatusInternalServerError, "internal server error")
			}
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}

// respondWithError sends an error message as JSON.
func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

// respondWithJSON sends a JSON response.
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}
