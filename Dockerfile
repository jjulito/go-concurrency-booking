# Build Stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/api/main.go

# Production Stage
FROM alpine:3.18

WORKDIR /root/

COPY --from=builder /app/main .
COPY --from=builder /app/config ./config
# Copy migrations if needed for runtime migration (better to use init container or separate job)
COPY --from=builder /app/scripts ./scripts

EXPOSE 8080

CMD ["./main"]
