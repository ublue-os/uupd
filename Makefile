# uupd build system
#
# Common targets:
#   make build     build the uupd binary into output/
#   make install   install the binary, systemd units, polkit rule and config
#   make test      run the unit tests
#   make vet       run go vet
#   make check     run vet and tests
#   make help      list all targets

SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

# Toolchain
GO ?= go
GO_BUILD_FLAGS ?= -trimpath
GO_TEST_FLAGS ?= -v -cover
GO_VET_FLAGS ?=
CGO_ENABLED ?= 0
LINTER ?= golangci-lint

# Version (defaults to dev; override with VERSION=1.2.3 or from git describe)
VERSION ?= dev

# Project
BINARY := uupd
OUTPUT_DIR := output
BIN := $(OUTPUT_DIR)/$(BINARY)
GO_PACKAGES := ./...

# Installation paths. PREFIX defaults to /usr because the shipped systemd
# units invoke /usr/bin/uupd.
PREFIX ?= /usr
BINDIR ?= $(PREFIX)/bin
UNITDIR ?= $(PREFIX)/lib/systemd/system
POLKITRULESDIR ?= $(PREFIX)/share/polkit-1/rules.d
SYSCONFDIR ?= /etc
DESTDIR ?=

.PHONY: all build install install-bin install-units install-config uninstall \
	test test-cover e2e e2e-list e2e-image vet check lint fmt fmt-check tidy \
	vendor clean help

all: build

build: | $(OUTPUT_DIR) ## Build the uupd binary into output/
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GO_BUILD_FLAGS) -ldflags "-X github.com/ublue-os/uupd/cmd.version=$(VERSION)" -o $(BIN) .

$(OUTPUT_DIR):
	mkdir -p $(OUTPUT_DIR)

install: install-bin install-units install-config ## Install binaries, units, polkit rule and config
ifneq ($(DESTDIR),)
else
	@echo
	@echo "Installed to $(PREFIX). To enable automatic updates, run:"
	@echo "  sudo systemctl daemon-reload"
	@echo "  sudo systemctl enable --now $(BINARY).timer"
endif

install-bin: build ## Install only the uupd binary
	install -Dpm 0755 $(BIN) $(DESTDIR)$(BINDIR)/$(BINARY)

install-units: ## Install the systemd units and polkit rule
	install -Dpm 0644 $(BINARY).service $(DESTDIR)$(UNITDIR)/$(BINARY).service
	install -Dpm 0644 $(BINARY)-manual.service $(DESTDIR)$(UNITDIR)/$(BINARY)-manual.service
	install -Dpm 0644 $(BINARY).timer $(DESTDIR)$(UNITDIR)/$(BINARY).timer
	install -Dpm 0644 $(BINARY).rules $(DESTDIR)$(POLKITRULESDIR)/$(BINARY).rules

install-config: ## Install the default config, preserving an existing one
	@if [ -e "$(DESTDIR)$(SYSCONFDIR)/$(BINARY)/config.json" ]; then \
		echo "Keeping existing $(DESTDIR)$(SYSCONFDIR)/$(BINARY)/config.json"; \
	else \
		install -Dpm 0644 config.json "$(DESTDIR)$(SYSCONFDIR)/$(BINARY)/config.json"; \
	fi

uninstall: ## Remove installed files (preserves config in /etc)
	rm -f $(DESTDIR)$(BINDIR)/$(BINARY)
	rm -f $(DESTDIR)$(UNITDIR)/$(BINARY).service
	rm -f $(DESTDIR)$(UNITDIR)/$(BINARY)-manual.service
	rm -f $(DESTDIR)$(UNITDIR)/$(BINARY).timer
	rm -f $(DESTDIR)$(POLKITRULESDIR)/$(BINARY).rules
	@if [ -e "$(DESTDIR)$(SYSCONFDIR)/$(BINARY)/config.json" ]; then \
		echo "Preserved $(DESTDIR)$(SYSCONFDIR)/$(BINARY)/config.json"; \
	fi
ifneq ($(DESTDIR),)
else
	@echo "Run 'systemctl daemon-reload' to apply the removed units."
endif

test: ## Run the unit tests
	$(GO) test $(GO_TEST_FLAGS) $(GO_PACKAGES)

test-cover: | $(OUTPUT_DIR) ## Run tests and write output/coverage.out
	$(GO) test -coverprofile=$(OUTPUT_DIR)/coverage.out $(GO_PACKAGES)
	$(GO) tool cover -func=$(OUTPUT_DIR)/coverage.out

IMAGE ?=
e2e: ## Run e2e tests for every supported image (sudo)
	sudo ./tests/e2e/e2e.sh --all

e2e-list: ## List e2e images
	./tests/e2e/e2e.sh --list

e2e-image: ## Run e2e for one image: make e2e-image IMAGE=bluefin
	@if [ -z "$(IMAGE)" ]; then \
		echo "Usage: make e2e-image IMAGE=<name>"; \
		./tests/e2e/e2e.sh --list; \
		exit 1; \
	fi
	sudo ./tests/e2e/e2e.sh "$(IMAGE)"

vet: ## Run go vet
	$(GO) vet $(GO_VET_FLAGS) $(GO_PACKAGES)

check: vet test ## Run vet and the unit tests

lint: ## Run golangci-lint
	@command -v $(LINTER) >/dev/null 2>&1 || { \
		echo "$(LINTER) is not installed; see https://golangci-lint.run/"; \
		exit 1; \
	}
	$(LINTER) run

fmt: ## Format Go source
	$(GO) fmt $(GO_PACKAGES)

fmt-check: ## Fail if any Go source is not gofmt'd
	@unformatted="$$(gofmt -l . | grep -v '^vendor/' || true)"; \
	if [ -n "$$unformatted" ]; then \
		echo "The following files need gofmt:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

tidy: ## Update go.mod/go.sum
	$(GO) mod tidy

vendor: ## Refresh the vendor directory
	$(GO) mod vendor

clean: ## Remove build artifacts
	rm -rf $(OUTPUT_DIR) dist

help: ## Show this help
	@awk 'BEGIN { \
		FS = ":.*##"; \
		printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"; \
	} /^[a-zA-Z0-9_-]+:.*## / { \
		printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2; \
	}' $(MAKEFILE_LIST)
