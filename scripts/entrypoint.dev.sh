#!/bin/sh
set -eu

SERVICE="${SERVICE:-collector}"
CONFIG_SAMPLE="/app/configs/${SERVICE}/config.sample.yaml"
CONFIG_OUTPUT="/app/configs/${SERVICE}/config.yaml"

# Generate config from sample using envsubst (if config doesn't exist)
if [ ! -f "$CONFIG_OUTPUT" ] && [ -f "$CONFIG_SAMPLE" ]; then
    echo "Generating config from sample..."
    envsubst < "$CONFIG_SAMPLE" > "$CONFIG_OUTPUT"
fi

# Run air with service-specific config
exec air -c "configs/${SERVICE}/air.toml"
