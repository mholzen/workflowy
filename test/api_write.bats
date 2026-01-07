#!/usr/bin/env bats

# Integration tests for write API operations
# These tests require a valid API key AND a TEST_PARENT_ID
# WARNING: These tests create/modify real nodes in your Workflowy account

load test_helper

setup_file() {
    export WORKFLOWY_BIN="${WORKFLOWY_BIN:-./workflowy}"
    export API_KEY_FILE="${API_KEY_FILE:-$HOME/.workflowy/api.key}"
    export TEST_PARENT_ID="${TEST_PARENT_ID:-}"

    if [[ ! -x "$WORKFLOWY_BIN" ]]; then
        return
    fi
    if [[ ! -f "$API_KEY_FILE" ]]; then
        return
    fi
    if [[ -z "$TEST_PARENT_ID" ]]; then
        return
    fi

    # Use BATS_FILE_TMPDIR which persists across tests in the same file
    export SEARCH_TEST_NODES_FILE="$BATS_FILE_TMPDIR/search_test_nodes"

    local output node_id

    output=$("$WORKFLOWY_BIN" create --parent-id="$TEST_PARENT_ID" --name="batsearch_static test node")
    node_id=$(echo "$output" | awk '{print $1}')
    echo "SEARCH_BASIC=$node_id" >> "$SEARCH_TEST_NODES_FILE"

    output=$("$WORKFLOWY_BIN" create --parent-id="$TEST_PARENT_ID" --name="BATSEARCHUPPER_STATIC test node")
    node_id=$(echo "$output" | awk '{print $1}')
    echo "SEARCH_CASE=$node_id" >> "$SEARCH_TEST_NODES_FILE"

    output=$("$WORKFLOWY_BIN" create --parent-id="$TEST_PARENT_ID" --name="batregex_static-123 test")
    node_id=$(echo "$output" | awk '{print $1}')
    echo "SEARCH_REGEX=$node_id" >> "$SEARCH_TEST_NODES_FILE"

    output=$("$WORKFLOWY_BIN" create --parent-id="$TEST_PARENT_ID" --name="batreplace_dryrun DRYRUN_OLD value")
    node_id=$(echo "$output" | awk '{print $1}')
    echo "REPLACE_DRYRUN=$node_id" >> "$SEARCH_TEST_NODES_FILE"

    output=$("$WORKFLOWY_BIN" create --parent-id="$TEST_PARENT_ID" --name="batreplace_apply APPLY_OLD here")
    node_id=$(echo "$output" | awk '{print $1}')
    echo "REPLACE_APPLY=$node_id" >> "$SEARCH_TEST_NODES_FILE"

    output=$("$WORKFLOWY_BIN" create --parent-id="$TEST_PARENT_ID" --name="batcapture_static task-888")
    node_id=$(echo "$output" | awk '{print $1}')
    echo "REPLACE_CAPTURE=$node_id" >> "$SEARCH_TEST_NODES_FILE"

    output=$("$WORKFLOWY_BIN" create --parent-id="$TEST_PARENT_ID" --name="batcasei_static TODO_CASE item")
    node_id=$(echo "$output" | awk '{print $1}')
    echo "REPLACE_CASEI=$node_id" >> "$SEARCH_TEST_NODES_FILE"

    # Delete the export cache to avoid rate limiting issues from previous test runs
    rm -f "$HOME/.workflowy/export-cache.json"

    # Force refresh to get the newly created nodes in the cache
    "$WORKFLOWY_BIN" search --force-refresh "batsearch_static" > /dev/null 2>&1 || true
}

teardown_file() {
    # BATS_FILE_TMPDIR is automatically cleaned up by bats
    if [[ -f "$BATS_FILE_TMPDIR/search_test_nodes" ]]; then
        while IFS='=' read -r key node_id; do
            "$WORKFLOWY_BIN" delete "$node_id" 2>/dev/null || true
        done < "$BATS_FILE_TMPDIR/search_test_nodes"
    fi
}

get_test_node_id() {
    local key="$1"
    local nodes_file="$BATS_FILE_TMPDIR/search_test_nodes"
    if [[ -f "$nodes_file" ]]; then
        grep "^${key}=" "$nodes_file" | cut -d'=' -f2
    fi
}

setup() {
    if [[ ! -x "$WORKFLOWY_BIN" ]]; then
        skip "Binary not found at $WORKFLOWY_BIN - run 'just build' first"
    fi
    skip_if_no_api_key
    skip_if_no_test_parent
}

teardown() {
    cleanup_created_nodes
}

# Create Command Tests

@test "create node with name" {
    run run_workflowy create --parent-id="$TEST_PARENT_ID" --name="bats test node $(date +%s)"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "created" ]]

    node_id=$(echo "$output" | awk '{print $1}')
    [[ "$node_id" =~ ^[a-f0-9-]+$ ]]
    track_node_for_cleanup "$node_id"
}

@test "create node returns valid id" {
    run run_workflowy create --parent-id="$TEST_PARENT_ID" --name="bats id test $(date +%s)"
    [ "$status" -eq 0 ]

    node_id=$(echo "$output" | awk '{print $1}')
    [[ "$node_id" =~ ^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$ ]]
    track_node_for_cleanup "$node_id"
}

@test "create and retrieve node" {
    local test_name="bats retrieve test $(date +%s)"
    run run_workflowy create --parent-id="$TEST_PARENT_ID" --name="$test_name"
    [ "$status" -eq 0 ]

    node_id=$(echo "$output" | awk '{print $1}')
    track_node_for_cleanup "$node_id"

    run run_workflowy get "$node_id"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "$test_name" ]]
}

# Complete/Uncomplete Command Tests

@test "complete node" {
    run run_workflowy create --parent-id="$TEST_PARENT_ID" --name="bats complete test $(date +%s)"
    [ "$status" -eq 0 ]
    node_id=$(echo "$output" | awk '{print $1}')
    track_node_for_cleanup "$node_id"

    run run_workflowy complete "$node_id"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "completed" ]]
}

@test "uncomplete node" {
    run run_workflowy create --parent-id="$TEST_PARENT_ID" --name="bats uncomplete test $(date +%s)"
    [ "$status" -eq 0 ]
    node_id=$(echo "$output" | awk '{print $1}')
    track_node_for_cleanup "$node_id"

    run run_workflowy complete "$node_id"
    [ "$status" -eq 0 ]

    run run_workflowy uncomplete "$node_id"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "uncompleted" ]]
}

@test "complete and uncomplete roundtrip" {
    run run_workflowy create --parent-id="$TEST_PARENT_ID" --name="bats roundtrip test $(date +%s)"
    [ "$status" -eq 0 ]
    node_id=$(echo "$output" | awk '{print $1}')
    track_node_for_cleanup "$node_id"

    run run_workflowy complete "$node_id"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "$node_id" ]]
    [[ "$output" =~ "completed" ]]

    run run_workflowy uncomplete "$node_id"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "$node_id" ]]
    [[ "$output" =~ "uncompleted" ]]
}

# Update Command Tests

@test "update node name" {
    run run_workflowy create --parent-id="$TEST_PARENT_ID" --name="bats update test $(date +%s)"
    [ "$status" -eq 0 ]
    node_id=$(echo "$output" | awk '{print $1}')
    track_node_for_cleanup "$node_id"

    local new_name="bats updated name $(date +%s)"
    run run_workflowy update "$node_id" --name="$new_name"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "updated" ]]

    run run_workflowy get "$node_id"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "$new_name" ]]
}

# Delete Command Tests

@test "delete node" {
    run run_workflowy create --parent-id="$TEST_PARENT_ID" --name="bats delete test $(date +%s)"
    [ "$status" -eq 0 ]
    node_id=$(echo "$output" | awk '{print $1}')

    run run_workflowy delete "$node_id"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "deleted" ]]
}

@test "create and delete node" {
    local test_name="bats delete verify test $(date +%s)"
    run run_workflowy create --parent-id="$TEST_PARENT_ID" --name="$test_name"
    [ "$status" -eq 0 ]
    node_id=$(echo "$output" | awk '{print $1}')

    run run_workflowy delete "$node_id"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "$node_id" ]]
    [[ "$output" =~ "deleted" ]]

    run run_workflowy get "$node_id"
    [ "$status" -ne 0 ]
}

# Error Handling Tests

@test "delete invalid node id fails gracefully" {
    run run_workflowy delete "invalid-node-id"
    [ "$status" -ne 0 ]
}

@test "complete invalid node id fails gracefully" {
    run run_workflowy complete "invalid-node-id"
    [ "$status" -ne 0 ]
}

# Search Command Tests (use pre-created nodes from setup_file)

@test "search finds created node" {
    run run_workflowy search "batsearch_static" --item-id="$TEST_PARENT_ID" --force-refresh
    [ "$status" -eq 0 ]
    [[ "$output" =~ "batsearch_static" ]]
}

@test "search case insensitive" {
    run run_workflowy search -i "batsearchupper_static" --item-id="$TEST_PARENT_ID"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "BATSEARCHUPPER_STATIC" ]]
}

@test "search with regex" {
    run run_workflowy search -E "batregex_static-[0-9]+" --item-id="$TEST_PARENT_ID"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "batregex_static" ]]
}

# Replace Command Tests (use pre-created nodes from setup_file)

@test "replace dry-run shows changes without applying" {
    local node_id
    node_id=$(get_test_node_id "REPLACE_DRYRUN")

    run run_workflowy replace --dry-run --parent-id="$TEST_PARENT_ID" "DRYRUN_OLD" "DRYRUN_NEW"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "dry-run" ]]
    [[ "$output" =~ "DRYRUN_OLD" ]]
    [[ "$output" =~ "DRYRUN_NEW" ]]

    run run_workflowy get "$node_id"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "DRYRUN_OLD" ]]
}

@test "replace applies changes" {
    local node_id
    node_id=$(get_test_node_id "REPLACE_APPLY")

    run run_workflowy replace --parent-id="$TEST_PARENT_ID" "APPLY_OLD" "APPLY_NEW"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "Updated 1 node" ]]

    run run_workflowy get "$node_id"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "APPLY_NEW" ]]
}

@test "replace with capture groups" {
    local node_id
    node_id=$(get_test_node_id "REPLACE_CAPTURE")

    run run_workflowy replace --parent-id="$TEST_PARENT_ID" "task-([0-9]+)" 'item_$1'
    [ "$status" -eq 0 ]
    [[ "$output" =~ "Updated 1 node" ]]

    run run_workflowy get "$node_id"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "item_888" ]]
}

@test "replace case insensitive" {
    local node_id
    node_id=$(get_test_node_id "REPLACE_CASEI")

    run run_workflowy replace -i --parent-id="$TEST_PARENT_ID" "todo_case" "DONE_CASE"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "Updated 1 node" ]]

    run run_workflowy get "$node_id"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "DONE_CASE" ]]
}

@test "replace no matches" {
    run run_workflowy replace --parent-id="$TEST_PARENT_ID" "NONEXISTENT_PATTERN_xyz123" "replacement"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "No matches found" ]]
}
