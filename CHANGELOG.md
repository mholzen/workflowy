# Changelog

All notable changes to this project will be documented in this file.

## [0.8.1] - Group Transform and MCP Access Methods

### Added
- `transform group` command to group children by created/modified date with Workflowy time tags
  - `--by` flag for group transform: `created.day`, `created.month`, `created.year`, `modified.day`, etc.
  - `--order` flag for group transform: `+created`, `-created`, `+modified`, `-modified`
  - `--regex` flag for split transform: treat separator as a regular expression pattern
  - `--list` flag for split transform: split by markdown list markers (`-`, `*`, `+`, `1.`, `2.`, etc.)

- MCP `--method` flag sets default access method for all operations: `get`, `export`, or `backup`
  - Per-tool `method` parameter on all MCP tools to override the server default
  - MCP `--backup-file` flag to specify custom backup file path

### Changed
- MCP access method is now a hybrid approach: server-wide default with per-tool override
- Interface parity: MCP tools now have the same `method` flexibility as CLI commands

### Documentation
- Updated CLI.md with group transform examples
- Updated MCP.md with access method documentation

## [0.8.0] - Search Grouping and Sorting

### Added
- `--group-by` flag for search: group results by `parent`, `path`, `tree`, `modified.<unit>`, or `created.<unit>`
- `--order-by` flag for search: sort results by `match`, `parent`, `path`, `modified`, or `created` (prefix `+`/`-` for asc/desc)
- `--path-max-length` flag: control max characters per path segment when using `--group-by=path`
- `--include-completed` flag: opt into including completed nodes in search results
- MCP search tool now supports `group_by`, `order_by`, `path_max_length`, and `include_completed` parameters
- Single-dash options can now be combined (e.g., `search -iE` instead of `search -i -E`)

### Changed
- Completed nodes and their descendants are now excluded from search results by default
- API timeout increased from 10s to 30s

### Documentation
- Updated CLI.md and MCP.md with all new search options
- Added blog post for the release

## [0.7.5] - Permission Validation Improvements

### Fixed
- Permission errors now logged at WARN level (previously DEBUG), visible in standard `--log info` output
- Access denied errors for target keys (e.g., "inbox") now say "not within read-root" instead of misleading "node not found"
- Newly created nodes that fail cache validation now return a clear error: "not found in cache; if newly created, retry in ~60s"
- Stale cache fallback log now includes the HTTP response code from the failed API call

### Changed
- Create operations default to `--read-root-id` as parent when `--write-root-id` is not set
- Validation functions resolve target keys to UUIDs before tree lookup
- Validation retries with a forced cache refresh when a node is not found (handles newly created nodes)
- Refactored validation functions to share retry logic via `validateAccessWithRetry`

### Documentation
- Added "Create Default Parent" section to MCP docs explaining parent priority
- Added "Cache and Rate Limiting" section to MCP docs explaining newly created node behavior

## [0.7.4] - Read Restrictions

### Added
- `--read-root-id` global flag to restrict all operations (read and write) to a specific subtree
- MCP `--read-root-id` flag for fully sandboxed AI access
- MCP tool descriptions dynamically show read restrictions when active
- `workflowy_targets` returns `read_root` info when read restrictions are set
- `ValidateReadAccess` and `ValidateAccess` generalized tree validation functions

### Changed
- When `--read-root-id` is set, bare requests (no ID) default to the read-root subtree
- Write operations check both read-root and write-root constraints when both are set

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

