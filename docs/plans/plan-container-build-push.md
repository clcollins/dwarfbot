# Plan: Container Image Build & Push via GitHub Actions

## Status: Draft

## Objective

Add automated multi-architecture container image builds and pushes to
quay.io/clcollins/dwarfbot via GitHub Actions, with proper OCI labels
and CI validation.

## Requirements

1. **Build triggers and tagging:**
   - Every PR: build only (no push), tagged with short git SHA and
     `pr-<number>` for validation
   - Every tagged release (`v*`): build and push, tagged with short git
     SHA and version (e.g., `dwarfbot:v0.2.0`)
   - Every merge to `main`: build and push, tagged with short git SHA
     and `latest`

2. **Multi-architecture:** Build linux/amd64 and linux/arm64 images in a
   single OCI manifest

3. **OCI/Red Hat labels:** Add standard metadata labels following OCI
   image-spec and Red Hat conventions

4. **CI validation:** Ensure Containerfile changes continue to produce
   images with expected labels and a working binary

5. **Registry:** quay.io/clcollins/dwarfbot with robot account
   credentials via GitHub secrets

## Architecture Decision: Separate Workflow + Podman

- **Separate workflow file** (`image-build-push.yaml`) rather than
  extending `ci.yaml`. The existing CI handles code quality; the new
  workflow handles container publishing. Different triggers are needed
  (the new workflow needs `push tags: ['v*']`).
- **Podman with QEMU** for multi-arch builds. Podman's `--platform`
  flag and `podman manifest` handle per-architecture builds and manifest
  creation natively without Docker dependencies.
- **Existing `image-build` job in `ci.yaml`** is preserved and enhanced
  with build-arg passing and label validation. It continues to serve as
  a podman-based smoke test.

## Files to Create/Modify

### 1. New: `.github/workflows/image-build-push.yaml`

Multi-arch build-and-push workflow using Podman:

- `qemu-user-static` package for ARM64 emulation
- `podman login` for quay.io authentication (conditional, not on PRs)
- Shell-based tag and label generation from git context
- `podman build --platform` for per-architecture builds
- `podman manifest` for multi-arch manifest creation and push

Triggers:

```yaml
on:
  pull_request:
    branches: [main, master]
  push:
    branches: [main]
    tags: ['v*']
```

Tag strategy via shell script in metadata step:

| Event        | Tags generated                    |
|--------------|-----------------------------------|
| PR #22       | `pr-22`, `abc1234` (build only)   |
| Push to main | `latest`, `abc1234`               |
| Tag v0.2.0   | `v0.2.0`, `0.2.0`, `abc1234`      |

Push is conditional: skipped when `github.event_name == 'pull_request'`

### 2. Modify: `Containerfile`

Add `ARG` and `LABEL` instructions to the runtime stage:

```dockerfile
ARG BUILD_DATE="1970-01-01T00:00:00Z"
ARG VCS_REF="unknown"
ARG VERSION="dev"

LABEL org.opencontainers.image.title="dwarfbot" \
      org.opencontainers.image.description="DwarfBot - a multi-platform chat bot" \
      org.opencontainers.image.url="https://github.com/clcollins/dwarfbot" \
      org.opencontainers.image.source="https://github.com/clcollins/dwarfbot" \
      org.opencontainers.image.revision="${VCS_REF}" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.vendor="clcollins" \
      org.opencontainers.image.licenses="MIT" \
      io.k8s.display-name="dwarfbot" \
      io.k8s.description="DwarfBot - a multi-platform chat bot" \
      is.collins.cluster.image.revision="${VCS_REF}" \
      is.collins.cluster.image.version="${VERSION}" \
      is.collins.cluster.image.created="${BUILD_DATE}" \
      is.collins.cluster.build.commit.id="${VCS_REF}" \
      is.collins.cluster.build.date="${BUILD_DATE}"
```

Labels follow:

- **OCI image-spec** (`org.opencontainers.image.*`): title, description,
  url, source, revision, version, created, vendor, licenses
- **Kubernetes** (`io.k8s.*`): display-name, description
- **Collins cluster** (`is.collins.cluster.image.*`,
  `is.collins.cluster.build.*`): image metadata and build provenance

ARGs have sensible defaults so local `podman build` still works without
passing build args.

The `--build-arg` values passed by the CI workflow override these
defaults at build time with actual CI values.

### 3. New: `.dockerignore`

Reduce build context size and prevent unnecessary files from being
included:

```text
.git
.github
out/
docs/
img/
deploy/
*.md
LICENSE
.markdownlint.yaml
.dwarfbot.yaml
```

Using `.dockerignore` (not `.containerignore`) because it is recognized
by both Docker buildx and Podman.

### 4. Modify: `Makefile`

Add git metadata variables and update `image-build` to pass build args:

- New variables: `GIT_SHA`, `GIT_COMMIT`, `BUILD_DATE`, `VERSION`
- Updated `image-build` target: passes `--build-arg` for all three ARGs,
  tags with both `$(GIT_SHA)` and `latest`
- New `image-push` target: pushes tagged images to registry

### 5. Modify: `.github/workflows/ci.yaml`

Enhance the existing `image-build` job to:

- Pass `--build-arg` values to the podman build command
- Add a "Validate OCI labels" step that inspects the built image and
  verifies expected labels are present
- Add a "Validate image runs" step that runs the container and verifies
  `--help` output

This provides CI validation that Containerfile changes produce correct
metadata, without requiring registry credentials.

## Secret Configuration

Two repository secrets must be added in GitHub repository settings
(Settings > Secrets and variables > Actions):

| Secret          | Description                    |
|-----------------|--------------------------------|
| `QUAY_USERNAME` | Quay.io robot account username |
| `QUAY_PASSWORD` | Quay.io robot account token    |

Recommendation: Create a Quay.io robot account with push access scoped
to `clcollins/dwarfbot` only.

## Implementation Steps

1. Create `.dockerignore`
2. Modify `Containerfile` to add ARG/LABEL instructions
3. Update `Makefile` with git metadata variables and build-arg passing
4. Update `.github/workflows/ci.yaml` to validate labels in image-build
   job
5. Create `.github/workflows/image-build-push.yaml`
6. Run `make ci` to validate all changes
7. Run `make image-build` to validate local container build
8. Manual: configure `QUAY_USERNAME` and `QUAY_PASSWORD` secrets in
   GitHub repo settings

## Risks and Mitigations

| Risk                                                                       | Mitigation                                                                         |
|----------------------------------------------------------------------------|------------------------------------------------------------------------------------|
| QEMU arm64 emulation is slow (10-20 min)                                   | No caching currently; future: registry-based caching or Go cross-compilation       |
| UBI go-toolset arm64 image might not exist for 1.25 tag                    | Verify availability; fallback: use `golang:1.25` for builder stage                 |
| checkmake rejects Makefile changes                                         | Follow Makefile conventions checkmake expects                                      |
| `--build-arg` values conflict with Containerfile LABEL defaults            | CI build-arg values override Containerfile ARG defaults (desired behavior)         |

## Lessons Learned (PR #9 Review)

_Lessons captured from PR #9 Copilot code review and CI
qualification._

### What Went Well

- Separate workflow file kept CI concerns cleanly separated
  from container publishing
- Conditional push (`github.event_name != 'pull_request'`)
  prevented accidental pushes from PR builds
- OCI label validation in CI caught label issues early

### What Went Wrong

- **Podman not installed on runner** (Copilot round 1 #1):
  The workflow used `podman` commands but `ubuntu-latest`
  does not guarantee Podman is available. Would have failed
  at the first `podman` invocation. Caught by Copilot
  review; fixed by adding explicit `apt-get install podman`.

- **Manifest push used wrong object type** (Copilot round 1
  #3): The manifest push loop used `podman tag` on manifest
  lists and then pushed each tag separately. `podman tag`
  operates on images, not manifest lists. Fixed by pushing
  the source manifest directly to each tag destination.

- **Manifest creation skipped for PRs** (Copilot round 2
  #1): PR builds skipped manifest creation entirely, meaning
  multi-arch assembly was never validated on PRs. Fixed by
  creating the manifest locally for all builds and only
  skipping the registry push.

- **PR tags never applied locally** (Copilot round 3 #1):
  Tags like `pr-9` and the short SHA were computed in
  metadata but never applied to local images during PR
  builds, making the stated tagging behavior inconsistent
  with reality. Fixed by applying all computed tags locally.

- **Version label mismatch** (Copilot round 3 #2): When
  publishing a version tag like `v0.2.0`, the OCI
  `version` label used `v0.2.0` but the published tag was
  `0.2.0` (without prefix). Fixed by using the no-`v` form
  for the version build-arg.

### Lessons Learned

- GitHub-hosted runners may not have all container tools
  pre-installed — always explicitly install required tools
- When building multi-arch manifests with Podman, use
  `containers-storage:` transport to reference locally-built
  images instead of assuming registry access
- Multi-arch manifest creation should run on PR builds too
  (just skip the push) to validate the assembly process
- Tag metadata computed in one step should be applied in a
  later step — don't just log what "would be" created
- Version labels and published tags must use the same format
  to avoid consumer confusion
