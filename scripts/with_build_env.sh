#!/usr/bin/env bash
set -euo pipefail

# Runs a command inside the Docker build environment.
#
# Usage:
#   ./scripts/with_build_env.sh <command> [args...]
#
# Examples:
#   ./scripts/with_build_env.sh make test
#   ./scripts/with_build_env.sh go test -v ./...
#   ./scripts/with_build_env.sh bash  # interactive shell

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

IMAGE_NAME="prediction_markets-build"
DOCKERFILE="${PROJECT_ROOT}/Dockerfile"

# Build the builder image if it doesn't exist or is outdated.
build_image() {
    echo "Building Docker image ${IMAGE_NAME}..." >&2
    docker build \
        --target builder \
        -t "${IMAGE_NAME}" \
        -f "${DOCKERFILE}" \
        "${PROJECT_ROOT}"
}

# Check if image exists.
if ! docker image inspect "${IMAGE_NAME}" >/dev/null 2>&1; then
    build_image
fi

# Run the command in the container.
# - Mount the project directory
# - Use the current user's UID/GID to avoid permission issues
# - Pass through the terminal for interactive commands
if [ $# -eq 0 ]; then
    echo "Usage: $0 <command> [args...]" >&2
    exit 1
fi

INTERACTIVE_FLAGS=""
if [ -t 0 ]; then
    INTERACTIVE_FLAGS="-it"
fi

exec docker run \
    --rm \
    ${INTERACTIVE_FLAGS} \
    -v "${PROJECT_ROOT}:/app" \
    -w /app \
    "${IMAGE_NAME}" \
    "$@"
