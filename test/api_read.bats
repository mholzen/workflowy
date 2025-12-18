#!/usr/bin/env bats

# Integration tests for read-only API operations
# These tests require a valid API key

load test_helper

setup() {
    if [[ ! -x "$WORKFLOWY_BIN" ]]; then
        skip "Binary not found at $WORKFLOWY_BIN - run 'just build' first"
    fi
    skip_if_no_api_key
}

# Get Command Tests

@test "get root returns items" {
    run run_workflowy get
    [ "$status" -eq 0 ]
    # Should have some output (list of root items)
    [ -n "$output" ]
}

@test "get with json format is valid" {
    require_jq
    run run_workflowy get --format=json --log=error
    [ "$status" -eq 0 ]
    assert_valid_json "$output"
}

@test "get specific item by id" {
    skip_if_no_test_parent
    run run_workflowy get "$TEST_PARENT_ID"
    [ "$status" -eq 0 ]
    [ -n "$output" ]
}

@test "get with depth limits children" {
    run run_workflowy get --depth=1
    [ "$status" -eq 0 ]
}

# List Command Tests

@test "list returns flat list of items" {
    run run_workflowy list --depth=2
    [ "$status" -eq 0 ]
    [ -n "$output" ]
}

@test "list with json format is valid" {
    require_jq
    run run_workflowy list --depth=2 --format=json --log=error
    [ "$status" -eq 0 ]
    assert_valid_json "$output"
}

# Search Command Tests

@test "search finds matching items" {
    run run_workflowy search "test"
    # May or may not find results, but should not error
    [ "$status" -eq 0 ]
}

@test "search with json format is valid" {
    require_jq
    run run_workflowy search "the" --format=json --log=error
    [ "$status" -eq 0 ]
    # Output might be empty array or results
    assert_valid_json "$output"
}

@test "search case insensitive by default" {
    run run_workflowy search "THE"
    [ "$status" -eq 0 ]
}

# Version Command Tests

@test "version command outputs version info" {
    run run_workflowy version
    [ "$status" -eq 0 ]
    [[ "$output" =~ "version" ]] || [[ "$output" =~ "Version" ]] || [[ "$output" =~ [0-9]+\.[0-9]+ ]]
}

# Help Command Tests

@test "help command shows usage" {
    run run_workflowy --help
    [ "$status" -eq 0 ]
    [[ "$output" =~ "USAGE" ]] || [[ "$output" =~ "Usage" ]]
    [[ "$output" =~ "COMMANDS" ]] || [[ "$output" =~ "Commands" ]]
}

@test "report help shows subcommands" {
    run run_workflowy report --help
    [ "$status" -eq 0 ]
    [[ "$output" =~ "count" ]]
    [[ "$output" =~ "children" ]]
    [[ "$output" =~ "created" ]]
    [[ "$output" =~ "modified" ]]
}
