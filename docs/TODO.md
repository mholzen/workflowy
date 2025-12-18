# TODO

## SHOULD DO

- [ ] BUG: strip html tags when creating links in reports, as they are not recognized when pasting to WF
- [ ] BUG: rename 'markdown' format to 'document' to avoid confusion with 'list'
- [ ] FEATURE: add last modified to count report
- [ ] FEATURE: search and replace
- [ ] FEATURE: build an MCP server
  - [ ] CODE: unify markdown for display and for upload
- [ ] BUG: list --format=json should merge nodes
- [ ] FEATURE: list available targets

## SHOULD CONSIDER

- [ ] 'list' could return hierarchical list (unless --flat), simply identical to 'get' without top item (default depth for list should probably be 1)
- [ ] document bookmarklet
- [ ] document rate limiting on updating reports
- [ ] FEATURE: add support for multiple reports, including "all"
- [ ] improve status and error reporting
  - [ ] status report for create, update is JSON -- should be similar to complete/uncomplete
- [ ] FEATURE: ranking reports should support topN _and_ thresholds


# DONE

## Next version
- [x] FEATURE: add delete command
- [x] BUG: errors now return non-zero exit code
- [x] BUG: when reports are uploaded, links are not created
- [x] FEATURE: add integration tests
- [x] BUG: create does not receive the location the api key on create
- [x] FEATURE: inform user of where to get api an api key if missing
  - [x] CODE: unify client creation code to single function
- [x] FEATURE: unify error and log messaging for consistency
- [x] improve status and error reporting: error reporting looks like a log mesage (time not necessary)
