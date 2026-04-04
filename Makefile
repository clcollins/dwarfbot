-include .env

CONTAINER_SUBSYS ?= podman
NAME := dwarfbot
PROJECT := clcollins
IMAGE_REGISTRY := quay.io

CONTAINER_FILE := Containerfile
CI_CONTAINER_FILE := test/Containerfile.ci
CI_IMAGE := $(NAME)-ci
IMAGE_STRING := $(IMAGE_REGISTRY)/$(PROJECT)/$(NAME)

GIT_SHA := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
VERSION ?= dev

GO := go
GOFLAGS ?=

# Tool binaries (installed via go install)
GOLANGCI_LINT := $(shell command -v golangci-lint 2>/dev/null)
CHECKMAKE := $(shell command -v checkmake 2>/dev/null)

.PHONY: all
all: fmt vet lint go-test build

# --- Go targets ---

.PHONY: fmt
fmt:
	$(GO) fmt ./...

.PHONY: fmt-check
fmt-check:
	@output=$$(gofmt -l .); \
	if [ -n "$$output" ]; then \
		echo "Files not formatted:"; \
		echo "$$output"; \
		exit 1; \
	fi

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

.PHONY: go-test
go-test:
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
	$(CONTAINER_SUBSYS) build -f $(CONTAINER_FILE) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		--build-arg VCS_REF=$(GIT_COMMIT) \
		--build-arg VERSION=$(VERSION) \
		-t $(IMAGE_STRING):$(GIT_SHA) -t $(IMAGE_STRING):latest .

.PHONY: image-push
image-push: image-build
	$(CONTAINER_SUBSYS) push $(IMAGE_STRING):$(GIT_SHA)
	$(CONTAINER_SUBSYS) push $(IMAGE_STRING):latest

# --- Linting tools ---

.PHONY: checkmake
checkmake:
ifdef CHECKMAKE
	checkmake Makefile
else
	@echo "checkmake is required but not installed (install: go install github.com/checkmake/checkmake/cmd/checkmake@latest)"
	@exit 1
endif

.PHONY: mdlint
mdlint:
	@command -v markdownlint-cli2 >/dev/null 2>&1 \
		&& markdownlint-cli2 '**/*.md' '#node_modules' \
		|| { echo "markdownlint-cli2 not found (install: npm install -g markdownlint-cli2)"; exit 1; }

.PHONY: kubeconform
kubeconform:
	@command -v kubeconform >/dev/null 2>&1 \
		&& kubeconform -strict -ignore-missing-schemas deploy/ \
		|| { echo "kubeconform not found (install: go install github.com/yannh/kubeconform/cmd/kubeconform@latest)"; exit 1; }

.PHONY: yamllint
yamllint:
	@command -v yamllint >/dev/null 2>&1 \
		&& yamllint -c .yamllint.yaml . \
		|| { echo "yamllint not found (install: pip install yamllint)"; exit 1; }

.PHONY: doc-check
doc-check:
	@if [ ! -d docs/plans ]; then echo "ERROR: docs/plans/ directory not found"; exit 1; fi
	@count=$$(find docs/plans -name '*.md' | wc -l); \
	if [ "$$count" -eq 0 ]; then echo "ERROR: no plan documents found in docs/plans/"; exit 1; fi
	@echo "OK: docs/plans/ contains plan documents"

.PHONY: containerfile-check
containerfile-check:
	./test/validate-containerfile.sh $(CONTAINER_FILE)

.PHONY: shellcheck
shellcheck:
	@command -v shellcheck >/dev/null 2>&1 \
		&& find test/ -name '*.sh' -exec shellcheck {} + \
		|| { echo "shellcheck not found (install: dnf install shellcheck)"; exit 1; }

# --- Aggregate targets ---

.PHONY: test-all
test-all: fmt vet lint go-test build checkmake mdlint yamllint kubeconform containerfile-check shellcheck doc-check image-build
	@echo "All checks passed."

# --- CI targets ---

.PHONY: ci-build
ci-build:
	$(CONTAINER_SUBSYS) build -f $(CI_CONTAINER_FILE) -t $(CI_IMAGE) .

.PHONY: ci-all
ci-all: ci-build
	$(CONTAINER_SUBSYS) run --rm -v $$(pwd):/src:Z -w /src $(CI_IMAGE) make ci-checks

.PHONY: ci-checks
ci-checks: fmt-check vet lint go-test build checkmake mdlint yamllint kubeconform containerfile-check shellcheck doc-check
	@echo "All CI checks passed."

.PHONY: test
test: ci-all

.PHONY: ci
ci: fmt vet go-test build

.PHONY: clean
clean:
	rm -rf out/
