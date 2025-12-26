# Build stage
FROM golang:1.25.5-alpine3.23 AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Install Go tools for linting/formatting
RUN go install mvdan.cc/gofumpt@latest && \
    go install golang.org/x/tools/cmd/goimports@latest && \
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Default command: run tests
CMD ["make", "test"]

# Production build stage
FROM builder AS build
RUN make build

# Runtime stage (minimal image)
FROM alpine:3.23 AS runtime

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=build /app/target/collector /app/collector
COPY config.sample.yaml /app/config.yaml

ENTRYPOINT ["/app/collector"]
