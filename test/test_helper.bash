#!/usr/bin/env bash

# Common test helper functions for bats integration tests

# Path to the workflowy binary (built by just build)
export WORKFLOWY_BIN="${WORKFLOWY_BIN:-./workflowy}"

# Test timeout for commands (seconds)
export TEST_TIMEOUT="${TEST_TIMEOUT:-10}"

# API key file location
export API_KEY_FILE="${API_KEY_FILE:-$HOME/.workflowy/api.key}"

# Parent node ID for integration tests that create nodes
# Set this to a dedicated test sandbox node in your Workflowy account
export TEST_PARENT_ID="${TEST_PARENT_ID:-}"

# Track created nodes for cleanup
CREATED_NODES=()

run_workflowy() {
    timeout "$TEST_TIMEOUT" "$WORKFLOWY_BIN" "$@"
}

skip_if_no_api_key() {
    if [[ ! -f "$API_KEY_FILE" ]]; then
        skip "API key file not found at $API_KEY_FILE"
    fi
}

skip_if_no_test_parent() {
    if [[ -z "$TEST_PARENT_ID" ]]; then
        skip "TEST_PARENT_ID not set - required for write operations"
    fi
}

require_jq() {
    if ! command -v jq &> /dev/null; then
        skip "jq is required for this test"
    fi
}

assert_valid_json() {
    local json="$1"
    if ! echo "$json" | jq . > /dev/null 2>&1; then
        echo "Invalid JSON: $json" >&2
        return 1
    fi
}

extract_node_id() {
    local output="$1"
    echo "$output" | grep -oE '[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}' | head -1
}

track_node_for_cleanup() {
    local node_id="$1"
    CREATED_NODES+=("$node_id")
}

cleanup_created_nodes() {
    for node_id in "${CREATED_NODES[@]}"; do
        "$WORKFLOWY_BIN" delete "$node_id" 2>/dev/null || true
    done
    CREATED_NODES=()
}
