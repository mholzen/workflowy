# Release Process

This document describes how to create a new release of the Workflowy CLI.

## Prerequisites

1. **GitHub Personal Access Token** with `repo` scope
   - Go to https://github.com/settings/tokens
   - Generate a new token (classic)
   - Select `repo` scope (full control of private repositories)
   - Save the token securely

2. **GoReleaser installed**
   ```bash
   brew install goreleaser
   ```

3. **Clean working directory**
   ```bash
   git status  # Should show no uncommitted changes
   ```

## Release Steps

### 1. Update Version References

Update version in `server.json` (MCP registry manifest):
```bash
# Update both "version" and the Docker tag in "identifier"
# Example: for releasing v0.7.0
{
  "version": "0.7.0",
  "packages": [
    {
      "identifier": "ghcr.io/mholzen/workflowy:0.7.0",
      ...
    }
  ]
}
```

Update any other version references if needed:
- `README.md`
- Any other docs mentioning version

### 2. Test Everything

Run the test suite to ensure everything works:
```bash
just test
```

Test a local release build:
```bash
just release-test
```

This creates a snapshot release in `./dist/` without publishing. Verify:
```bash
# Test the built binary
./dist/workflowy_darwin_arm64_v8.0/workflowy version
./dist/workflowy_darwin_arm64_v8.0/workflowy --help

# Check the generated Homebrew formula
cat dist/homebrew/Formula/workflowy.rb
```

### 3. Create and Push Git Tag

Create a semantic version tag:
```bash
# Create annotated tag
git tag -a v0.2.0 -m "Release v0.2.0 - Description of changes"

# Push the tag
git push origin v0.2.0
```

**Note:** The tag must start with `v` (e.g., `v0.2.0`, `v1.0.0`)

### 4. Set GitHub Token

Export your GitHub token as an environment variable:
```bash
export GITHUB_TOKEN=your_github_token_here
```

Or add it to your shell profile (~/.zshrc, ~/.bashrc):
```bash
export GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxx
```

### 5. Run Release

Execute the release:
```bash
just release
```

This will:
1. Run `go mod tidy`
2. Run `go test ./...`
3. Build binaries for all platforms (Linux, macOS, Windows; amd64, arm64)
4. Create archives (tar.gz, zip)
5. Generate checksums
6. Create GitHub release with all artifacts
7. Automatically update the Homebrew tap at `mholzen/homebrew-workflowy`

### 6. Verify Release

1. **Check GitHub Release**
   - Go to https://github.com/mholzen/workflowy/releases
   - Verify the new release appears with all artifacts

2. **Check Homebrew Formula**
   - Go to https://github.com/mholzen/homebrew-workflowy
   - Verify `Formula/workflowy-cli.rb` was updated with new version

3. **Test Installation**
   ```bash
   # Update tap
   brew update

   # Upgrade to new version
   brew upgrade workflowy-cli

   # Verify version
   workflowy version
   ```

### 7. Publish Docker Image (MCP Registry)

Build and push the Docker image to GitHub Container Registry for the MCP registry:

```bash
# Login to ghcr.io (requires PAT with write:packages scope)
echo "$GITHUB_TOKEN" | docker login ghcr.io -u mholzen --password-stdin

# Build the image
docker build -t ghcr.io/mholzen/workflowy:0.7.0 .

# Push to registry
docker push ghcr.io/mholzen/workflowy:0.7.0

# Optionally tag as latest
docker tag ghcr.io/mholzen/workflowy:0.7.0 ghcr.io/mholzen/workflowy:latest
docker push ghcr.io/mholzen/workflowy:latest
```

**Note:** The `GITHUB_TOKEN` needs `write:packages` scope. Create one at https://github.com/settings/tokens if needed.

## What Gets Released

GoReleaser creates binaries for:
- **macOS**: Intel (x86_64), ARM (Apple Silicon)
- **Linux**: Intel (x86_64), ARM (arm64)
- **Windows**: Intel (x86_64), ARM (arm64)

All binaries include version information:
```bash
workflowy version
# Output:
# workflowy version 0.2.0
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
git tag -d v0.2.0

# Delete remote tag (be careful!)
git push origin :refs/tags/v0.2.0

# Create new tag
git tag -a v0.2.0 -m "Release v0.2.0"
git push origin v0.2.0
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
- **MINOR** version (0.2.0): New functionality, backwards compatible
- **PATCH** version (0.1.1): Backwards compatible bug fixes

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
git tag -a v0.2.0 -m "Release v0.2.0"
git push origin v0.2.0

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
