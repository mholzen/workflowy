# Changelog

All notable changes to this project will be documented in this file.

## [0.5.0] - Add MCP server capability

- Add support for an MCP server, with the ability to expose some or all existing
  commands, and the the ability to send logs to a file for troubleshooting.


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

