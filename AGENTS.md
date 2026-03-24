# Agent Guide for Prometheus MCP Server

This file provides guidance to Claude Code, Codex, and other tools when working with code in this repository.

## Project Overview

This is an MCP (Model Context Protocol) server that allows LLMs to interact with running Prometheus instances via the Prometheus HTTP API. The server provides tools for executing PromQL queries, analyzing metrics, managing targets, reading official Prometheus documentation, and much more.

For detailed user-facing documentation, see the project [README.md](README.md). Consult it when you need specifics on:
- Full tool list, tool sets, and tool selection flags, when more information is needed than what is provided in the tool schema
- Compatible Prometheus backends (Thanos, etc.) and their tool differences
- MCP resource details beyond what is provided in the resource schema
- Command-line flags and environment variables
- Installation methods (binary, Docker, Helm, system packages)
- Security and authentication configuration
- Telemetry: metrics, Grafana dashboard, logging
- Time format handling for tool inputs

## Required Knowledge

To successfully work on this project, you must be an expert in the following domains:
- Software engineering best practices
- Go programming language (golang)
- Prometheus and PromQL
- Observability and monitoring systems
- Telemetry and alerting
- Metrics and time series databases
- Large Language Models (LLMs)
- Model Context Protocol (MCP)

## Development

Always favor using `Makefile` targets over their raw command equivalents. The Makefile exists to reproducibly automate build steps and ensure consistency across development environments.

```bash
# View all targets
make help

# Build binary (runs submodules, fmt, tidy, lint, test first)
make binary
# or
make build

# Format code
make fmt

# Run linter
make lint

# Run tests
make test

# Build all release artifacts (containers, packages, etc)
make build-all
```

When working on this project, you will primarily be working with tests (Go tests and any other test harnesses needed). Use the Makefile targets above to build, format, lint, and test your changes.

## Architecture

### High-Level Structure

The project is organized into three main layers:

1. **Entry Point** (`cmd/prometheus-mcp-server/main.go`)
   - Initializes the MCP server with middlewares
   - Sets up HTTP server for metrics and optional HTTP transport
   - Uses `oklog/run` for goroutine orchestration and graceful shutdown
   - Supports both stdio and HTTP transports for MCP communication

2. **MCP Server Layer** (`pkg/mcp/`)
   - **server.go**: Core MCP server initialization, ServerContainer, and middleware chain
   - **tools.go**: Tool definitions and toolset management
   - **handlers.go**: Tool handler implementations
   - **resources.go**: MCP resource definitions (metrics list, targets, docs)
   - **registration.go**: Tool and resource registration with the MCP server
   - **docs.go**: Documentation search and retrieval functionality
   - **docs_updater.go**: Live documentation auto-update from prometheus/docs repository
   - **middleware.go**: Telemetry middleware for metrics and logging
   - **errors.go**: Custom error types for graceful 404 handling
   - **logging.go**: MCP client logging support
   - **types.go**: Input/output type definitions for tools

3. **Supporting Packages**
   - **pkg/prometheus/**: Prometheus API client creation with custom User-Agent
   - **internal/metrics/**: Prometheus metrics registry and instrumentation
   - **internal/version/**: Build version information

### Server Architecture

The MCP server uses a `ServerContainer` pattern for dependency injection. This provides explicit dependency management for tool and resource handlers.

#### ServerContainer

The `ServerContainer` holds all dependencies needed by handlers:

- **Core Dependencies**: Logger, default Prometheus API client, RoundTripper
- **Configuration**: Truncation limit, TOON output mode, API timeout, client logging
- **Documentation State**: Protected by `sync.RWMutex` for concurrent reads and live updates

Dependencies are injected via `ServerContainer` rather than context. Tool handlers receive the container directly through closures during registration. The only context-carried data is the authorization header from HTTP requests (for auth forwarding).

#### Middleware

The server uses a telemetry middleware that instruments tool/resource calls with duration and failure metrics, and provides debug logging.

#### Auth Forwarding

For the HTTP transport, an `authContextMiddleware` extracts Authorization headers from incoming requests and adds them to the request context. `GetAPIClient()` creates request-specific API clients when auth is present, falling back to the default client otherwise.

### Prometheus API Interaction

The server interacts with Prometheus in two ways:

1. **Via prometheus/client_golang** (`pkg/prometheus/api.go`)
   - Used for standard Prometheus API endpoints
   - Client created with custom User-Agent
   - Supports Prometheus HTTP config for TLS/auth

2. **Via direct HTTP requests** (`handlers.go`)
   - Used for non-standard endpoints or backend-specific APIs
   - Reuses RoundTripper from ServerContainer for consistent auth

## Testing

Test files follow Go conventions with `_test.go` suffix. Key test files and patterns:

- **handlers_test.go**: Comprehensive tool handler tests
- **api_mock_test.go**: Mock Prometheus API for testing
- **registration_test.go**: Toolset loading validation
- **middleware_test.go**: Telemetry middleware tests
- **docs_updater_test.go**: Documentation auto-update tests
- **errors_test.go**: 404 detection and error wrapping tests
- **logging_test.go**: Structured logging tests

Tests use mock API responses and validate both success and error cases.

## Configuration Files

- **devbox.json**: Devbox development environment dependencies
- **go.mod**: Go module dependencies (requires Go 1.26.0+)
- **.goreleaser.yaml**: Multi-platform build configuration (Linux, macOS, Windows, FreeBSD, ARM variants)
- **examples/mcp.json**: Example MCP server configuration for tooling integration
- **examples/.gemini/settings.json**: Gemini CLI configuration example
- **examples/http-config.yml**: Prometheus HTTP client config (TLS, auth)
- **examples/k8s-deployment.yml**: Example Kubernetes deployment manifest
- **examples/openshift-deployment.yml**: Example OpenShift deployment manifest

## Key Dependencies

- **modelcontextprotocol/go-sdk**: Official MCP Go SDK
- **blevesearch/bleve/v2**: Full-text search for documentation
- **go-git/go-git/v5**: Git operations for docs auto-update
- **prometheus/client_golang**: Prometheus API client and metrics
- **alpkeskin/gotoon**: TOON encoding for token-efficient output
- **tmc/langchaingo**: Markdown text splitting for documentation chunking

## Embedded Assets

- **Prometheus Documentation**: Git submodule at `cmd/prometheus-mcp-server/external/docs/docs`
  - Updated with: `make submodules`
  - Indexed for full-text search using Bleve
  - Supports live auto-update via `--docs.auto-update` flag

- **MCP Instructions**: `pkg/mcp/assets/instructions.md`
  - Embedded instructions for LLMs on how to use the Prometheus MCP server
  - Loaded at server startup and provided to LLM clients

## Git Submodules

The project uses git submodules for embedding Prometheus documentation:

```bash
# Initialize/update submodules
git submodule update --init --remote --recursive
# or
make submodules
```

## Release Process

Releases are automated via GoReleaser:

- Multi-platform binaries (Linux, macOS, Windows)
- Container images (amd64, arm64)
- System packages (deb, rpm, archlinux)
- Systemd service files included in packages
