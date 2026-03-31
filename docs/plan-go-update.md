# Plan: Update Go Version and Dependencies

## Context

Dwarfbot was running Go 1.22 in `go.mod` with a Containerfile
referencing the much older `golang:1.15` builder image. All
dependencies were outdated. This update modernizes the project
to use Go 1.25 and refreshes every dependency to its latest
compatible version.

## Changes Made

### Go Version

- **`go.mod`**: Updated from `go 1.22` to `go 1.25`
- **`Containerfile`**: Updated builder image from `golang:1.15`
  to `ubi9/go-toolset:1.25`
- **`Containerfile`**: Updated base image from
  `ubi8/ubi-minimal` to `ubi9/ubi-minimal`

### Dependencies Updated

| Package | Old Version | New Version |
| ------- | ----------- | ----------- |
| `github.com/spf13/cobra` | v1.8.1 | v1.10.2 |
| `github.com/spf13/viper` | v1.19.0 | v1.21.0 |
| `github.com/fsnotify/fsnotify` | v1.7.0 | v1.9.0 |
| `github.com/pelletier/go-toml/v2` | v2.2.2 | v2.2.4 |
| `github.com/sagikazarmark/locafero` | v0.6.0 | v0.12.0 |
| `github.com/spf13/afero` | v1.11.0 | v1.15.0 |
| `github.com/spf13/cast` | v1.6.0 | v1.10.0 |
| `github.com/spf13/pflag` | v1.0.5 | v1.0.10 |
| `golang.org/x/crypto` | (none) | v0.49.0 |
| `golang.org/x/sys` | v0.21.0 | v0.42.0 |
| `golang.org/x/text` | v0.16.0 | v0.35.0 |

New indirect dependencies added by viper v1.21.0:

- `github.com/go-viper/mapstructure/v2` v2.5.0
- `go.yaml.in/yaml/v3` v3.0.4

### Bug Fix

- **`cmd/root.go`**: Fixed `go vet` warning — non-constant
  format string in `log.Fatalf()` call changed to use `"%s"`
  format specifier.

## Verification

- `go build ./...` — compiles cleanly
- `go vet ./...` — passes with no warnings
- `go mod tidy` — no extraneous or missing dependencies

## Notes

- Go 1.25 is the target version for this update, matching
  the available UBI9 go-toolset builder image.
- UBI 9 (ubi-minimal) replaces UBI 8 as the base container
  image, as UBI 8 is approaching end of maintenance support.

## Post-Mortem (PR #3 Review)

_Lessons captured from PR #3 Copilot code review._

### What Went Wrong

- **Plan doc version mismatch** (Copilot #19/#32): The plan
  document originally stated Go 1.24 and listed dependency
  versions that didn't match the actual `go.mod` contents
  (e.g., viper listed as v1.21.0 but `go.mod` had v1.20.1,
  locafero listed as v0.6.0→v0.12.0 but actual was v0.7.0).
  Caught by Copilot review.

### Lessons Learned

- After running `go get -u` and `go mod tidy`, re-read
  `go.mod` and update the plan's dependency table to match
  actual resolved versions — don't assume `go get -u` will
  pull the exact version you expect
- Plan documents should be updated as a final step after all
  code changes are committed, not written speculatively
