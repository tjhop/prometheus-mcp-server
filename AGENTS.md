# Agent Guide for Prometheus MCP Server

An MCP server that gives LLM clients typed tools and resources for working with a running Prometheus instance — PromQL queries, metric/series/label discovery, target and rule inspection, management endpoints, and gated TSDB-admin operations — plus custom tools and resources backed by an embedded, Bleve-indexed copy of the official Prometheus documentation. Compatible backends (e.g. Thanos) get tailored toolsets. See [README.md](README.md) for the full tool list, backend differences, install methods, auth, telemetry, and time-format handling.

## Development workflow

Use `Makefile` targets — not raw `go` commands. The Makefile wires submodules, formatting, tidy, lint, and tests as prerequisites, and drives `goreleaser` for builds.

| Task | Command |
|---|---|
| Verify code (fmt + tidy + test) | `make test` |
| Verify with lint too | `make lint test` |
| Full local build | `make build` |
| Release artifacts (containers, packages) | `make build-all` |
| List all targets | `make help` |

A single `make lint test` invocation runs each prereq once (no duplicated fmt/tidy) — use it as the default pre-commit check. `make build` does everything above *and* compiles the binary.

## Code layout (`pkg/mcp/`)

- `server.go` — `ServerContainer` (DI struct), middleware chain, HTTP + stdio transport.
- `tools.go` — `*mcp.Tool` definitions with annotations.
- `types.go` — tool input structs (`jsonschema` tags) and their `slog.LogValuer` implementations.
- `handlers.go` — tool handler methods on `*ServerContainer`.
- `registration.go` — toolset composition, per-backend toolsets (prometheus, thanos), `CoreTools` list.
- `resources.go` — MCP resources (metric list, targets, docs).
- `docs.go`, `docs_updater.go` — Bleve-indexed doc search with optional live auto-update.
- `middleware.go`, `errors.go`, `logging.go` — telemetry middleware, graceful 404 handling, MCP client logging.

Supporting: `pkg/prometheus/` (API client builder with custom User-Agent), `internal/metrics/` (metrics registry + namespace), `internal/version/`.

## Adding a new tool

1. **Input type** in `types.go`: struct with `jsonschema` tags and a `LogValue() slog.Value` method so structured logs group fields cleanly. Reuse `TimeRangeInput`, `TruncatableInput` where applicable.
2. **Tool definition** in `tools.go`: `*mcp.Tool` with `Annotations.ReadOnlyHint` *or* `Annotations.DestructiveHint` (use the `ptr(true)` helper — can't take the address of a constant). Zero-input tools use `InputSchema: emptyInputSchema`, not the SDK's `EmptyInput` — see issue [#119](https://github.com/tjhop/prometheus-mcp-server/issues/119) for the OpenAI strict-schema workaround.
3. **Handler** method on `*ServerContainer` in `handlers.go`, signature `(ctx, req, input) (*mcp.CallToolResult, any, error)`. `s.GetAPIClient(ctx)` returns both the prom Go client *and* the matching `http.RoundTripper` — use the client for endpoints it supports, and route everything else through `s.doHTTPRequest` (see "HTTP plumbing" below). Never reach for the default client/transport fields directly; that bypasses request-scoped auth.
4. **Register** in `initPrometheusToolset()` in `registration.go`. If Thanos doesn't support the tool, add its name to `thanosRemovedTools`. Leave `CoreTools` alone unless the tool is essential enough that it should still load even when a user explicitly narrows the toolset — the server already defaults to `--mcp.tools=all`.
5. **Test** in `handlers_test.go` (mock Prometheus lives in `api_mock_test.go`). `registration_test.go` validates toolset composition — it will fail loudly if the maps are inconsistent.
6. Update the tool table in `README.md`.

## Project-specific quirks/gotchas

- **Time inputs** accept epoch seconds, RFC3339, *or* Go duration strings relative to now (`5m`, `1h30m`). Use `ParseTimestampOrDuration` / `parseTimeWithDefault` — don't hand-parse.
- **HTTP plumbing.** `s.GetAPIClient(ctx)` returns both the prom Go client and the matching `http.RoundTripper`; `authContextMiddleware` makes the RoundTripper request-scoped when an `Authorization` header is present. For anything outside `promv1.API` — custom backends, management endpoints, new APIs — pull the RoundTripper and call `s.doHTTPRequest(ctx, method, rt, path, expectJSON)`, or `s.doManagementAPICall(ctx, method, path)` for `/-/...` endpoints. Both wrappers share the connection pool, emit `target_path`-labelled telemetry, and surface 404s as `ErrEndpointNotSupported`. References: `ThanosStoresHandler` for `/api/v1/...` calls, the Management API handlers for `/-/...`.
- **Docs state** in `ServerContainer` is guarded by `sync.RWMutex` because `--docs.auto-update` can swap the Bleve index at runtime. Read under `RLock`, write under `Lock`.
- **TSDB admin tools** (`delete_series`, `clean_tombstones`, `snapshot`) gate on the `--dangerous.enable-tsdb-admin-tools` flag and set `DestructiveHint`. `delete_series` additionally requires both `start_time` and `end_time` to avoid accidental full-data wipes.
- **Metrics** register through `metrics.Registry` and use `metrics.MetricNamespace` via `prometheus.BuildFQName` — don't register globally.
- **Backend toolsets** derive from the base Prometheus toolset: each backend has a `<backend>RemovedTools` list and an `init<Backend>Toolset` initializer that prunes unsupported tools and adds any backend-specific ones (see `initThanosToolset` for the canonical example). To add a new backend, extend `PrometheusBackends`, add the removal list + initializer, and wire it into `getToolset`'s switch — don't fork a parallel map.
- **TOON output mode** is a server flag affecting how results are serialized to clients — not usually relevant when editing handler logic, but don't assume JSON shape in end-to-end tests.

## Testing

`*_test.go` files sit alongside the code they test. `handlers_test.go` is where most new tool tests land; use `api_mock_test.go`'s mock Prometheus. Run `make test` often — it's fast.
