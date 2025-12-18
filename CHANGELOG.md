# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

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

