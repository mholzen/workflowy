#!/usr/bin/env bats

# Integration tests for write API operations
# These tests require a valid API key AND a TEST_PARENT_ID
# WARNING: These tests create/modify real nodes in your Workflowy account

load test_helper

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

    # Output format is: <node-id> created
    node_id=$(echo "$output" | awk '{print $1}')
    [[ "$node_id" =~ ^[a-f0-9-]+$ ]]
    track_node_for_cleanup "$node_id"
}

@test "create node returns valid id" {
    run run_workflowy create --parent-id="$TEST_PARENT_ID" --name="bats id test $(date +%s)"
    [ "$status" -eq 0 ]

    # Output format is: <node-id> created
    node_id=$(echo "$output" | awk '{print $1}')
    [[ "$node_id" =~ ^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$ ]]
    track_node_for_cleanup "$node_id"
}

@test "create and retrieve node" {
    # Create
    local test_name="bats retrieve test $(date +%s)"
    run run_workflowy create --parent-id="$TEST_PARENT_ID" --name="$test_name"
    [ "$status" -eq 0 ]

    # Output format is: <node-id> created
    node_id=$(echo "$output" | awk '{print $1}')
    track_node_for_cleanup "$node_id"

    # Retrieve and verify
    run run_workflowy get "$node_id"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "$test_name" ]]
}

# Complete/Uncomplete Command Tests

@test "complete node" {
    # Create a test node first
    run run_workflowy create --parent-id="$TEST_PARENT_ID" --name="bats complete test $(date +%s)"
    [ "$status" -eq 0 ]
    node_id=$(echo "$output" | awk '{print $1}')
    track_node_for_cleanup "$node_id"

    # Complete it
    run run_workflowy complete "$node_id"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "completed" ]]
}

@test "uncomplete node" {
    # Create a test node first
    run run_workflowy create --parent-id="$TEST_PARENT_ID" --name="bats uncomplete test $(date +%s)"
    [ "$status" -eq 0 ]
    node_id=$(echo "$output" | awk '{print $1}')
    track_node_for_cleanup "$node_id"

    # Complete it
    run run_workflowy complete "$node_id"
    [ "$status" -eq 0 ]

    # Uncomplete it
    run run_workflowy uncomplete "$node_id"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "uncompleted" ]]
}

@test "complete and uncomplete roundtrip" {
    # Create
    run run_workflowy create --parent-id="$TEST_PARENT_ID" --name="bats roundtrip test $(date +%s)"
    [ "$status" -eq 0 ]
    node_id=$(echo "$output" | awk '{print $1}')
    track_node_for_cleanup "$node_id"

    # Complete
    run run_workflowy complete "$node_id"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "$node_id" ]]
    [[ "$output" =~ "completed" ]]

    # Uncomplete
    run run_workflowy uncomplete "$node_id"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "$node_id" ]]
    [[ "$output" =~ "uncompleted" ]]
}

# Update Command Tests

@test "update node name" {
    # Create a test node first
    run run_workflowy create --parent-id="$TEST_PARENT_ID" --name="bats update test $(date +%s)"
    [ "$status" -eq 0 ]
    node_id=$(echo "$output" | awk '{print $1}')
    track_node_for_cleanup "$node_id"

    # Update the name
    local new_name="bats updated name $(date +%s)"
    run run_workflowy update "$node_id" --name="$new_name"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "updated" ]]

    # Verify the update
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
