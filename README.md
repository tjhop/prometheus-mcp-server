# Prometheus MCP Server

## About
This is an MCP server to allow LLMs to interact with a running Prometheus instance via the API to do things like generate and execute promql queries, list and analyze metrics, etc.

### Tools

| Tool Name | Description |
| --- | --- |
| `alertmanagers` | Get overview of Prometheus Alertmanager discovery |
| `build_info` | Get Prometheus build information |
| `config` | Get Prometheus configuration |
| `exemplars_query` | Performs a query for exemplars by the given query and time range |
| `flags` | Get runtime flags |
| `label_names` | Returns the unique label names present in the block in sorted order by given time range and matchers |
| `label_values` | Performs a query for the values of the given label, time range and matchers |
| `list_alerts` | List all active alerts |
| `list_rules` | List all alerting and recording rules that are loaded |
| `list_targets` | Get overview of Prometheus target discovery |
| `metric_metadata` | Returns metadata about metrics currently scraped by the metric name | 
| `query` | Execute an instant query against the Prometheus datasource |
| `range_query` | Execute a range query against the Prometheus datasource |
| `runtime_info` | Get Prometheus runtime information |
| `series` | Finds series by label matchers |
| `targets_metadata` | Returns metadata about metrics currently scraped by the target |
| `tsdb_stats` | Get usage and cardinality statistics from the TSDB |
| `wal_replay_status` | Get current WAL replay status |

__NOTE:__ For the time being, the [TSDB Admin API endpoints](https://prometheus.io/docs/prometheus/latest/querying/api/#tsdb-admin-apis) aren't implemented. Both because of the potential for damage, as well as the fact that these endpoints are not enabled unless prometheus is started with an additional flag. Implementation requires more thought/consideration.

| Tool Name | Description |
| --- | --- |
| `clean_tombstones` | Removes the deleted data from disk and cleans up the existing tombstones |
| `delete_series` | deletes data for a selection of series in a time range |
| `snapshot` | creates a snapshot of all current data into snapshots/<datetime>-<rand> under the TSDB's data directory and returns the directory as response |

### Resources

_Not implemented yet_

### Prompts

_Not implemented yet_

## Usage

### Local LLM with Ollama
See [`mcp.json`](./examples/mcp.json) for an example MCP config for use with tooling.
Requires [`ollama`](https://github.com/ollama/ollama) to be installed.

<details>
<summary>Using MCP Inspector and a local ollama instance:</summary>

Requires [MCP Inpsector](https://github.com/modelcontextprotocol/inspector) to be installed:

```bash
make inspector
```
</details>

<details>
<summary>Using mcphost and a local ollama instance:</summary>

Requires [`mcphost`](https://github.com/mark3labs/mcphost) to be installed:

```bash
make mcphost
```
</details>

## Development
### Development Environment with Devbox + Direnv
If you use [Devbox](https://www.jetify.com/devbox) and
[Direnv](https://direnv.net/), then simply entering the directory for the repo
should set up the needed software.

### Manual Setup
Required software:
- Working Go environment
- Podman for local tests/linting/etc
- GNU Make
- [ollama](https://github.com/ollama/ollama)
- [mcp inspector](https://github.com/modelcontextprotocol/inspector)
- [mcphost](https://github.com/mark3labs/mcphost)

```bash
~/go/src/github.com/tjhop/prometheus-mcp-server (main [ ]) -> make
# autogenerate help messages for comment lines with 2 `#`
 help:                  print this help message
 tidy:                  tidy modules
 fmt:                   apply go code style formatter
 lint:                  run linters
 binary:                build a binary
 build:                 alias for `binary`
 test:                  run tests
 container:             build container image with binary
 image:                 alias for `container`
 podman:                alias for `container`
 docker:                alias for `container`
 mcphost:               use mcphost to run the prometheus-mcp-server against a local ollama model
 inspector:             use inspector to run the prometheus-mcp-server
```
