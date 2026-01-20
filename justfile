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

# Create and publish release
release VERSION:
	just release-prep {{VERSION}}
	just push-tag {{VERSION}}
	goreleaser release --clean

# Extract version from CHANGELOG.md (topmost ## [x.x.x] entry)
changelog-version:
	@grep -m1 '^## \[' CHANGELOG.md | sed 's/## \[\([^]]*\)\].*/\1/'

# Extract description from CHANGELOG.md (text after version)
changelog-description:
	@grep -m1 '^## \[' CHANGELOG.md | sed 's/## \[[^]]*\] - //'

# Verify VERSION matches CHANGELOG.md
verify-version VERSION:
	#!/bin/bash
	changelog_version=$(just changelog-version)
	if [ "{{VERSION}}" != "$changelog_version" ]; then
		echo "ERROR: VERSION {{VERSION}} does not match CHANGELOG.md version $changelog_version"
		exit 1
	fi
	echo "Version {{VERSION}} matches CHANGELOG.md"

# Update server.json with new version
update-server-json VERSION:
	#!/bin/bash
	just verify-version {{VERSION}}
	sed -i '' 's/"version": "[^"]*"/"version": "{{VERSION}}"/' server.json
	sed -i '' 's|ghcr.io/mholzen/workflowy:[^"]*|ghcr.io/mholzen/workflowy:{{VERSION}}|' server.json
	echo "Updated server.json to version {{VERSION}}"

# Create git tag with description from CHANGELOG.md
create-tag VERSION:
	#!/bin/bash
	just verify-version {{VERSION}}
	description=$(just changelog-description)
	git tag -a "v{{VERSION}}" -m "Release v{{VERSION}} - $description"
	echo "Created tag v{{VERSION}} with message: Release v{{VERSION}} - $description"

# Push tag to remote
push-tag VERSION:
	git push origin "v{{VERSION}}"

# Full release prep: verify, update server.json, create and push tag
release-prep VERSION:
	just verify-version {{VERSION}}
	just update-server-json {{VERSION}}
	just create-tag {{VERSION}}
	@echo ""
	@echo "Ready to push. Run: just push-tag {{VERSION}}"

# Login to GitHub Container Registry
docker-login:
	@echo "Logging in to ghcr.io..."
	@echo "$$GITHUB_CONTAINER_REGISTRY_TOKEN" | docker login ghcr.io -u mholzen --password-stdin

# Build and push multi-platform Docker image
docker-build VERSION:
	docker buildx build --platform linux/amd64,linux/arm64 \
		-t "ghcr.io/mholzen/workflowy:{{VERSION}}" \
		-t "ghcr.io/mholzen/workflowy:latest" \
		--push .

# Full Docker release: login, build, push, and publish to MCP registry
docker-release VERSION:
	just docker-login
	just docker-build {{VERSION}}
	mcp-publisher login github
	mcp-publisher publish
	@echo "Docker image pushed and MCP registry updated"

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
