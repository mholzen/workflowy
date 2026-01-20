# Release Process

This document describes how to create a new release of the Workflowy CLI.

## Prerequisites

1. **GitHub Tokens**
   - `GITHUB_TOKEN` with `repo` scope for goreleaser
   - `GITHUB_CONTAINER_REGISTRY_TOKEN` with `write:packages` scope for Docker
   - Go to https://github.com/settings/tokens to create tokens

2. **Tools installed**
   ```bash
   brew install goreleaser
   brew install mcp-publisher  # For MCP registry
   ```

3. **Clean working directory**
   ```bash
   git status  # Should show no uncommitted changes
   ```

## Quick Release

```bash
# 1. Update CHANGELOG.md with new version (e.g., 0.7.1)

# 2. Test locally
just test
just release-test

# 3. Full release (binaries + Homebrew + Scoop)
just release 0.7.1

# 4. Docker + MCP registry
just docker-release 0.7.1
```

## What Each Command Does

### `just release VERSION`

1. **verify-version** - Checks VERSION matches CHANGELOG.md
2. **update-server-json** - Updates version in server.json
3. **create-tag** - Creates annotated git tag with changelog description
4. **push-tag** - Pushes tag to origin
5. **goreleaser release** - Builds and publishes:
   - Binaries for all platforms (Linux, macOS, Windows; amd64, arm64)
   - GitHub release with archives and checksums
   - Homebrew formula to `mholzen/homebrew-workflowy`
   - Scoop manifest to `mholzen/scoop-workflowy`

### `just docker-release VERSION`

1. **docker-login** - Logs in to ghcr.io
2. **docker-build** - Builds and pushes multi-platform image (amd64, arm64)
3. **mcp-publisher publish** - Updates MCP registry

## Verify Release

1. **GitHub Release**: https://github.com/mholzen/workflowy/releases
2. **Homebrew**: https://github.com/mholzen/homebrew-workflowy
3. **Scoop**: https://github.com/mholzen/scoop-workflowy
4. **Docker**: https://ghcr.io/mholzen/workflowy

```bash
# Test Homebrew
brew update && brew upgrade workflowy-cli && workflowy version

# Test Scoop (Windows)
scoop update workflowy && workflowy version

# Test Docker
docker run --rm ghcr.io/mholzen/workflowy:latest version
```

## What Gets Released

GoReleaser creates binaries for:
- **macOS**: Intel (x86_64), ARM (Apple Silicon)
- **Linux**: Intel (x86_64), ARM (arm64)
- **Windows**: Intel (x86_64), ARM (arm64)

All binaries include version information:
```bash
workflowy version
# Output:
# workflowy version ${VERSION}
# commit: abc123...
# built: 2025-11-23T12:34:56Z
```

## Troubleshooting

### Error: "git is currently in a dirty state"
```bash
# Commit or stash your changes
git status
git add .
git commit -m "Prepare for release"
```

### Error: "GITHUB_TOKEN not set"
```bash
export GITHUB_TOKEN=your_token
```

### Error: "tag already exists"
```bash
# Delete local tag
git tag -d "v${VERSION}"

# Delete remote tag (be careful!)
git push origin ":refs/tags/v${VERSION}"

# Create new tag
git tag -a "v${VERSION}" -m "Release v${VERSION}"
git push origin "v${VERSION}"
```

### Release Failed Midway
If the release fails, you may need to:
1. Delete the incomplete GitHub release manually
2. Delete the tag if needed
3. Fix the issue
4. Try again

## Version Numbering

We follow [Semantic Versioning](https://semver.org/):
- **MAJOR** version (1.0.0): Incompatible API changes
- **MINOR** version (0.x.0): New functionality, backwards compatible
- **PATCH** version (0.x.1): Backwards compatible bug fixes

## Changelog

GoReleaser automatically generates a changelog from git commit messages. For best results:
- Use clear, descriptive commit messages
- Prefix commits with type: `feat:`, `fix:`, `docs:`, `chore:`, etc.
- Commits prefixed with `docs:`, `test:`, `chore:`, `ci:` are excluded from changelog

## Future: Automated Releases

To automate the release process using GitHub Actions:

### Setup

1. **Create GitHub Actions Workflow**

Create `.github/workflows/release.yml`:

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: stable

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

2. **Simplified Release Process**

With GitHub Actions, the release process becomes:

```bash
# 1. Test locally
just test
just release-test

# 2. Create and push tag
git tag -a "v${VERSION}" -m "Release v${VERSION}"
git push origin "v${VERSION}"

# That's it! GitHub Actions automatically:
# - Runs tests
# - Builds all binaries
# - Creates GitHub release
# - Updates Homebrew formula
```

### Benefits of Automation

- ✅ No need to set GITHUB_TOKEN locally
- ✅ No need to run `just release` manually
- ✅ Consistent build environment (Ubuntu)
- ✅ Build logs available in GitHub Actions UI
- ✅ Can't accidentally release from dirty working directory

### Enabling Automation

When ready to enable automated releases:

1. Create the `.github/workflows/release.yml` file above
2. Commit and push to main
3. Next time you push a tag, release happens automatically

The `GITHUB_TOKEN` is automatically provided by GitHub Actions - no need to manage tokens manually.
