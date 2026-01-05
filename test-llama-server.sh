#!/bin/bash
# Integration test for llama-server backend

set -e

echo "=== llama-server Integration Test ==="
echo ""

# Configuration
LLAMA_PORT=8082
LLAMA_URL="http://localhost:${LLAMA_PORT}"
MAX_WAIT=30

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Helper functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

cleanup() {
    log_info "Cleaning up..."
    if [ ! -z "$SERVER_PID" ]; then
        log_info "Stopping llama-server (PID: $SERVER_PID)..."
        kill $SERVER_PID 2>/dev/null || true
        wait $SERVER_PID 2>/dev/null || true
    fi
}

# Set trap to cleanup on exit
trap cleanup EXIT

# Check if llama-server is already running
if curl -s "${LLAMA_URL}/health" > /dev/null 2>&1; then
    log_warn "llama-server is already running on port ${LLAMA_PORT}"
    log_warn "Using existing server instance"
    EXISTING_SERVER=true
else
    EXISTING_SERVER=false

    # Start llama-server
    log_info "Starting llama-server..."
    make llama-server > /tmp/llama-server-test.log 2>&1 &
    SERVER_PID=$!
    log_info "llama-server started (PID: $SERVER_PID)"

    # Wait for server to be ready
    log_info "Waiting for llama-server to be ready (max ${MAX_WAIT}s)..."
    for i in $(seq 1 $MAX_WAIT); do
        if curl -s "${LLAMA_URL}/health" > /dev/null 2>&1; then
            log_info "llama-server is ready!"
            break
        fi
        if [ $i -eq $MAX_WAIT ]; then
            log_error "Timeout waiting for llama-server"
            log_error "Server log:"
            cat /tmp/llama-server-test.log
            exit 1
        fi
        sleep 1
    done
fi

echo ""
log_info "Running tests..."
echo ""

# Test 1: Health check
log_info "Test 1: Health check"
if curl -s "${LLAMA_URL}/health" | jq -e '.status == "ok"' > /dev/null 2>&1; then
    log_info "✅ Health check passed"
else
    log_error "❌ Health check failed"
    exit 1
fi

# Test 2: Basic inference
log_info "Test 2: Basic inference"
RESPONSE=$(curl -s "${LLAMA_URL}/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -d '{
        "model": "default",
        "messages": [
            {"role": "system", "content": "You are a helpful assistant."},
            {"role": "user", "content": "Say hello in exactly 2 words."}
        ],
        "temperature": 0.1,
        "max_tokens": 10
    }')

if echo "$RESPONSE" | jq -e '.choices[0].message.content' > /dev/null 2>&1; then
    CONTENT=$(echo "$RESPONSE" | jq -r '.choices[0].message.content')
    log_info "✅ Basic inference passed"
    log_info "   Response: $CONTENT"
else
    log_error "❌ Basic inference failed"
    echo "$RESPONSE" | jq .
    exit 1
fi

# Test 3: Tool calling (if tools are supported)
log_info "Test 3: Tool calling"
TOOL_RESPONSE=$(curl -s "${LLAMA_URL}/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -d '{
        "model": "default",
        "messages": [
            {"role": "system", "content": "You are a helpful assistant with access to tools."},
            {"role": "user", "content": "Search for information about Go programming"}
        ],
        "temperature": 0.1,
        "max_tokens": 100,
        "tools": [
            {
                "type": "function",
                "function": {
                    "name": "search",
                    "description": "Search for information",
                    "parameters": {
                        "type": "object",
                        "properties": {
                            "query": {
                                "type": "string",
                                "description": "The search query"
                            }
                        },
                        "required": ["query"]
                    }
                }
            }
        ]
    }')

if echo "$TOOL_RESPONSE" | jq -e '.choices[0].message' > /dev/null 2>&1; then
    # Check if tool calls are present
    TOOL_CALLS=$(echo "$TOOL_RESPONSE" | jq '.choices[0].message.tool_calls // []')
    if [ "$TOOL_CALLS" != "[]" ]; then
        log_info "✅ Tool calling passed"
        log_info "   Tool calls: $(echo $TOOL_CALLS | jq -c .)"
    else
        log_warn "⚠️  Tool calling response received, but no tool calls (model may need different prompting)"
        log_info "   Response: $(echo "$TOOL_RESPONSE" | jq -c '.choices[0].message.content')"
    fi
else
    log_error "❌ Tool calling failed"
    echo "$TOOL_RESPONSE" | jq .
    exit 1
fi

# Test 4: Context window
log_info "Test 4: Context window check"
if curl -s "${LLAMA_URL}/v1/models" | jq -e '.data[0]' > /dev/null 2>&1; then
    MODEL_INFO=$(curl -s "${LLAMA_URL}/v1/models" | jq -r '.data[0]')
    log_info "✅ Models endpoint accessible"
    log_info "   Model info: $(echo $MODEL_INFO | jq -c .)"
else
    log_warn "⚠️  Models endpoint not available (may be expected)"
fi

echo ""
log_info "=== All tests completed successfully! ==="
echo ""

# Summary
echo "Summary:"
echo "  ✅ Health check"
echo "  ✅ Basic inference"
if [ "$TOOL_CALLS" != "[]" ]; then
    echo "  ✅ Tool calling"
else
    echo "  ⚠️  Tool calling (partial)"
fi
echo ""

if [ "$EXISTING_SERVER" = false ]; then
    log_info "Server log saved to: /tmp/llama-server-test.log"
fi
