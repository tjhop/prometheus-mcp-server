GOCMD := go
GOFMT := ${GOCMD} fmt
GOMOD := ${GOCMD} mod
BINARY := "prometheus-mcp-server"
RELEASE_CONTAINER_NAME := "${BINARY}"
GOLANGCILINT_CACHE := ${CURDIR}/.golangci-lint/build/cache
OLLAMA_MODEL ?= "ollama:qwen2.5-coder:3b"

## help:			print this help message
.PHONY: help
help: Makefile
	# autogenerate help messages for comment lines with 2 `#`
	@sed -n 's/^##//p' $<

## tidy:			tidy modules
tidy:
	${GOMOD} tidy

## fmt:			apply go code style formatter
fmt:
	${GOFMT} -x ./...

## lint:			run linters
lint:
	mkdir -p ${GOLANGCILINT_CACHE} || true
	podman run --rm -v ${CURDIR}:/app -v ${GOLANGCILINT_CACHE}:/root/.cache -w /app docker.io/golangci/golangci-lint:latest golangci-lint run -v

## binary:		build a binary
binary: fmt tidy lint
	goreleaser build --clean --single-target --snapshot --output .

## build:			alias for `binary`
build: binary

## test: 			run tests
test: fmt tidy lint
	go test -race -v ./...

## container: 		build container image with binary
container: binary
	podman image build -t "${RELEASE_CONTAINER_NAME}:latest" .

## image:			alias for `container`
image: container

## podman:		alias for `container`
podman: container

## docker:		alias for `container`
docker: container

## mcphost:		use mcphost to run the prometheus-mcp-server against a local ollama model
mcphost:
	mcphost --debug --config ./mcp.json --model "${OLLAMA_MODEL}"

## inspector:		use inspector to run the prometheus-mcp-server
inspector:
	npx @modelcontextprotocol/inspector --config ./mcp.json --server "${BINARY}"
