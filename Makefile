SHELL := /bin/bash
.DEFAULT_GOAL := help

APP_NAME        ?= storverity
APP_META_PKG    ?= github.com/soyunomas/storverity/internal/appmeta
FRONTEND_DIR    ?= frontend
BUILD_DIR       ?= build/bin
DIST_DIR        ?= dist
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
VERSION          ?= 0.1.0-dev
COMMIT           ?= $(shell git rev-parse HEAD 2>/dev/null || printf unknown)
SOURCE_DATE_EPOCH ?= $(shell git log -1 --format=%ct 2>/dev/null || printf 0)
BUILD_DATE       ?= $(shell date -u -d '@$(SOURCE_DATE_EPOCH)' '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || printf unknown)
APP_LDFLAGS      = -X $(APP_META_PKG).Version=$(VERSION) -X $(APP_META_PKG).Commit=$(COMMIT) -X $(APP_META_PKG).BuildDate=$(BUILD_DATE)

.PHONY: \
	help info doctor bootstrap deps lock-update lock-check \
	go-deps go-mod-check fmt fmt-check vet test test-rawprobe test-cover test-cover-html check-core run list \
	frontend-install frontend-update frontend-lock-check frontend-audit frontend-test frontend-check frontend-build frontend-dev frontend-all \
	wails-install wails-doctor linux-deps dev build desktop-dev desktop-build desktop-clean \
	package-appimage package-tarball package release-checksums release \
	check ci ci-go ci-frontend ci-desktop ci-package \
	clean clean-coverage clean-frontend clean-packaging clean-all

##@ General

help: ## Show this help message
	@printf '\nStorVerity development targets\n\n'
	@awk 'BEGIN {FS = ":.*## "; printf "Usage:\n  make \033[36m<target>\033[0m\n"} \
		/^##@/ {printf "\n\033[1m%s\033[0m\n", substr($$0, 5); next} \
		/^[a-zA-Z0-9_.-]+:.*## / {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@printf '\nCommon examples:\n'
	@printf '  make doctor          Check the local toolchain\n'
	@printf '  make bootstrap       Install pinned project dependencies\n'
	@printf '  make check           Run core + frontend validation\n'
	@printf '  make test-rawprobe   Run raw-probe tests without block-device access\n'
	@printf '  make dev             Start the Wails development app\n'
	@printf '  make package         Build AppImage + deterministic tarball\n'
	@printf '  make ci              Run the full CI-equivalent validation\n\n'

info: ## Print project build configuration
	@printf 'Application:       %s\n' '$(APP_NAME)'
	@printf 'Version:           %s\n' '$(VERSION)'
	@printf 'Commit:            %s\n' '$(COMMIT)'
	@printf 'Build date:        %s\n' '$(BUILD_DATE)'
	@printf 'Source epoch:      %s\n' '$(SOURCE_DATE_EPOCH)'
	@printf 'Go packages:       %s\n' '$(GO_PACKAGES)'
	@printf 'Frontend:          %s\n' '$(FRONTEND_DIR)'
	@printf 'Wails version:     %s\n' '$(WAILS_VERSION)'
	@printf 'Wails build tags:  %s\n' '$(WAILS_TAGS)'
	@printf 'Go baseline:       %s\n' '$(GO_VERSION)'
	@printf 'Node baseline:     %s\n' '$(NODE_VERSION)'

# doctor checks presence first so failures are actionable before printing versions.
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

bootstrap: deps wails-install ## Install pinned Go/frontend dependencies and the Wails CLI

deps: go-deps frontend-install ## Install project dependencies from committed lock data

lock-update: ## Regenerate Go and npm dependency lock data intentionally
	$(GO) mod tidy
	cd $(FRONTEND_DIR) && $(NPM) install --package-lock-only --ignore-scripts
	@echo 'Updated go.mod/go.sum and $(FRONTEND_DIR)/package-lock.json'

lock-check: go-mod-check frontend-lock-check ## Verify committed dependency metadata is current

##@ Go core / CLI

go-deps: ## Download Go modules using go.mod/go.sum
	$(GO) mod download

go-mod-check: ## Fail if go.mod or go.sum is not tidy/reproducible
	$(GO) mod tidy
	@git diff --exit-code -- go.mod go.sum || { \
		echo 'error: Go module metadata changed; run: make lock-update' >&2; \
		exit 1; \
	}

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

test-rawprobe: ## Run raw-probe/safety tests without accessing real block devices
	$(GO) test -race ./internal/rawprobe ./internal/safety ./internal/appservice

test-cover: ## Run Go tests with coverage and print the coverage summary
	$(GO) test -race -coverprofile=$(COVERAGE_FILE) $(GO_PACKAGES)
	$(GO) tool cover -func=$(COVERAGE_FILE)

test-cover-html: test-cover ## Generate an HTML Go coverage report
	$(GO) tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo 'Coverage report: $(COVERAGE_HTML)'

check-core: fmt-check go-mod-check vet test ## Run all Go core/CLI checks

run: list ## Alias for the diagnostic device-list CLI

list: ## List discovered storage devices as JSON
	$(GO) run ./cmd/storverity list

##@ Frontend

frontend-install: ## Install exact frontend dependencies from package-lock.json
	cd $(FRONTEND_DIR) && $(NPM) ci

frontend-update: ## Update/install frontend dependencies and refresh package-lock.json
	cd $(FRONTEND_DIR) && $(NPM) install

frontend-lock-check: ## Fail if package.json and package-lock.json are inconsistent
	cd $(FRONTEND_DIR) && $(NPM) install --package-lock-only --ignore-scripts
	@git diff --exit-code -- $(FRONTEND_DIR)/package-lock.json || { \
		echo 'error: npm lockfile changed; run: make lock-update' >&2; \
		exit 1; \
	}

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

frontend-all: frontend-audit frontend-test frontend-check frontend-build ## Run all frontend validation after dependencies are installed

##@ Wails desktop

wails-install: ## Install the pinned Wails CLI
	$(GO) install github.com/wailsapp/wails/v2/cmd/wails@$(WAILS_VERSION)

wails-doctor: ## Run Wails environment diagnostics
	$(WAILS) doctor

linux-deps: ## Install Ubuntu/Debian packages required for Wails/WebKitGTK
	sudo apt-get update
	sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev

dev: desktop-dev ## Alias for the Wails desktop development app

build: desktop-build ## Alias for a production desktop build

desktop-dev: ## Start the Wails desktop app in development mode
	$(WAILS) dev -tags '$(WAILS_TAGS)'

desktop-build: ## Build a clean production desktop binary with embedded release metadata
	SOURCE_DATE_EPOCH='$(SOURCE_DATE_EPOCH)' $(WAILS) build -clean -tags '$(WAILS_TAGS)' -ldflags '$(APP_LDFLAGS)'

desktop-clean: ## Remove Wails desktop build output
	rm -rf $(BUILD_DIR)

##@ Packaging / release

package-appimage: desktop-build ## Build the x86_64 AppImage from the production desktop binary
	VERSION='$(VERSION)' ARCH=x86_64 DIST_DIR='$(abspath $(DIST_DIR))' bash scripts/package-appimage.sh

package-tarball: desktop-build ## Build a deterministic portable Linux tarball
	VERSION='$(VERSION)' ARCH=x86_64 SOURCE_DATE_EPOCH='$(SOURCE_DATE_EPOCH)' DIST_DIR='$(abspath $(DIST_DIR))' bash scripts/package-tarball.sh

package: desktop-build ## Build all Linux release packages and checksums
	VERSION='$(VERSION)' ARCH=x86_64 DIST_DIR='$(abspath $(DIST_DIR))' bash scripts/package-appimage.sh
	VERSION='$(VERSION)' ARCH=x86_64 SOURCE_DATE_EPOCH='$(SOURCE_DATE_EPOCH)' DIST_DIR='$(abspath $(DIST_DIR))' bash scripts/package-tarball.sh
	$(MAKE) release-checksums DIST_DIR='$(DIST_DIR)'

release-checksums: ## Write deterministic SHA-256 checksums for release artifacts
	@set -euo pipefail; \
	mkdir -p '$(DIST_DIR)'; \
	rm -f '$(DIST_DIR)/SHA256SUMS'; \
	mapfile -t files < <(find '$(DIST_DIR)' -maxdepth 1 -type f -name 'StorVerity-*' -printf '%f\n' | LC_ALL=C sort); \
	((${#files[@]} > 0)) || { echo 'error: no release artifacts found' >&2; exit 1; }; \
	cd '$(DIST_DIR)'; \
	sha256sum "$${files[@]}" > SHA256SUMS

release: check package ## Validate and build release artifacts
	@echo 'Release artifacts:'
	@ls -lh '$(DIST_DIR)'

##@ Validation / CI

check: check-core frontend-install frontend-all ## Run all non-packaging validation

ci-go: fmt-check go-mod-check vet ## Run the Go CI checks including race + coverage
	$(GO) test -race -cover $(GO_PACKAGES)

ci-frontend: frontend-install frontend-all ## Run the complete frontend CI job from package-lock.json

ci-desktop: go-deps wails-install desktop-build ## Run the desktop smoke-build job (system packages must already exist)

ci-package: ## Package an existing CI desktop binary as AppImage/tarball and verify checksums
	VERSION='$(VERSION)' ARCH=x86_64 DIST_DIR='$(abspath $(DIST_DIR))' bash scripts/package-appimage.sh
	VERSION='$(VERSION)' ARCH=x86_64 SOURCE_DATE_EPOCH='$(SOURCE_DATE_EPOCH)' DIST_DIR='$(abspath $(DIST_DIR))' bash scripts/package-tarball.sh
	$(MAKE) release-checksums DIST_DIR='$(DIST_DIR)'
	cd '$(DIST_DIR)' && sha256sum --check SHA256SUMS

ci: ci-go ci-frontend ci-desktop ci-package ## Run the complete CI-equivalent pipeline locally

##@ Cleanup

clean-coverage: ## Remove generated coverage reports
	rm -f $(COVERAGE_FILE) $(COVERAGE_HTML)

clean-frontend: ## Remove generated frontend assets while preserving the tracked embed placeholder
	@if [[ -d '$(FRONTEND_DIR)/dist' ]]; then \
		find '$(FRONTEND_DIR)/dist' -mindepth 1 ! -name '.gitkeep' -delete; \
	fi

clean-packaging: ## Remove generated packaging/release artifacts
	rm -rf build/appimage build/tarball $(DIST_DIR)

clean: clean-coverage clean-frontend desktop-clean clean-packaging ## Remove generated build/test artifacts

clean-all: clean ## Also remove installed frontend dependencies
	rm -rf $(FRONTEND_DIR)/node_modules
