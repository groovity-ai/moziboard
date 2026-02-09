# Build Stage
FROM golang:alpine AS builder

WORKDIR /app

# Copy source code first
COPY . .

# Reset & Tidy modules inside container to avoid host conflicts
RUN rm -f go.mod go.sum
RUN go mod init moziboard-backend
# Force specific version compatible with Go 1.23 if needed, or let tidy resolve
RUN go get github.com/jackc/pgx/v5@v5.7.1
RUN go mod tidy

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Run Stage
FROM alpine:latest

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/main .

EXPOSE 8080

CMD ["./main"]
