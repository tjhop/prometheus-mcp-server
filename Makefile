GOCMD := go
GOFMT := ${GOCMD} fmt
GOMOD := ${GOCMD} mod
BINARY := prometheus-mcp-server
GOLANGCILINT_CACHE := ${CURDIR}/.golangci-lint/build/cache
OLLAMA_MODEL ?= ollama:gpt-oss:20b
OPENWEBUI_VERSION ?= v0.6.15

.PHONY: help
help: ## print this help message
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-z0-9A-Z_-]+:.*?##/ { printf "  \033[36m%-30s\033[0m%s\n", $$1, $$2 }' $(MAKEFILE_LIST)

submodules: ## ensure git submodules are initialized and updated
	git submodule update --init --remote --recursive

tidy: ## tidy modules
	${GOMOD} tidy

fmt: ## apply go code style formatter
	${GOFMT} -x ./...

lint: ## run linters
	golangci-lint run -v

binary: submodules fmt tidy lint test ## build a binary
	goreleaser build --clean --single-target --snapshot --output .

build: binary ## alias for `binary`

build-all: submodules fmt tidy lint ## test release process with goreleaser, does not publish/upload
	docker run --privileged --rm tonistiigi/binfmt --install all
	goreleaser release --snapshot --clean

container: build-all ## build container images with goreleaser, alias for `build-all`

image: build-all ## build container images with goreleaser, alias for `build-all`

test: fmt tidy ## run tests
	go test -race -v ./...

mcphost: build ## use mcphost to run the prometheus-mcp-server against a local ollama model
	mcphost --debug --config ./mcp.json --model "${OLLAMA_MODEL}"

inspector: build ## use inspector to run the prometheus-mcp-server in STDIO transport mode
	npx @modelcontextprotocol/inspector --config ./mcp.json --server "${BINARY}"

inspector-http: ## use inspector to run the prometheus-mcp-server in streamable HTTP transport mode
	npx @modelcontextprotocol/inspector --config ./mcp.json --server "${BINARY}-http"

open-webui: build ## use open-webui to run the prometheus-mcp-server
	docker run --rm -d -p 11119:8080 --add-host=host.docker.internal:host-gateway -v open-webui:/app/backend/data --name open-webui "ghcr.io/open-webui/open-webui:${OPENWEBUI_VERSION}"
	uvx mcpo --port 18000 -- "./${BINARY}" 

gemini: build ## use gemini-cli to run the prometheus-mcp-server against Google Gemini models
	npx @google/gemini-cli
