GOCMD := go
GOFMT := ${GOCMD} fmt
GOMOD := ${GOCMD} mod
BINARY := "prometheus-mcp-server"
RELEASE_CONTAINER_NAME := "${BINARY}"
GOLANGCILINT_CACHE := ${CURDIR}/.golangci-lint/build/cache
OLLAMA_MODEL ?= "ollama:qwen2.5-coder:3b"
OPENWEBUI_VERSION ?= "v0.6.15"

.PHONY: help
help: ## print this help message
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-z0-9A-Z_-]+:.*?##/ { printf "  \033[36m%-30s\033[0m%s\n", $$1, $$2 }' $(MAKEFILE_LIST)

tidy: ## tidy modules
	${GOMOD} tidy

fmt: ## apply go code style formatter
	${GOFMT} -x ./...

lint: ## run linters
	mkdir -p ${GOLANGCILINT_CACHE} || true
	# convert this to use golangic-lint from devbox, rather than podman
	podman run --rm -v ${CURDIR}:/app -v ${GOLANGCILINT_CACHE}:/root/.cache -w /app docker.io/golangci/golangci-lint:latest golangci-lint run -v

binary: fmt tidy lint ## build a binary
	goreleaser build --clean --single-target --snapshot --output .

build: binary ## alias for `binary`

test: fmt tidy lint ## run tests
	go test -race -v ./...

container: binary ## build container image with binary
	podman image build -t "${RELEASE_CONTAINER_NAME}:latest" .

image: container ## alias for `container`

podman: container ## alias for `container`

docker: container ## alias for `container`

mcphost: build ## use mcphost to run the prometheus-mcp-server against a local ollama model
	mcphost --debug --config ./mcp.json --model "${OLLAMA_MODEL}"

inspector: build ## use inspector to run the prometheus-mcp-server
	npx @modelcontextprotocol/inspector --config ./mcp.json --server "${BINARY}"

open-webui: build ## use open-webui to run the prometheus-mcp-server
	podman run --rm -d -p 11119:8080 --add-host=host.docker.internal:host-gateway -v open-webui:/app/backend/data --name open-webui "ghcr.io/open-webui/open-webui:${OPENWEBUI_VERSION}"
	uvx mcpo --port 18000 -- "./${BINARY}" 
