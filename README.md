# Prometheus MCP Server

## About
This is an MCP server to allow LLMs to interact with a running Prometheus instance via the API to do things like generate and execute promql queries, list and analyze metrics, etc. The full list of tools supported can be found with the MCP tool list endpoint or the `/tools` command in servers like `mcphost`.

## Usage

See [`mcp.json`](./examples/mcp.json) for an example MCP config for use with tooling.

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

## Features

- [ ] Check Prometheus health
- [ ] List metrics
- [ ] List labels
- [ ] Query support
    - [X] Instant queries
    - [ ] Range queries
    - [ ] Query builder
- [X] Get TSDB stats/info
- [ ] Get scrape target health/info

## Development

Required software:
- Working Go environment
- Podman for local tests/linting/etc
- GNU Make

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
