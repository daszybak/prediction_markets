# Build stage
FROM golang:1.25.5-alpine3.23 AS builder

# Install build dependencies
RUN apk add --no-cache git make curl

# Install Go tools for linting/formatting
RUN go install mvdan.cc/gofumpt@latest && \
    go install golang.org/x/tools/cmd/goimports@latest && \
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Install just (direct binary download)
RUN curl -L https://github.com/casey/just/releases/download/1.36.0/just-1.36.0-x86_64-unknown-linux-musl.tar.gz | tar xz -C /usr/local/bin just

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Default command: run tests
CMD ["make", "test"]

# Development stage with hot reload
FROM builder AS dev

RUN apk add --no-cache gettext
RUN go install github.com/air-verse/air@latest
RUN mkdir -p /app/tmp

# Note: Volume mount .:/app will override /app, so entrypoint path
# must match the local filesystem structure (scripts/entrypoint.dev.sh)
ENV SERVICE=collector
ENTRYPOINT ["/app/scripts/entrypoint.dev.sh"]

# Production build stage
FROM builder AS build
ARG SERVICE=collector
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/target/${SERVICE} ./cmd/${SERVICE}

# Migration stage (runs before services start)
FROM alpine:3.23 AS migrate

RUN apk add --no-cache postgresql-client curl

# Install golang-migrate
RUN curl -L https://github.com/golang-migrate/migrate/releases/download/v4.18.1/migrate.linux-amd64.tar.gz | tar xz \
    && mv migrate /usr/local/bin/migrate

WORKDIR /app
COPY db/migrations /app/db/migrations
COPY scripts/migrate.sh /app/migrate.sh
RUN chmod +x /app/migrate.sh

CMD ["/app/migrate.sh"]

# Runtime stage (minimal image)
FROM alpine:3.23 AS runtime

RUN apk add --no-cache ca-certificates gettext

WORKDIR /app

ARG SERVICE=collector
COPY --from=build /app/target/${SERVICE} /app/service
COPY configs/${SERVICE}/config.sample.yaml /app/configs/${SERVICE}/config.sample.yaml
COPY scripts/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["/app/service"]
