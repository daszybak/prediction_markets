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

# Generate config from sample using envsubst
if [ -f "$CONFIG_SAMPLE" ]; then
    envsubst < "$CONFIG_SAMPLE" > "$CONFIG_OUTPUT"
fi

exec "$@"
