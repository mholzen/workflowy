# Workflowy API Selection Design

## Summary

Allow users to choose whether network operations use Workflowy's production or beta API. The production API remains the default. The CLI exposes the choice through `--api`, while MCP supports both a server-wide `--api` default and an optional `api` argument on each network-capable tool.

This change selects the Workflowy deployment only. It does not normalize the different response contracts or add mirror-specific behavior. Those changes can build on this selection interface later.

## Motivation

The Workflowy production and beta deployments expose the same endpoint paths under different hosts, but their capabilities and response contracts differ. Most notably, the beta API supports mirrors and exposes `data.mirror.origin_id` when retrieving a mirror node, while the current production API does not. Users need an explicit way to run the CLI and MCP tools against either deployment while the beta contract evolves.

References:

- Production: <https://workflowy.com/api-reference/#nodes-retrieve>
- Beta: <https://beta.workflowy.com/api-reference/#nodes-retrieve>

## User Interface

### CLI

All commands that can construct a Workflowy network client accept:

```text
--api <production|beta>
    Workflowy API deployment to use (default: production)
```

Examples:

```bash
workflowy get <id> --api=beta
workflowy create "Node" --api=production
workflowy mcp --api=beta
```

The flag follows the existing command-local flag convention, so it can appear after the subcommand. Commands that are entirely local and never construct a Workflowy client do not need the flag.

The flag is present on `get`, `list`, `create`, `update`, `move`, `delete`, `complete`, `uncomplete`, `targets`, `search`, `replace`, `transform`, `id`, `mcp`, and every `report` subcommand. Reports receive it because they may load data from the API or upload their result, even when a particular invocation reads its input from a backup. `version` is the only current command that does not receive it.

### MCP

The MCP server accepts `--api=production|beta`. This establishes the default deployment for all tools hosted by that process.

Every MCP tool that can make a Workflowy network request accepts an optional string argument constrained by its schema to the enum values `production` and `beta`:

```json
{
  "api": "beta"
}
```

The per-tool argument overrides the server default for the complete tool invocation, including ID resolution, access validation, reads, and writes. The precedence is:

```text
tool api argument > MCP server --api flag > production
```

Tools that exclusively read a local backup, such as the current mirror report, do not expose a meaningless per-tool `api` argument. The MCP server may still be started with `--api` when those tools are enabled alongside network-capable tools.

The argument is present on `workflowy_get`, `workflowy_list`, `workflowy_search`, `workflowy_targets`, `workflowy_id`, `workflowy_create`, `workflowy_update`, `workflowy_move`, `workflowy_delete`, `workflowy_complete`, `workflowy_uncomplete`, `workflowy_report_count`, `workflowy_report_children`, `workflowy_report_created`, `workflowy_report_modified`, `workflowy_replace`, and `workflowy_transform`. It is absent only from the current backup-only `workflowy_report_mirrors` tool.

### Relationship to Access Method

`api` and `method` describe separate decisions:

- `api` chooses the Workflowy deployment: `production` or `beta`.
- `method` chooses the data access mechanism: `get`, `export`, or `backup`.

`method=backup` selects the local source for reading and access validation; it does not imply that the entire command is read-only. A read-only backup invocation can remain fully offline, while a mutation, transform application, replacement, or report upload still sends writes through the API selected by `api`. The combination remains valid, and `api` is meaningful whenever any part of the invocation performs a network request.

## Domain Model

The API deployment is a closed value with two valid members:

| Value | Base URL |
| --- | --- |
| `production` | `https://workflowy.com/api/v1` |
| `beta` | `https://beta.workflowy.com/api/v1` |

The canonical user-facing values are the full words `production` and `beta`. Aliases such as `prod`, arbitrary URLs, and environment names such as `staging` are not accepted.

Deployment selection is not an API version. Both deployments currently use the `/api/v1` path, so naming the option `api_version` would establish the wrong semantics.

## Module Design

### Deployment Selection Module

A small Workflowy package module owns:

- the valid deployment values;
- parsing and validation;
- the mapping from deployment to base URL;
- the production default.

Its interface returns a categorized deployment value or a contextual error. Callers do not construct or compare base URLs themselves. This concentrates host knowledge at one seam and prevents CLI and MCP behavior from drifting.

### Client Construction

Workflowy client construction accepts a validated deployment and authentication options. The generic HTTP client continues to accept a base URL, but only the Workflowy-specific constructor performs the deployment-to-host mapping.

The CLI validates its `--api` value before constructing one client for the command. MCP retains clients for both deployments, using the same resolved API key, because any tool invocation may override the server default. Client construction performs no network request.

At the start of an MCP invocation, the tool builder resolves the requested deployment and selects the corresponding client for any network work. Selection must happen before network-backed ID resolution and read/write guard checks so every network request in one invocation goes to the same deployment. Local backup reads continue to use the backup provider.

The deployment value must be passed explicitly through this seam. It must not be stored in a mutable global or hidden in a context value.

CLI command construction and the MCP tool builder accept an internal client factory whose interface maps a validated deployment to a `workflowy.Client`. The production adapter constructs clients from the fixed deployment mapping. Tests replace the adapter with recording clients or local HTTP clients, so they can verify selection without DNS changes or real network requests. This is an internal testing seam, not a user-facing custom-host option.

### MCP Restriction Roots

`read_root_id` and `write_root_id` must be resolved against the same source used for access validation. Resolving either value once with the server default and reusing the result after a tool-level override would violate the promise that all network activity in an invocation uses one deployment. Always resolving through a network client would also break offline backup reads.

The MCP server therefore retains the raw configured root values and resolves the effective access method before resolving roots:

- With `get` or `export`, root target keys and short IDs resolve through the selected deployment and are verified against the selected deployment's export tree.
- With `backup`, full UUIDs are verified in the backup tree and short IDs are resolved by matching that tree. This path performs no network request.
- A target-key restriction cannot be resolved from the current backup format. A backup invocation using one fails contextually and instructs the user to configure the restriction with a full or short node ID. It does not silently contact either API.

The resulting UUIDs exist only for that invocation and are never reused by an invocation selecting a different deployment or validation source. Resolution errors identify the root value and either the selected deployment or backup file. A root that resolves in production but not beta prevents only beta-selected invocations; it does not prevent an otherwise production-configured MCP server from starting. Export caching already limits repeated network-based short-ID resolution requests, and additional root memoization is outside this feature until measurements show it is needed.

### Export Cache Isolation

Production and beta export responses must not share a cache entry. Otherwise a beta request could return production data without making a request, or the reverse.

Each Workflowy client owns an explicit export-cache path selected when that client is constructed:

| Deployment | Cache file |
| --- | --- |
| `production` | `~/.workflowy/export-cache.json` |
| `beta` | `~/.workflowy/export-cache-beta.json` |

Keeping the existing path for production preserves current behavior and existing caches. Beta receives a new isolated path. The cache module's read and write interfaces accept the path explicitly; they do not consult package-global deployment state. Cache reads, stale-cache fallback, writes, and export rate-limit timing all use the path owned by the selected client. Tests supply temporary paths to verify isolation without touching the user's cache.

Backup-file access is deployment-independent and remains unchanged.

## Data Flow

### CLI Command

1. Parse `--api`, defaulting to `production`.
2. Validate the value and map it to a deployment.
3. Resolve the API key using the existing precedence rules.
4. Construct a Workflowy client for the deployment.
5. Run ID resolution, access validation, and the requested operation with that client.
6. If the operation uses export, read and write only that deployment's cache.

### MCP Tool Invocation

1. Parse and validate the server's `--api` during startup, defaulting to `production`.
2. Construct authenticated clients for production and beta.
3. Retain configured read and write root values without resolving them against either deployment.
4. For each tool call, read the optional `api` and `method` arguments.
5. Resolve the effective deployment and access method using their documented precedence rules.
6. Select the corresponding client and the validation source for the effective method.
7. Resolve active read and write roots against that validation source.
8. Use those per-invocation roots for validation and the selected client for every network operation made by the tool call.

## Validation and Error Handling

Invalid CLI and MCP values fail without making a network request. New user-facing errors follow the repository convention and include the rejected value:

```text
Cannot select Workflowy API "staging": expected "production" or "beta"
```

CLI validation returns the error from the command. An invalid MCP server default prevents startup. An invalid per-tool override returns an MCP tool error and leaves the server running.

An MCP restriction root that cannot be resolved returns a contextual tool error such as:

```text
Cannot resolve read root "inbox" using Workflowy API "beta": <cause>
```

A target-key root requested with backup remains offline and reports the corrective action:

```text
Cannot resolve read root "inbox" from backup "<path>": target keys require an API; configure read_root_id with a full or short node ID
```

Network errors continue to use the existing operation-specific wrapping. Debug logging includes the selected deployment and request path, but authentication material is never logged.

## Testing

Unit tests cover:

- an omitted value resolves to `production`;
- `production` maps to `https://workflowy.com/api/v1`;
- `beta` maps to `https://beta.workflowy.com/api/v1`;
- invalid values return a contextual error containing the rejected value and valid choices;
- CLI commands send requests to the selected test server adapter;
- MCP uses its server default when a tool omits `api`;
- an MCP tool argument overrides the server default;
- all network requests within an MCP invocation, including network-backed ID resolution and access validation, use the selected client;
- read and write roots are resolved with the effective per-invocation validation source;
- a root-resolution failure in beta does not prevent production invocations or MCP startup;
- full and short restriction IDs resolve locally when validation uses a backup;
- a target-key restriction with backup fails contextually without making a network request;
- production and beta exports use different cache files;
- the production cache retains its existing path;
- two clients in the same process cannot read or write each other's cache entries;
- a read-only backup invocation using locally resolvable IDs performs no network request regardless of API selection;
- writes and report uploads use the selected deployment even when their read or validation source is a backup.

Existing production-default tests must continue to pass. Test coverage must not decrease.

Integration tests cover at least one read-only command against each deployment when credentials and network access are explicitly available. They belong under the existing read-only integration target; this feature does not add destructive integration coverage.

## Documentation

The same change updates:

- the README's CLI and MCP setup examples;
- `docs/CLI.md`, including the option, valid values, default, and interaction with `method`;
- `docs/MCP.md`, including server-wide configuration, per-tool overrides, and precedence;
- MCP tool schemas for every network-capable tool.

The documentation must continue to describe production as the default and beta as opt-in.

## Alternatives Rejected

### Boolean Beta Flag

`--beta-api` and `beta_api` are initially short but create asymmetric negation and do not model two named deployments cleanly.

### Arbitrary Base URL

`--api-base-url` exposes infrastructure details, expands the supported surface to unrequested custom endpoints, and makes validation and documentation weaker.

### API Environment Name

`--api-environment` is accurate but unnecessarily verbose. `api` is clear when its closed set of values is shown in help and MCP schemas.

## Non-Goals

- Normalizing production and beta node response shapes.
- Implementing new mirror behavior.
- Automatically detecting which deployment supports an operation.
- Falling back from one deployment to the other after an error.
- Supporting arbitrary Workflowy hosts or custom base URLs.
- Selecting separate API keys per deployment.
