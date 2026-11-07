# Stage 1: Build
FROM golang:1.22-alpine3.21 AS builder

WORKDIR /app
COPY go.mod ./
# COPY go.sum ./
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o edge-frontend .

# Stage 2: Minimal Hardened Runtime
FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata && apk --no-cache upgrade

WORKDIR /app
COPY --from=builder /app/edge-frontend .

EXPOSE 8080

USER 10001:10001

ENTRYPOINT ["/app/edge-frontend"]
