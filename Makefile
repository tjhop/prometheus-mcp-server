# Copyright The Prometheus Authors
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Needs to be defined before including Makefile.common to auto-generate targets
DOCKER_ARCHS ?= amd64 armv7 arm64

include Makefile.common

DOCKER_IMAGE_NAME ?= prometheus-mcp

# Pinned snapshot of github.com/prometheus/docs, embedded into the binary via
# go:embed. Bump DOCS_VERSION to a new commit or tag and run `make docs` (or any
# build target) to refresh the embedded copy.
DOCS_VERSION     ?= bd8a3f4fe92454ea0709895d6d9c771b8e86e710
DOCS_DIR         := cmd/prometheus-mcp/external/docs
DOCS_TARBALL_URL := https://github.com/prometheus/docs/archive/$(DOCS_VERSION).tar.gz

# Dev tooling knobs.
BINARY            := prometheus-mcp-server
OLLAMA_MODEL      ?= ollama:gpt-oss:20b
OPENWEBUI_VERSION ?= v0.6.15

.PHONY: help
help: ## print this help message (see Makefile.common for the standard prometheus targets)
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nProject targets:\n"} /^[a-z0-9A-Z_-]+:.*?##/ { printf "  \033[36m%-30s\033[0m%s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: docs
docs: ## download and extract the pinned prometheus/docs snapshot for embedding
	@if [ "$$(cat $(DOCS_DIR)/COMMIT_HASH 2>/dev/null)" = "$(DOCS_VERSION)" ]; then \
		echo ">> prometheus/docs@$(DOCS_VERSION) already present"; \
	else \
		echo ">> downloading prometheus/docs@$(DOCS_VERSION)"; \
		rm -rf $(DOCS_DIR); \
		mkdir -p $(DOCS_DIR); \
		curl -sfL $(DOCS_TARBALL_URL) | tar -xzf - -C $(DOCS_DIR) --strip-components=1; \
		echo "$(DOCS_VERSION)" > $(DOCS_DIR)/COMMIT_HASH; \
	fi

# The binary embeds the prometheus/docs snapshot via go:embed, so it must be
# downloaded before building or testing.
.PHONY: build
build: docs common-build

.PHONY: crossbuild
crossbuild: docs promu
	$(PROMU) crossbuild $(PROMU_OPTS)

.PHONY: test
test: docs common-test

.PHONY: helm-sync-dashboards
helm-sync-dashboards: ## copy grafana dashboards into helm chart for packaging
	mkdir -p charts/prometheus-mcp-server/dashboards
	cp grafana/*.json charts/prometheus-mcp-server/dashboards/

.PHONY: helm-lint
helm-lint: helm-sync-dashboards ## run helm chart linting
	ct lint --config ct.yaml

.PHONY: helm-template
helm-template: helm-sync-dashboards ## render helm templates for inspection
	helm template prometheus-mcp-server charts/prometheus-mcp-server/

.PHONY: helm-test
helm-test: helm-sync-dashboards ## install helm chart and run tests (requires a running cluster)
	ct install --config ct.yaml

.PHONY: mcphost
mcphost: build ## use mcphost to run the prometheus-mcp-server against a local ollama model
	mcphost --debug --config ./mcp.json --model "${OLLAMA_MODEL}"

.PHONY: inspector
inspector: build ## use inspector to run the prometheus-mcp-server in STDIO transport mode
	npx @modelcontextprotocol/inspector --config ./mcp.json --server "${BINARY}"

.PHONY: inspector-http
inspector-http: ## use inspector to run the prometheus-mcp-server in streamable HTTP transport mode
	npx @modelcontextprotocol/inspector --config ./mcp.json --server "${BINARY}-http"

.PHONY: open-webui
open-webui: build ## use open-webui to run the prometheus-mcp-server
	docker run --rm -d -p 11119:8080 --add-host=host.docker.internal:host-gateway -v open-webui:/app/backend/data --name open-webui "ghcr.io/open-webui/open-webui:${OPENWEBUI_VERSION}"
	uvx mcpo --port 18000 -- "./${BINARY}"

.PHONY: gemini
gemini: build ## use gemini-cli to run the prometheus-mcp-server against Google Gemini models
	npx @google/gemini-cli
