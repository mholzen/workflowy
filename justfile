build:
	go build -o workflowy ./cmd/workflowy

test:
	go test ./...

test-integration: build
	bats test/reports.bats

test-integration-api: build
	bats test/api_read.bats

test-integration-write: build
	@echo "WARNING: This will create/modify nodes in your Workflowy account"
	@echo "Requires TEST_PARENT_ID to be set to a sandbox node"
	bats test/api_write.bats

test-integration-all: build
	bats test/*.bats

test-all: test test-integration

# Test release build locally without publishing
release-test:
	goreleaser release --snapshot --clean

# Create and publish release (requires git tag)
release:
	goreleaser release --clean

get item_id:
	go run cmd/workflowy/main.go {{item_id}}

bookmarklet:
	#!/bin/bash
	# Remove comments, minify, and prepend javascript:
	sed 's|//.*||g' bookmarklet/get-projectid.source.js | \
	tr -d '\n' | \
	sed 's/  */ /g' | \
	sed 's/ *{ */{/g' | \
	sed 's/ *} */}/g' | \
	sed 's/ *( */(/g' | \
	sed 's/ *) */)/g' | \
	sed 's/ *; */;/g' | \
	sed 's/ *, */,/g' | \
	sed 's/^ */javascript:/' > bookmarklet/get-projectid.js
	echo "Bookmarklet created in bookmarklet/get-projectid.js"
