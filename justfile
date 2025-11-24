build:
	go build -o workflowy ./cmd/workflowy

test:
	go test ./...

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
