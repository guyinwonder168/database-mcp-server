# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.26.0

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY cmd ./cmd
COPY internal ./internal

ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" \
    go build -trimpath -ldflags="-s -w" -o /out/database-mcp-server ./cmd/server/main.go

FROM alpine:3.23

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S app && \
    adduser -S -G app -u 10001 appuser

WORKDIR /app

COPY --from=builder /out/database-mcp-server /usr/local/bin/database-mcp-server

RUN chown -R appuser:app /app

USER appuser

ENTRYPOINT ["/usr/local/bin/database-mcp-server"]
