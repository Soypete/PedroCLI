#!/usr/bin/env bash
# Build and push PedroCLI images to local ZOT registry
# Usage: ./ops/scripts/build-push.sh [pedrocli|whisper|all]
# Default: build and push both images

set -euo pipefail

ZOT_REGISTRY="${ZOT_REGISTRY:-100.81.89.62:5000}"
PEDROCLI_IMAGE="${ZOT_REGISTRY}/pedrocli-http:latest"
WHISPER_IMAGE="${ZOT_REGISTRY}/whisper:latest"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

cd "${REPO_ROOT}"

build_pedrocli() {
    echo "==> Building pedrocli-http image..."
    docker build \
        --file Dockerfile \
        --target builder \
        --tag "${PEDROCLI_IMAGE}" \
        --build-arg BINARY=http-server \
        . || \
    docker build \
        --file Dockerfile \
        --tag "${PEDROCLI_IMAGE}" \
        .
    echo "==> Pushing ${PEDROCLI_IMAGE}..."
    docker push "${PEDROCLI_IMAGE}"
    echo "✓ pedrocli-http pushed to ${PEDROCLI_IMAGE}"
}

build_whisper() {
    echo "==> Building whisper image (this takes a few minutes — compiling C++ and downloading model)..."
    docker build \
        --file Dockerfile.whisper \
        --tag "${WHISPER_IMAGE}" \
        .
    echo "==> Pushing ${WHISPER_IMAGE}..."
    docker push "${WHISPER_IMAGE}"
    echo "✓ whisper pushed to ${WHISPER_IMAGE}"
}

TARGET="${1:-all}"

case "${TARGET}" in
    pedrocli)
        build_pedrocli
        ;;
    whisper)
        build_whisper
        ;;
    all)
        build_pedrocli
        build_whisper
        ;;
    *)
        echo "Usage: $0 [pedrocli|whisper|all]"
        exit 1
        ;;
esac

echo ""
echo "Done. Images available at ${ZOT_REGISTRY}"
