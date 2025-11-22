- read data from a backup file and use it for retrieval operations, instead of the API
  - add options --use-backup-file[=<filename>], which defaults to using the latest file without a filename
    or uses the provided filename
  - use workflowy-size-report/pkg/workflowy/backup.go for this

- add command that generates markdown from a node and its children
  - this function will use logical defaults at first, but might be configurable in the future
  - use the layoutMode to determine the markdown decorator (header, bullets, etc...)
  - children node of a header node will be paragraphs
  - special tags in the name can affect the behaviour
    `#exclude` omits that node from the markdown output
    `#h1`, `#h2`, ... override the layout mode of the node (so that it is possible to create a document entirely with bullet points in workflowy but control the formatting output in Markdown)
  - automatically apply styling to headers:
    - h1 might be uppercased
    - nodes with bullet layout that are treated as paragraphs might capitalized and terminated with a period
  - this code will probably require a new package.  we do have workflowy-size-report/pkg/markdown/ but it pretty
    trivial.  Nonetheless, start there and see what we can reuse.
  - an important part of this change is configurability, as it will require trial and error to get it right.
    - we probably should have a series of test documents in workflowy that we can download to test the markdown
      generator

- add a command that converts markdown to workflowy nodes (essentially the reverse of the previous feature),
  so that it can be used by the next feature, which will be to integrate the report generators from workflowy-size-report/pkg/workflowy/{count,ranking}.go and upload them directly to workflowy using the API.

- add command "report {count|children|modified|created|all}" that generates reports
  - by default, they will be output to stdout
  - add an option --upload which will use the report generated as nodes and use the API to upload directly
