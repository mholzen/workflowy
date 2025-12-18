#!/usr/bin/env bats

# Integration tests for report commands
# These tests can run with --method=backup (no API key required)

load test_helper

setup() {
    # Ensure binary is built
    if [[ ! -x "$WORKFLOWY_BIN" ]]; then
        skip "Binary not found at $WORKFLOWY_BIN - run 'just build' first"
    fi
}

# Count Report Tests

@test "report count outputs markdown format" {
    run run_workflowy report count --method=backup --threshold=50
    [ "$status" -eq 0 ]
    [[ "$output" =~ "# Descendant Count Report" ]]
    [[ "$output" =~ "descendants" ]]
}

@test "report count with threshold filters results" {
    run run_workflowy report count --method=backup --threshold=99
    [ "$status" -eq 0 ]
    # With 99% threshold, should only show root
    [[ "$output" =~ "100.0%" ]]
}

@test "report count json output is valid" {
    require_jq
    run run_workflowy report count --method=backup --threshold=50 --format=json --log=error
    [ "$status" -eq 0 ]
    assert_valid_json "$output"
}

@test "report count json contains expected structure" {
    require_jq
    run run_workflowy report count --method=backup --threshold=50 --format=json --log=error
    [ "$status" -eq 0 ]

    # Should have name and children
    name=$(echo "$output" | jq -r '.name')
    [[ "$name" =~ "Descendant Count Report" ]]

    children_count=$(echo "$output" | jq '.children | length')
    [ "$children_count" -ge 1 ]
}

# Children Report Tests

@test "report children outputs ranked list" {
    run run_workflowy report children --method=backup --top-n=5
    [ "$status" -eq 0 ]
    [[ "$output" =~ "# Top 5 Nodes by Children Count" ]]
    [[ "$output" =~ "1." ]]
    [[ "$output" =~ "children)" ]]
}

@test "report children respects top-n limit" {
    run run_workflowy report children --method=backup --top-n=3
    [ "$status" -eq 0 ]

    # Count the number of list items (lines starting with "- ")
    item_count=$(echo "$output" | grep -c "^- [0-9]")
    [ "$item_count" -eq 3 ]
}

@test "report children json includes full item data" {
    require_jq
    run run_workflowy report children --method=backup --top-n=1 --format=json --log=error
    [ "$status" -eq 0 ]

    # First child should have a child with an ID (the full workflowy item)
    item_id=$(echo "$output" | jq -r '.children[0].children[0].id')
    [[ "$item_id" =~ ^[a-f0-9-]+$ ]]
}

# Created Report Tests

@test "report created outputs oldest nodes" {
    run run_workflowy report created --method=backup --top-n=5
    [ "$status" -eq 0 ]
    [[ "$output" =~ "Oldest Nodes by Creation Date" ]]
}

@test "report created json is valid" {
    require_jq
    run run_workflowy report created --method=backup --top-n=3 --format=json --log=error
    [ "$status" -eq 0 ]
    assert_valid_json "$output"
}

# Modified Report Tests

@test "report modified outputs oldest modified nodes" {
    run run_workflowy report modified --method=backup --top-n=5
    [ "$status" -eq 0 ]
    [[ "$output" =~ "Oldest Nodes by Modification Date" ]]
}

@test "report modified json is valid" {
    require_jq
    run run_workflowy report modified --method=backup --top-n=3 --format=json --log=error
    [ "$status" -eq 0 ]
    assert_valid_json "$output"
}
