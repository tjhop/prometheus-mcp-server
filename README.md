# Prometheus MCP Server

## About
This is an MCP server to allow LLMs to interact with a running Prometheus instance via the API to do things like generate and execute promql queries, list and analyze metrics, etc.

### Tools

| Tool Name | Description |
| --- | --- |
| `alertmanagers` | Get overview of Prometheus Alertmanager discovery |
| `build_info` | Get Prometheus build information |
| `execute_query` | Execute an instant query against the Prometheus datasource |
| `flags` | Get runtime flags |
| `list_alerts` | List all active alerts |
| `list_rules` | List all alerting and recording rules that are loaded |
| `list_targets` | Get overview of Prometheus target discovery |
| `runtime_info` | Get Prometheus runtime information |
| `tsdb_stats` | Get usage and cardinality statistics from the TSDB |
| `wal_replay_status` | Get current WAL replay status |

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
