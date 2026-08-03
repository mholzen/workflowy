# HTTP Debug Logging Design

## Summary

Log every Workflowy HTTP request at debug level from the generic HTTP client's request seam. The log record includes the HTTP method and request path but never includes authentication headers or request bodies.

## Motivation

The current uncommitted implementation enables HTTP request logging through a context value set only by the CLI's GET fetch path. As a result, export calls, writes, report uploads, MCP operations, and other HTTP requests do not produce the same diagnostic record. HTTP logging is a transport concern and should be implemented consistently where all requests pass through the client.

## Behavior

`Client.Do` emits this structured record immediately before sending every successfully constructed HTTP request:

```text
DEBUG: http request (method='GET', path='/nodes/example')
```

The call to `slog.Debug` is unconditional. The configured logging handler decides whether the record is emitted:

- `--log=debug` emits the record.
- The default `--log=info` and higher levels suppress it.

The record includes only:

- `method`: the HTTP method passed to `Client.Do`;
- `path`: the relative request path passed to `Client.Do`.

It must not include the authorization header, API key, request body, or response body.

This applies uniformly to CLI and MCP requests, including GET, export, create, update, move, complete, uncomplete, delete, target-listing, report-upload, replace, and transform operations.

## Module Design

The generic HTTP client is the single module responsible for the diagnostic record because every Workflowy network operation crosses its `Do` interface. Callers do not opt in through context values and do not need to know how HTTP logging is implemented.

The context helpers `WithDebugHTTPLogging` and `DebugHTTPLoggingEnabled` are removed, along with the GET-specific call in `fetchItems`. No logging transport wrapper or additional configuration type is introduced.

The existing logging configuration remains the only user-facing control. This keeps the interface small and avoids duplicating log-level state in request contexts.

## Data Flow

1. A CLI or MCP operation calls a Workflowy client method.
2. The Workflowy client method calls `Client.Do` with a method and relative path.
3. `Client.Do` encodes the body, constructs the request, sets headers, and applies authentication as before.
4. Immediately before calling the HTTP transport, `Client.Do` submits the structured debug record to `slog`.
5. The configured handler emits or suppresses the record according to its level.
6. `Client.Do` sends the HTTP request as before.

Logging does not alter request construction, authentication, response decoding, or error handling.

## Error Handling

Logging introduces no new error path. `slog.Debug` does not return an error, and request errors continue through the existing client behavior unchanged.

## Testing

Unit tests verify:

- GET, POST, and DELETE calls submit a record containing the method and path when the logger accepts debug records;
- the same calls do not appear when the logger threshold is info;
- no context marker is required;
- request execution and response handling remain unchanged.

Tests that replace the process-global default logger restore it with `t.Cleanup` and do not run in parallel.

The GET-specific context-propagation test is removed because that interface no longer exists. The full unit suite and `go vet ./...` must pass.

## Documentation

Existing `--log` documentation already describes the `debug` level. Update both `docs/CLI.md` and `docs/MCP.md` with a concise troubleshooting example showing that `--log=debug` reports HTTP method and path for all API operations. No new CLI or MCP parameter is introduced.

## Non-Goals

- Logging headers, API keys, or bodies.
- Logging response bodies.
- Adding a second HTTP-logging flag.
- Changing the format of non-HTTP log records.
- Fixing the separate curl helper's credential-handling or error-reporting issues.
