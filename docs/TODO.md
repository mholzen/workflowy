# TODO

## SHOULD DO

- FEATURE: Add --name and --note flags to search and replace commands to allow
  searching/replacing in notes (currently only operates on name)
- FEATURE: add a more general purpose Dockerfile for easy installation from agents
- BUG: strip html tags when creating links in reports, as they are not recognized when pasting to WF
- BUG: rename 'markdown' format to 'document' to avoid confusion with 'list'
- CODE: unify markdown for display and for upload
- BUG: list --format=json should merge nodes
- FEATURE: implement the move command
- BUGL: support multiple flags with a single dash (eg. search -iE '...')

## SHOULD CONSIDER

- CODE: Extract shared parameter structures for CLI and MCP feature parity
  - Motivation: CLI features (like split transform's --separator flag) can be added
    without corresponding MCP tool parameters, causing feature drift
  - Solution: Create shared config structs in `pkg` layer that both CLI flags and
    MCP tool parameters populate, ensuring both interfaces expose the same capabilities
- 'list' could return hierarchical list (unless --flat), simply identical to
  'get' without top item (default depth for list should probably be 1)
- document bookmarklet
- document rate limiting on updating reports
- FEATURE: add support for multiple reports, including "all"
- improve status and error reporting
  - status report for create, update is JSON -- should be similar to complete/uncomplete
- FEATURE: ranking reports should support topN _and_ thresholds
