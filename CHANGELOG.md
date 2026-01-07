# Changelog

All notable changes to this project will be documented in this file.

## [0.5.2] - Short IDs and configuration improvements

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

