# Changelog

All notable changes to this project will be documented in this file.

## [0.7.3] - Transform Improvements

### Added
- Debug logging for shell transforms (`-x`): logs input/output (truncated to 40 chars) at debug level
- `--all` flag for transform (equivalent to `--depth=-1`)

### Changed
- Transform command now uses same fetch logic as `get` command (supports `get`, `export`, `backup` methods)
- Transform default depth changed from unlimited to 2 (consistent with `get` command)
- Write-root validation only performed when `--write-root-id` flag is actually set

### Documentation
- Added missing MCP tools to README: `workflowy_move`, `workflowy_transform`, `workflowy_id`, `workflowy_report_mirrors`
- Added Claude Code setup command: `claude mcp add --transport=stdio workflowy -- workflowy mcp --expose=all`
- Added Transform Content section to README with examples
- Updated feature list with sandboxed AI access and content transformation

## [0.7.2] - Mirror Report

### Added
- `report mirrors` command to rank nodes by mirror count (most mirrored first)
- MCP tool `workflowy_report_mirrors` for AI assistant access to mirror data
- Mirror report shows original node with parent context and all mirror locations

### Notes
- Mirror data is only available in backup files (`--method=backup` required)

## [0.7.1] - Write Restrictions

### Added
- `--write-root-id` global flag to restrict write operations to a specific subtree
- MCP `--write-root-id` flag for sandboxed AI access
- MCP tool descriptions dynamically show write restrictions when active
- `workflowy_targets` returns `write_root` info when restrictions are set

### Changed
- Create command defaults to write-root-id as parent when no parent specified and restrictions are active

## [0.7.0] - Transform, Move and Id commands

### Added
- `move` command to relocate nodes to a new parent with position control
- `transform` command with built-in transforms: lowercase, uppercase, capitalize, title, trim, no-punctuation, no-whitespace
- `split` transform to split node content by separator into child nodes
- `--as-child` flag for transform to insert results as children instead of replacing
- `id` command to resolve short IDs and target keys to full UUIDs
- MCP tools: `workflowy_transform`, `workflowy_move`, `workflowy_id`

### Changed
- Consistent terminology across CLI and MCP: unified `id` parameter naming and `node` references
- Split transform requires explicit `split` name with default separator `,`
- Minimal Dockerfile for stdio interface (MCP registry compatible)

## [0.6.0] - Short IDs and configuration improvements

### Added
- Short ID support: use last 12 characters of a node ID instead of full UUID
- Workflowy internal link support: paste URLs from "Copy Internal Link" directly as node IDs
- `WORKFLOWY_API_KEY` environment variable support with precedence: flag > env var > default file
- Tilde (`~`) expansion for `--api-key-file` and `--log-file` paths

### Fixed
- Node IDs are now sanitized to strip non-hexadecimal characters

## [0.5.1] - Fix bug with MCP commands that return more than one result

### Fixed
- For MCP commands that have many results, return objects instead of arrays
- Use Hooks for logging instead of custom handler wrapper


## [0.5.0] - Add MCP server capability

- Add support for an MCP server, with the ability to expose some or all existing
  commands, and the ability to send logs to a file for troubleshooting.


## [0.4.1] - Strip HTML tags when printing reports

### Added
- strip HTML tags in Markdown report outputs, so that they paste properly (use --preserve-tags to not strip)


## [0.4.0] - Add search and replace, delete, targets commands

### Added
- Targets command
- Search and replace command
- Delete command
- Integration tests
- Inform user of where to get an API key if missing

### Fixed
- Errors now return non-zero exit code
- Reports upload now correctly creates links
- Create command properly receives the location from the API key
- Error reporting no longer looks like a log message (removed unnecessary timestamp)

### Changed
- Unified client creation code to single function
- Unified error and log messaging for consistency

