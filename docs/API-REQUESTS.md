# Workflowy API Feature Requests

## Mirror Node Resolution

**Request:** Return mirror nodes with a reference to their original node (source ID and name), rather than as empty-name nodes with only their own ID. Additionally, include mirror relationship metadata (the `mirrors` array) in API responses, not just in backup files.

**Benefit:** API calls would return accurate content for every node, including mirrors. MCP servers and agents would see real content instead of blank entries. Tooling could analyze mirror usage patterns — e.g., nodes mirrored in many places may indicate organizational ambiguity.

## Short ID Lookup

**Request:** Add a query parameter (e.g., `short_id=xxxx`) to the List or Get endpoint that resolves a short hex ID (the 12-character suffix visible in Workflowy internal links) to its full 32-character UUID.

**Benefit:** Users could copy an internal link (`Cmd+Shift+L`), paste it into a script or CLI command, and immediately operate on that node. Currently this requires downloading and searching a full export, which is slow and rate-limited. Server-side resolution would make "pick a node in the UI → run code against it" instant.

## Image and Attachment Access

**Request:** Include image URLs or base64 data in node API responses, with an optional parameter to include/exclude.

**Benefit:** Tools like `workflowy-sync` (which lets an agent work from a Workflowy task tree) could access screenshots, diagrams, and mockups attached to nodes. API consumers would get complete node content rather than a text-only subset.

## Real-time Updates

**Request:** Webhooks or WebSocket support for change notifications on a node and its descendants.

**Benefit:** MCP servers and CLI tools would not need to poll for changes. The `workflowy-sync` skill currently polls every 10-300 seconds to detect new tasks — push notifications would make the collaboration loop near-instant.

## Bulk Operations

**Request:** Batch create/update/delete operations in a single API call (e.g., `POST /bulk` accepting an array of operations).

**Benefit:** Creating multiple nodes (e.g., splitting content into children or adding question options) currently requires N sequential API calls, which is slow and rate-limited. Atomic batches would be faster and more reliable.

## Enhanced Search

**Request:** Server-side search with regex support and date filters (e.g., `GET /search?q=...&regex=true&modified_after=...`), returning matching nodes with parent path context.

**Benefit:** Search currently requires fetching the entire tree and filtering client-side, which is inefficient for large outlines. Server-side search would be faster and reduce API load.

## Node History

**Request:** Access to node edit history (e.g., `GET /node/:id/history` returning timestamped versions).

**Benefit:** No way to see previous versions of a node or recover deleted content via API. History access would enable undo, audit trails, and change tracking in tooling.

## Rate Limit Headers

**Request:** Include standard `X-RateLimit-*` headers in responses showing remaining quota and reset time.

**Benefit:** Rate limiting is currently opaque. CLI and MCP tools cannot proactively back off before hitting limits, leading to failed requests and degraded user experience.
