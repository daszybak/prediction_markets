#!/bin/sh
set -eu

SERVICE="${SERVICE:-collector}"
CONFIG_SAMPLE="/app/configs/${SERVICE}/config.sample.yaml"
CONFIG_OUTPUT="/app/configs/${SERVICE}/config.yaml"

# Generate config from sample using envsubst
if [ -f "$CONFIG_SAMPLE" ]; then
    envsubst < "$CONFIG_SAMPLE" > "$CONFIG_OUTPUT"
fi

exec "$@"
