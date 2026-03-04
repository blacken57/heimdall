# Stage 1: Build
FROM golang:1.24-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o /heimdall ./cmd/heimdall

# Stage 2: Runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
RUN addgroup -g 1001 heimdall && adduser -u 1001 -G heimdall -s /bin/sh -D heimdall
RUN mkdir -p /data && chown heimdall:heimdall /data
COPY --from=builder /heimdall /usr/local/bin/heimdall
COPY --chown=heimdall:heimdall web/ /app/web/
WORKDIR /app
USER heimdall
EXPOSE 8080
ENV DB_PATH=/data/heimdall.db
HEALTHCHECK --interval=30s --timeout=5s CMD wget -qO- http://localhost:8080/health || exit 1
ENTRYPOINT ["/usr/local/bin/heimdall"]
