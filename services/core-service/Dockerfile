# Stage 1: Build
FROM golang:1.22-alpine3.21 AS builder

RUN apk --no-cache add git ca-certificates tzdata

WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download || true

COPY . .
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o core-service .

# Stage 2: Minimal Hardened Runtime
FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata && apk --no-cache upgrade

WORKDIR /app
COPY --from=builder /app/core-service .

EXPOSE 8082

USER 10001:10001

ENTRYPOINT ["/app/core-service"]
