- list currently includes children, remove for consistency
- document bookmarklet
- document rate limiting on updating reports

- document that rationale behind generating markdown from nodes without formatting
  - write a test that creates nodes in various document styles (without layout) and compare their output with expected content:  https://workflowy.com/#/394fe0f16b5f
    - with correct layoutMode, formatting from layout
    - with layoutMode as tags, formatting from tags
    - with no layout and no tags, formatting derived

- consider using markdown to nodes for uploading reports.

- add support for multiple reports, including "all"

- improve status and error reporting
  - error reporting looks like a log mesage (time not necessary)
  - status report for update is JSON, probably unnecessary

- ranking reports should support topN _and_ thresholds