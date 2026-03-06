.PHONY: all dev build clean install lint test test-watch \
       backend backend-jockd backend-jockq backend-jockmcp \
       vex proto frontend electron release release-mac release-linux \
       help

# Default target
all: backend vex frontend

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

# ---------------------------------------------------------------------------
# Dependencies
# ---------------------------------------------------------------------------

install: ## Install npm dependencies
	npm ci

# ---------------------------------------------------------------------------
# Backend (Go)
# ---------------------------------------------------------------------------

BACKEND_DIR := backend
BACKEND_BIN := $(BACKEND_DIR)/bin

backend: backend-jockd backend-jockq backend-jockmcp ## Build all Go binaries

backend-jockd: ## Build jockd
	@echo "Building jockd..."
	@mkdir -p $(BACKEND_BIN)
	cd $(BACKEND_DIR) && go build -o bin/jockd ./cmd/jockd/

backend-jockq: ## Build jockq
	@echo "Building jockq..."
	@mkdir -p $(BACKEND_BIN)
	cd $(BACKEND_DIR) && go build -o bin/jockq ./cmd/jockq/

backend-jockmcp: ## Build jockmcp
	@echo "Building jockmcp..."
	@mkdir -p $(BACKEND_BIN)
	cd $(BACKEND_DIR) && go build -o bin/jockmcp ./cmd/jockmcp/

backend-test: ## Run Go tests
	cd $(BACKEND_DIR) && go test ./... -v

vex: ## Build vex binary (set VEX_SRC to override path)
	VEX_SRC=$${VEX_SRC:-$(HOME)/golang_code/vex} && \
	echo "Building vex from $$VEX_SRC..." && \
	mkdir -p $(BACKEND_BIN) && \
	cd "$$VEX_SRC" && go build -o "$(CURDIR)/$(BACKEND_BIN)/vex" ./cmd/vex/

proto: ## Regenerate protobuf Go code
	protoc \
		--go_out=$(BACKEND_DIR)/internal/proto \
		--go_opt=paths=source_relative \
		--go-grpc_out=$(BACKEND_DIR)/internal/proto \
		--go-grpc_opt=paths=source_relative \
		-I proto \
		proto/jock.proto

# ---------------------------------------------------------------------------
# Frontend
# ---------------------------------------------------------------------------

frontend: ## Build frontend with Vite
	npx vite build

lint: ## TypeScript type check
	npx tsc --noEmit

test: ## Run frontend tests
	npx vitest run

test-watch: ## Run frontend tests in watch mode
	npx vitest

# ---------------------------------------------------------------------------
# Dev
# ---------------------------------------------------------------------------

dev: backend vex ## Build backend + start dev server
	ELECTRON_RUN_AS_NODE= npx vite --port=3000

# ---------------------------------------------------------------------------
# Electron / Release
# ---------------------------------------------------------------------------

electron: backend vex ## Build everything and package with electron-builder
	ELECTRON_RUN_AS_NODE= npx vite build
	ELECTRON_RUN_AS_NODE= npx electron-builder

release: release-mac ## Alias for release-mac

release-mac: backend vex ## Package for macOS (dmg + zip)
	ELECTRON_RUN_AS_NODE= npx vite build
	ELECTRON_RUN_AS_NODE= npx electron-builder --mac

release-linux: backend vex ## Package for Linux (AppImage + deb)
	ELECTRON_RUN_AS_NODE= npx vite build
	ELECTRON_RUN_AS_NODE= npx electron-builder --linux

# ---------------------------------------------------------------------------
# Utilities
# ---------------------------------------------------------------------------

clean: ## Remove build artifacts
	rm -rf dist dist-electron release $(BACKEND_BIN)

version: ## Print current version from package.json
	@node -p "require('./package.json').version"

bump-patch: ## Bump patch version and create git tag
	@NEW=$$(node -p "let v=require('./package.json').version.split('.');v[2]++;v.join('.')") && \
	npm version $$NEW --no-git-tag-version && \
	git add package.json package-lock.json && \
	git commit -m "v$$NEW" && \
	git tag "v$$NEW" && \
	echo "Tagged v$$NEW — run 'git push --tags' to trigger release"

bump-minor: ## Bump minor version and create git tag
	@NEW=$$(node -p "let v=require('./package.json').version.split('.');v[1]++;v[2]=0;v.join('.')") && \
	npm version $$NEW --no-git-tag-version && \
	git add package.json package-lock.json && \
	git commit -m "v$$NEW" && \
	git tag "v$$NEW" && \
	echo "Tagged v$$NEW — run 'git push --tags' to trigger release"
