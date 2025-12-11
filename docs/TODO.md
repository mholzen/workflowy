# TODO

## SHOULD DO

- [ ] BUG: when reports are uploaded, links are not created
- [ ] BUG: strip html tags when creating links in reports, as they are not recognized when pasting to WF
- [ ] FEATURE: add last modified to count report
- [ ] FEATURE: search and replace
- [ ] FEATURE: build an MCP server
  - [ ] CODE: unify markdown for display and for upload
- [ ] BUG: list --format=json should merge nodes

## SHOULD CONSIDER

- [ ] default depth for list should probably be 1
- [ ] document bookmarklet
- [ ] document rate limiting on updating reports
- [ ] FEATURE: add support for multiple reports, including "all"
- [ ] improve status and error reporting
  - [ ] status report for create, update is JSON -- should be similar to complete/uncomplete

- [ ] FEATURE: ranking reports should support topN _and_ thresholds


# DONE

## Next version

- [x] FEATURE: inform user of where to get api an api key if missing
  - [x] CODE: unify client creation code to single function
- [x] FEATURE: unify error and log messaging for consistency
- [x] improve status and error reporting: error reporting looks like a log mesage (time not necessary)
