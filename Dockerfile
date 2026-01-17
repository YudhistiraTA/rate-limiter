# -------------------------- Build stage --------------------------
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Download dependencies first (better caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with minimal size + good security defaults
RUN CGO_ENABLED=0 \
    GOOS=linux \
    go build -trimpath -ldflags="-s -w" \
    -o /rate-limiter main.go

# -------------------------- Final stage --------------------------
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /rate-limiter .
COPY .env .

# Use non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

ENV PORT=8080

EXPOSE 8080

CMD ["/app/rate-limiter"]