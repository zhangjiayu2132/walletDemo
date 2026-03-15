# Go Wallet Service

A simple REST API wallet service built in Go with thread-safe in-memory storage.

## Features

- Create a new wallet with an initial balance of 0
- Retrieve a wallet's current balance
- Transfer funds between wallets safely
- High-concurrency support using read-write mutex locks

## Prerequisites

- [Go 1.20+](https://golang.org/dl/)

## Getting Started

### 1. Run the Server

You can run the server directly using `go run`:

```bash
go run ./cmd/server
```

The server will start on port `8080` by default. You can override this using the `PORT` environment variable:

```bash
PORT=9000 go run ./cmd/server
```

### 2. Run the Tests

To run all unit and integration tests:

```bash
go test -v ./...
```

*(Optional)* To run tests with the race detector enabled to ensure concurrent safety (requires CGO enabled):

```bash
go test -v -race ./...
```

### 3. Run with Docker

You can easily run the application using Docker Compose without needing Go installed locally:

```bash
docker-compose up --build -d
```

The server will be available on `http://localhost:8080`.

### 4. Run Load Test

To test the concurrent safety of the service, start the server and run the load test script:

```bash
go run ./scripts/load.go
```

## API Endpoints

### 1. Create Wallet
**POST** `/wallets`

Creates a new wallet with an initial balance of 0.

**Response:** `201 Created`
```json
{
  "id": "eJ1p2Mz8yL0QxWvA",
  "balance": 0
}
```

### 2. Get Wallet
**GET** `/wallets/{wallet_id}`

Retrieves the wallet ID and current balance.

**Response:** `200 OK`
```json
{
  "id": "eJ1p2Mz8yL0QxWvA",
  "balance": 100
}
```

**Errors:** `404 Not Found` if the wallet does not exist.

### 3. Transfer Funds
**POST** `/wallets/transfer`

Transfers a specified amount from one wallet to another.

**Request Body:**
```json
{
  "source_wallet": "eJ1p2Mz8yL0QxWvA",
  "destination_wallet": "kL9q1Nz7xM2PzWuC",
  "amount": 50
}
```

**Response:** `200 OK`

**Errors:**
- `400 Bad Request` if payload is invalid, amount is negative, or balance is insufficient.
- `404 Not Found` if one or both wallets do not exist.

## Project Structure

- `cmd/server/`: Contains the main entry point to start the HTTP server.
- `internal/api/`: Contains the HTTP handlers and routing logic.
- `internal/wallet/`: Contains the core business logic, `Wallet` struct, and in-memory storage implementation.

## Trade-offs and Design Decisions

- **In-Memory Storage**: Used a standard Go `map` protected by `sync.RWMutex` which allows concurrent reads (`GetWallet`) while ensuring exclusive access for writes (`CreateWallet`, `TransferFunds`).
- **Standard Library Only**: Relied solely on `net/http` to minimize dependencies, matching the simplicity requirement.
- **Integer Amounts**: Kept the balance as `int64` representing the smallest unit of currency (like cents) to avoid floating-point inaccuracies.
