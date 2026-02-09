FROM golang:1.23-alpine

WORKDIR /app

COPY go.mod ./
# RUN go mod download # (Commented out because go.sum is missing)

COPY . .

# Run tidy inside container to ensure deps are resolved
RUN go mod tidy
RUN go build -o main .

EXPOSE 8080

CMD ["./main"]
