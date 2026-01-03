#!/bin/sh
set -eu

# Required environment variables
: "${SERVICE:?SERVICE is required}"
: "${POSTGRES_HOST:?POSTGRES_HOST is required}"
: "${POSTGRES_PORT:?POSTGRES_PORT is required}"
: "${POSTGRES_USER:?POSTGRES_USER is required}"
: "${POSTGRES_PASSWORD:?POSTGRES_PASSWORD is required}"
: "${POSTGRES_DB:?POSTGRES_DB is required}"
: "${POSTGRES_SSLMODE:?POSTGRES_SSLMODE is required}"

CONFIG_SAMPLE="/app/configs/${SERVICE}/config.sample.yaml"
CONFIG_OUTPUT="/app/configs/${SERVICE}/config.yaml"

# Generate config from sample using envsubst (if config doesn't exist)
if [ ! -f "$CONFIG_OUTPUT" ] && [ -f "$CONFIG_SAMPLE" ]; then
    echo "Generating config from sample..."
    envsubst < "$CONFIG_SAMPLE" > "$CONFIG_OUTPUT"
fi

# Run air with service-specific config
exec air -c "configs/${SERVICE}/air.toml"
