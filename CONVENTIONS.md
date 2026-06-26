# Project Conventions

This document establishes standards for the dwarfbot repository.

## Container Engine & Images

The primary container engine is **podman**, controlled by the `CONTAINER_SUBSYS` variable
(`CONTAINER_SUBSYS ?= podman` in the Makefile). It can be overridden via the environment or
an `.env` file.

Base image choices:

- **Production runtime**: `registry.access.redhat.com/ubi9/ubi-minimal` — UBI for Red Hat
  supportability and a minimal attack surface.
- **Build stage**: `registry.access.redhat.com/ubi9/go-toolset` — matches the runtime OS family.
- **CI container**: `registry.fedoraproject.org/fedora-minimal` — lightweight Fedora base with
  access to the full Fedora package set for tooling.

Image tags must be **pinned to a specific version** — never use `:latest` in a `FROM` directive.

## CI Approach

All checks run inside a dedicated CI container (`test/Containerfile.ci`) to eliminate
environment inconsistencies across developer machines and GitHub Actions.

| Context | Command | Behavior |
| --- | --- | --- |
| Local (full) | `make ci-all` | Builds CI container, then runs all checks inside it |
| Local (Go only) | `make ci` | Runs `fmt`, `vet`, `go-test`, and `build` directly on the host |
| Inside container | `make ci-checks` | Runs every check; used by both `ci-all` and GHA jobs |

GitHub Actions builds the CI container image once (`ci-build` job), uploads it as an artifact,
and then individual check jobs download and load it in parallel.

All CI checks must pass locally before committing.

## Makefile Targets

### Required targets

| Target | Purpose |
| --- | --- |
| `ci-build` | Builds the CI container image |
| `ci-all` | Builds the CI container and runs all checks inside it (local entry point) |
| `ci-checks` | Runs all checks inside the container (used by GHA) |
| `fmt` / `fmt-check` | Formats Go code / checks formatting without modifying files |
| `vet` | Runs `go vet` |
| `lint` | Runs `golangci-lint` |
| `go-test` | Runs tests with `-race` and `-count=1` |
| `build` | Compiles the binary to `out/dwarfbot` |
| `checkmake` | Lints the Makefile with `checkmake` |
| `mdlint` | Lints Markdown with `markdownlint-cli2` |
| `yamllint` | Lints YAML with `yamllint` |
| `kubeconform` | Validates Kubernetes manifests in `deploy/` |
| `containerfile-check` | Validates `Containerfile` and `test/Containerfile.ci` |
| `shellcheck` | Lints shell scripts under `test/` |
| `doc-check` | Verifies that `docs/plans/` exists and contains plan documents |
| `image-build` | Builds the production container image with OCI labels |
| `image-push` | Builds and pushes the image to the registry |
| `clean` | Removes the `out/` directory |

Tool binaries (`golangci-lint`, `checkmake`) are looked up via `command -v` at make-time; missing
tools produce a clear error message directing the developer to use `make ci-all` for the pinned
toolchain.

## Standard Checks

The following checks are enforced in CI:

- **Go formatting** — `gofmt` via `make fmt-check`; format locally with `make fmt`
- **Go vet** — `make vet`
- **Go linting** — `golangci-lint` via `make lint`
- **Go tests** — `go test -race -count=1 ./...` via `make go-test`
- **Go build** — binary compiles and `--help` exits cleanly via `make build`
- **Makefile lint** — `checkmake Makefile` via `make checkmake`
- **Markdown lint** — `markdownlint-cli2` via `make mdlint` (line-length rule disabled)
- **YAML lint** — `yamllint` extending `default` profile via `make yamllint` (line-length and
  document-start rules disabled; `truthy.check-keys` disabled)
- **Kubernetes manifests** — `kubeconform -strict` on `deploy/` via `make kubeconform`
- **Containerfile validation** — custom script `test/validate-containerfile.sh` via
  `make containerfile-check`
- **Shell scripts** — `shellcheck` on all `*.sh` files under `test/` via `make shellcheck`
- **Plan documents** — `docs/plans/` must exist and contain at least one `.md` file via
  `make doc-check`

## OCI Image Labels

All production images must include the following labels, populated at build time via `ARG` values:

### OpenContainers standard labels

| Label | Value |
| --- | --- |
| `org.opencontainers.image.title` | `dwarfbot` |
| `org.opencontainers.image.description` | Human-readable description |
| `org.opencontainers.image.url` | Repository URL |
| `org.opencontainers.image.source` | Repository URL |
| `org.opencontainers.image.revision` | Full git SHA (`VCS_REF`) |
| `org.opencontainers.image.version` | Build version (`VERSION`) |
| `org.opencontainers.image.created` | RFC 3339 build timestamp (`BUILD_DATE`) |
| `org.opencontainers.image.vendor` | `clcollins` |
| `org.opencontainers.image.licenses` | `MIT` |

### Kubernetes display labels

| Label | Value |
| --- | --- |
| `io.k8s.display-name` | `dwarfbot` |
| `io.k8s.description` | Human-readable description |

### Custom cluster labels

| Label | Value |
| --- | --- |
| `is.collins.cluster.image.revision` | Full git SHA |
| `is.collins.cluster.image.version` | Build version |
| `is.collins.cluster.image.created` | RFC 3339 build timestamp |
| `is.collins.cluster.build.commit.id` | Full git SHA |
| `is.collins.cluster.build.date` | RFC 3339 build timestamp |

Label presence and correctness are validated in the `image-build` CI job.

## Container Image Tagging

Images are published to `quay.io/clcollins/dwarfbot`. Tags follow these rules:

| Trigger | Tags applied |
| --- | --- |
| Any build | `<short-sha>` (first 7 characters of the commit SHA) |
| Pull request | `pr-<number>` — built locally, **not pushed** to the registry |
| Push to `main` | `latest` |
| Git tag `v*` | `<full-tag>` (e.g. `v0.2.0`) and `<tag-without-v>` (e.g. `0.2.0`) |

Multi-architecture manifests (`linux/amd64` + `linux/arm64`) are assembled with
`podman manifest` before pushing.

## Documentation

Plan documents live in `docs/plans/` and use descriptive file names (not numbers). Names may
optionally include an ISO date prefix for chronological ordering (e.g.
`2026-06-23_mqtt-discord-mouthpiece.md`).

Plan documents **must never be overwritten or deleted**. They may only be updated to add:

- Lessons learned
- Post-mortem review (PMR) notes
- Lint fixes

When a new plan supersedes an existing one, the old plan is preserved and a note is added
indicating it has been superseded.

## Version Control

- Development happens on **feature branches**; commits go to `main` via pull request.
- Commit messages should be descriptive and explain the *why*, not just the *what*.
- Commits must be **signed**.
