package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

const baseURL = "http://localhost:8080"

type Wallet struct {
	ID      string `json:"id"`
	Balance int64  `json:"balance"`
}

type TransferRequest struct {
	SourceWalletID      string `json:"source_wallet"`
	DestinationWalletID string `json:"destination_wallet"`
	Amount              int64  `json:"amount"`
}

func main() {
	fmt.Println("Starting load test against", baseURL)

	// Check if the server is up
	_, err := http.Get(baseURL + "/wallets")
	// GET /wallets will return 405 Method Not Allowed, but that proves the server is up
	if err != nil {
		log.Fatalf("Server does not appear to be running at %s: %v\nRun 'go run ./cmd/server' first.", baseURL, err)
	}

	// 1. Create initial wallets for testing
	w1 := createWallet()
	w2 := createWallet()
	fmt.Printf("Created test wallets: %s, %s\n", w1.ID, w2.ID)

	// 2. Run concurrent load test
	concurrency := 100
	requestsPerWorker := 50

	start := time.Now()
	var wg sync.WaitGroup

	var totalRequests int32
	var successResponses int32
	var errorResponses int32

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < requestsPerWorker; j++ {
				atomic.AddInt32(&totalRequests, 1)

				var err error
				// Mix of operations to stress different locks
				if j%3 == 0 {
					err = getWallet(w1.ID)
				} else if j%3 == 1 {
					// This will return 400 Bad Request because balance is 0,
					// but it still successfully hits the server and tests the write lock
					err = transfer(w1.ID, w2.ID, 10, true)
				} else {
					_, err = createWalletErr()
				}

				if err == nil {
					atomic.AddInt32(&successResponses, 1)
				} else {
					atomic.AddInt32(&errorResponses, 1)
				}
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	fmt.Println("\n--- Load Test Results ---")
	fmt.Printf("Duration: %v\n", duration)
	fmt.Printf("Total Requests: %d\n", totalRequests)
	fmt.Printf("Successful Responses: %d\n", successResponses) // Includes expected 400s
	fmt.Printf("Failed Responses (Connection Errors): %d\n", errorResponses)
	fmt.Printf("Requests/sec: %.2f\n", float64(totalRequests)/duration.Seconds())
}

func createWallet() Wallet {
	w, err := createWalletErr()
	if err != nil {
		log.Fatalf("Failed to create wallet: %v", err)
	}
	return w
}

func createWalletErr() (Wallet, error) {
	var w Wallet
	resp, err := http.Post(baseURL+"/wallets", "application/json", nil)
	if err != nil {
		return w, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return w, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	json.NewDecoder(resp.Body).Decode(&w)
	return w, nil
}

func getWallet(id string) error {
	resp, err := http.Get(baseURL + "/wallets/" + id)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	return nil
}

func transfer(source, dest string, amount int64, allow400 bool) error {
	req := TransferRequest{
		SourceWalletID:      source,
		DestinationWalletID: dest,
		Amount:              amount,
	}
	data, _ := json.Marshal(req)
	resp, err := http.Post(baseURL+"/wallets/transfer", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		return nil
	}
	if allow400 && resp.StatusCode == http.StatusBadRequest {
		return nil // Expected if funds are insufficient
	}

	return fmt.Errorf("unexpected status: %d", resp.StatusCode)
}
