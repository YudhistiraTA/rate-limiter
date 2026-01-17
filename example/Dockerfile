FROM golang:1.23-alpine AS builder

ENV PORT=8000

WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o backend main.go

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/backend .
USER 1000
EXPOSE 8000
CMD ["./backend"]