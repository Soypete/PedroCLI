#!/bin/bash
# Test script for llama.cpp with model-specific tool calling

set -e

echo "=== Testing llama.cpp with Qwen 2.5 Coder 32B ==="
echo ""

# Find the model snapshot
SNAPSHOT=$(ls -1 ~/.cache/huggingface/hub/models--bartowski--Qwen2.5-Coder-32B-Instruct-GGUF/snapshots/ 2>/dev/null | head -n 1)

if [ -z "$SNAPSHOT" ]; then
    echo "ERROR: Qwen 2.5 Coder 32B model not found"
    echo "Please download the model first:"
    echo "  huggingface-cli download bartowski/Qwen2.5-Coder-32B-Instruct-GGUF Qwen2.5-Coder-32B-Instruct-Q4_K_M.gguf"
    exit 1
fi

MODEL_PATH="$HOME/.cache/huggingface/hub/models--bartowski--Qwen2.5-Coder-32B-Instruct-GGUF/snapshots/$SNAPSHOT/Qwen2.5-Coder-32B-Instruct-Q4_K_M.gguf"
LLAMA_PATH="/opt/homebrew/bin/llama-cli"

echo "Model: $MODEL_PATH"
echo "llama-cli: $LLAMA_PATH"
echo ""

# Export environment variables for the test
export LLAMA_CPP_PATH="$LLAMA_PATH"
export LLAMA_MODEL_PATH="$MODEL_PATH"

# Run the test
echo "Running tests..."
go test ./pkg/llm -v -run TestLlamaCppToolCalling -timeout 5m

echo ""
echo "=== Test complete ==="
echo ""
echo "Check debug output:"
echo "  cat /tmp/pedrocli-llamacpp-output.txt"
