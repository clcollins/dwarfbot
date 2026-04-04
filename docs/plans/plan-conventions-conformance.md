# Plan: Project Conventions Conformance

## Context

Align dwarfbot with the project conventions document to ensure consistent
CI, linting, documentation structure, and container practices across
repositories.

## Changes

### File/Directory Renames

- `.dockerignore` renamed to `.containerignore` (podman native format)
- `docs/plan-*.md` moved to `docs/plans/` (conventions-required path)

### New Linting

- **yamllint**: `.yamllint.yaml` config added, Makefile target and CI job
- **kubeconform**: validates `deploy/prometheus-rules.yaml` with
  `-ignore-missing-schemas` for CRD types
- **shellcheck**: lints all `test/*.sh` scripts
- **Containerfile validation**: `test/validate-containerfile.sh` checks for
  pinned tags and trusted registries
- **doc-check**: verifies `docs/plans/` contains plan documents

### Containerized CI

All CI checks now run inside a dedicated CI container image
(`test/Containerfile.ci`) based on `fedora-minimal:42` with all linting
tools pre-installed.

- **Locally**: `make ci-all` builds the CI container and runs `make ci-checks`
  inside it
- **GHA**: The CI container is built once, saved as an artifact, then each
  check runs as a parallel job inside that container
- **Host-only jobs**: `image-build` runs directly on the GHA runner (needs
  host podman)

### Makefile Restructuring

- `test` renamed to `go-test` (runs Go unit tests directly)
- New `test` target calls `ci-all` (full containerized CI suite)
- `ci` target retained for quick Go-only checks
- `.env` file support via `-include .env`
- New targets: `ci-build`, `ci-all`, `ci-checks`, `yamllint`, `kubeconform`,
  `containerfile-check`, `shellcheck`, `doc-check`

## Convention Additions Suggested

Practices from this repo that should be added to the conventions document:

- Go CI checks (fmt, vet, golangci-lint, test -race)
- Code coverage upload to tracking service
- Binary and container image smoke tests (--help)
- Multi-architecture container builds (amd64 + arm64)
- Build metadata injection (BUILD_DATE, VCS_REF, VERSION)
- Non-root container user (numeric UID)
- Separate image build/push workflow with dynamic tagging
- Table-driven tests and race detector in CI
- Test caching disabled in CI (-count=1)

## Verification

- `make ci-all` builds CI container and runs all checks
- `make ci` runs quick Go-only checks
- `make image-build` builds the application container
- GHA workflow builds CI container once, fans out parallel check jobs

## Lessons Learned

To be updated after implementation review.
