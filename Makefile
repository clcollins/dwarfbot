CONTAINER_SUBSYS ?= podman
NAME := dwarfbot
PROJECT := clcollins
IMAGE_REGISTRY := quay.io

CONTAINER_FILE := Containerfile
IMAGE_STRING := $(IMAGE_REGISTRY)/$(PROJECT)/$(NAME)

GO := go
GOFLAGS ?=

# Tool binaries (installed via go install)
GOLANGCI_LINT := $(shell command -v golangci-lint 2>/dev/null)
CHECKMAKE := $(shell command -v checkmake 2>/dev/null)

.PHONY: all
all: fmt vet lint test build

# --- Go targets ---

.PHONY: fmt
fmt:
	$(GO) fmt ./...

.PHONY: vet
vet:
	$(GO) vet ./...

.PHONY: lint
lint:
ifdef GOLANGCI_LINT
	golangci-lint run ./...
else
	@echo "golangci-lint is required but not installed (install: https://golangci-lint.run/welcome/install/)"
	@exit 1
endif

.PHONY: test
test:
	$(GO) test -v -count=1 -race ./...

.PHONY: test-cover
test-cover:
	$(GO) test -cover -count=1 ./...

.PHONY: build
build:
	mkdir -p out
	$(GO) build $(GOFLAGS) -o out/$(NAME) .

# --- Container targets ---

.PHONY: image-build
image-build:
	$(CONTAINER_SUBSYS) build -f $(CONTAINER_FILE) -t $(IMAGE_STRING):latest .

# --- Linting tools ---

.PHONY: checkmake
checkmake:
ifdef CHECKMAKE
	checkmake Makefile
else
	@echo "checkmake not installed, skipping (install: go install github.com/checkmake/checkmake/cmd/checkmake@latest)"
	@exit 1
endif

.PHONY: mdlint
mdlint:
	@command -v markdownlint-cli2 >/dev/null 2>&1 \
		&& markdownlint-cli2 '**/*.md' '#node_modules' \
		|| { echo "markdownlint-cli2 not found (install: npm install -g markdownlint-cli2)"; exit 1; }

# --- Aggregate targets ---

.PHONY: test-all
test-all: fmt vet lint test build checkmake mdlint image-build
	@echo "All checks passed."

.PHONY: ci
ci: fmt vet test build

.PHONY: clean
clean:
	rm -rf out/
