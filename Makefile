SHELL := /bin/bash
.DEFAULT_GOAL := help

APP_NAME        ?= storverity
FRONTEND_DIR    ?= frontend
BUILD_DIR       ?= build/bin
COVERAGE_FILE   ?= coverage.out
COVERAGE_HTML   ?= coverage.html
GO_PACKAGES     ?= ./cmd/... ./internal/...
GO_VERSION      ?= 1.25
NODE_VERSION    ?= 22.16.0
WAILS_VERSION   ?= v2.15.0
WAILS_TAGS      ?= webkit2_41
WAILS           ?= wails
NPM             ?= npm
GO              ?= go

.PHONY: \
	help info doctor bootstrap deps \
	go-deps fmt fmt-check vet test test-cover test-cover-html check-core run list \
	frontend-install frontend-audit frontend-test frontend-check frontend-build frontend-dev frontend-all \
	wails-install linux-deps desktop-dev desktop-build desktop-clean \
	check ci ci-go ci-frontend ci-desktop \
	clean clean-coverage clean-frontend clean-all

##@ General

help: ## Show this help message
	@printf '\nStorVerity development targets\n\n'
	@awk 'BEGIN {FS = ":.*## "; printf "Usage:\n  make \033[36m<target>\033[0m\n"} \
		/^##@/ {printf "\n\033[1m%s\033[0m\n", substr($$0, 5); next} \
		/^[a-zA-Z0-9_.-]+:.*## / {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@printf '\nCommon examples:\n'
	@printf '  make doctor          Check the local toolchain\n'
	@printf '  make bootstrap       Install project dependencies\n'
	@printf '  make check           Run core + frontend validation\n'
	@printf '  make desktop-dev     Start the Wails development app\n'
	@printf '  make ci              Run the full CI-equivalent validation\n\n'

info: ## Print project build configuration
	@printf 'Application:       %s\n' '$(APP_NAME)'
	@printf 'Go packages:       %s\n' '$(GO_PACKAGES)'
	@printf 'Frontend:          %s\n' '$(FRONTEND_DIR)'
	@printf 'Wails version:     %s\n' '$(WAILS_VERSION)'
	@printf 'Wails build tags:  %s\n' '$(WAILS_TAGS)'
	@printf 'Go baseline:       %s\n' '$(GO_VERSION)'
	@printf 'Node baseline:     %s\n' '$(NODE_VERSION)'

# doctor intentionally checks presence first and prints versions second so failures are actionable.
doctor: ## Check required development tools and print their versions
	@set -e; \
	for tool in git $(GO) node $(NPM); do \
		command -v "$$tool" >/dev/null 2>&1 || { echo "error: $$tool is required but was not found" >&2; exit 1; }; \
	done; \
	printf 'git:   '; git --version; \
	printf 'go:    '; $(GO) version; \
	printf 'node:  '; node --version; \
	printf 'npm:   '; $(NPM) --version; \
	if command -v $(WAILS) >/dev/null 2>&1; then \
		printf 'wails: '; $(WAILS) version; \
	else \
		echo 'wails: not installed (run: make wails-install)'; \
	fi

bootstrap: deps wails-install ## Install Go, frontend and Wails development dependencies

deps: go-deps frontend-install ## Install project dependencies without Wails CLI

##@ Go core / CLI

go-deps: ## Resolve Go module dependencies
	$(GO) mod download

fmt: ## Format all tracked Go source files
	@git ls-files '*.go' -z | xargs -0 -r gofmt -w

fmt-check: ## Fail if any tracked Go source file needs gofmt
	@files="$$(git ls-files '*.go')"; \
	bad="$$(printf '%s\n' "$$files" | xargs -r gofmt -l)"; \
	if [[ -n "$$bad" ]]; then \
		echo 'Files need gofmt:'; \
		echo "$$bad"; \
		exit 1; \
	fi

vet: ## Run go vet on the storage core and CLI
	$(GO) vet $(GO_PACKAGES)

test: ## Run race-enabled Go tests
	$(GO) test -race $(GO_PACKAGES)

test-cover: ## Run Go tests with coverage and print the coverage summary
	$(GO) test -race -coverprofile=$(COVERAGE_FILE) $(GO_PACKAGES)
	$(GO) tool cover -func=$(COVERAGE_FILE)

test-cover-html: test-cover ## Generate an HTML Go coverage report
	$(GO) tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo 'Coverage report: $(COVERAGE_HTML)'

check-core: fmt-check vet test ## Run all Go core/CLI checks

run: list ## Alias for the diagnostic device-list CLI

list: ## List discovered storage devices as JSON
	$(GO) run ./cmd/storverity list

##@ Frontend

frontend-install: ## Install Svelte/TypeScript dependencies
	cd $(FRONTEND_DIR) && $(NPM) install

frontend-audit: ## Audit frontend dependencies; fail on high/critical issues
	cd $(FRONTEND_DIR) && $(NPM) audit --audit-level=high

frontend-test: ## Run frontend unit tests
	cd $(FRONTEND_DIR) && $(NPM) test

frontend-check: ## Run Svelte/TypeScript static checks
	cd $(FRONTEND_DIR) && $(NPM) run check

frontend-build: ## Build production frontend assets
	cd $(FRONTEND_DIR) && $(NPM) run build

frontend-dev: ## Start the standalone Vite development server
	cd $(FRONTEND_DIR) && $(NPM) run dev

frontend-all: frontend-audit frontend-test frontend-check frontend-build ## Run all frontend validation except dependency installation

##@ Wails desktop

wails-install: ## Install the pinned Wails CLI
	$(GO) install github.com/wailsapp/wails/v2/cmd/wails@$(WAILS_VERSION)

linux-deps: ## Install Ubuntu/Debian packages required for Wails/WebKitGTK
	sudo apt-get update
	sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev

desktop-dev: ## Start the Wails desktop app in development mode
	$(WAILS) dev -tags '$(WAILS_TAGS)'

desktop-build: ## Build a clean production desktop binary
	$(WAILS) build -clean -tags '$(WAILS_TAGS)'

desktop-clean: ## Remove Wails desktop build output
	rm -rf $(BUILD_DIR)

##@ Validation / CI

check: check-core frontend-all ## Run all non-packaging validation

ci-go: fmt-check vet ## Run the Go CI checks including race + coverage
	$(GO) test -race -cover $(GO_PACKAGES)

ci-frontend: frontend-install frontend-all ## Run the complete frontend CI job

ci-desktop: go-deps wails-install desktop-build ## Run the desktop smoke-build job (system packages must already exist)

ci: ci-go ci-frontend ci-desktop ## Run the complete CI-equivalent pipeline locally

##@ Cleanup

clean-coverage: ## Remove generated coverage reports
	rm -f $(COVERAGE_FILE) $(COVERAGE_HTML)

clean-frontend: ## Remove generated frontend assets while preserving the tracked embed placeholder
	@if [[ -d '$(FRONTEND_DIR)/dist' ]]; then \
		find '$(FRONTEND_DIR)/dist' -mindepth 1 ! -name '.gitkeep' -delete; \
	fi

clean: clean-coverage clean-frontend desktop-clean ## Remove generated build/test artifacts

clean-all: clean ## Also remove installed frontend dependencies
	rm -rf $(FRONTEND_DIR)/node_modules
