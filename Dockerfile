FROM golang:1.20-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o wallet-server ./cmd/server

# Final stage
FROM alpine:latest

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/wallet-server .

# Expose port
EXPOSE 8080

# Run the executable
CMD ["./wallet-server"]
