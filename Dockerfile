# Build stage
FROM golang:1.25.5-alpine3.23 AS builder

# Install build dependencies
RUN apk add --no-cache git make curl

# Install Go tools for linting/formatting
RUN go install mvdan.cc/gofumpt@latest && \
    go install golang.org/x/tools/cmd/goimports@latest && \
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Install just
RUN curl --proto '=https' --tlsv1.2 -sSf https://just.systems/install.sh | sh -s -- --to /usr/local/bin

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

RUN go install github.com/air-verse/air@latest
RUN mkdir -p /app/tmp

# SERVICE env var selects which service to build/run
ENV SERVICE=collector
CMD ["air"]

# Production build stage
FROM builder AS build
ARG SERVICE=collector
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/target/${SERVICE} ./cmd/${SERVICE}

# Runtime stage (minimal image)
FROM alpine:3.23 AS runtime

RUN apk add --no-cache ca-certificates

WORKDIR /app

ARG SERVICE=collector
COPY --from=build /app/target/${SERVICE} /app/service
COPY config.sample.yaml /app/config.yaml

ENTRYPOINT ["/app/service"]
