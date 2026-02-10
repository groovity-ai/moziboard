FROM golang:alpine AS builder

WORKDIR /app

# Copy source code including go.mod
COPY . .

# Ensure clean slate for deps if needed, but prefer using existing go.mod
# RUN rm -f go.sum

# Download dependencies
RUN go mod tidy

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/main .

EXPOSE 8080
CMD ["./main"]
