# Plan: Non-Root, Filesystem-Free Container Image

## Context

DwarfBot's container image should run as a proper non-root user
following Red Hat/OpenShift conventions, and should not require
any filesystem access at runtime. Previously the image used
`USER 1000` and `initConfig()` fatally exited if no config file
was found on disk, preventing environment-variable-only operation
inside a container.

## Lessons from Prior Plans

- **plan-go-update.md**: Migrated to UBI9 base images
  (`ubi9/go-toolset:1.25` builder, `ubi9/ubi-minimal` runtime)
- **plan-discord-support.md**: Config supports YAML file, CLI
  flags, and environment variables, but the code did not actually
  support running without a config file (fatal on missing)
- **plan-unit-tests.md**: Injectable `exitFunc` pattern for
  testability; `net.Pipe()` for IRC testing without network

## Changes Made

### 1. Config file made optional (`cmd/root.go`)

Rewrote `initConfig()` to gracefully handle missing config files:

- If `--config` flag explicitly set and file fails: **fatal**
- If no `--config` and file not found: **log and continue**
  using env vars / CLI flags
- If file exists but has parse errors: **fatal**

Uses `viper.ConfigFileNotFoundError` with `errors.As()` to
distinguish "not found" from "parse error".

### 2. Environment variable prefix (`cmd/root.go`)

Added `viper.SetEnvPrefix("DWARFBOT")` so environment variables
use the `DWARFBOT_` prefix:

_Updated: Config keys renamed for provider-prefix standardization.
See `docs/plan-standardize-config-keys.md` for details._

| Config Key | CLI Flag | Env Var |
| --- | --- | --- |
| `name` | `--name` | `DWARFBOT_NAME` |
| `twitch_token` | `--twitch-token` | `DWARFBOT_TWITCH_TOKEN` |
| `twitch_channels` | `--twitch-channels` | `DWARFBOT_TWITCH_CHANNELS` |
| `twitch_server` | `--twitch-server` | `DWARFBOT_TWITCH_SERVER` |
| `twitch_port` | `--twitch-port` | `DWARFBOT_TWITCH_PORT` |
| `verbose` | `--verbose` | `DWARFBOT_VERBOSE` |
| `metrics_port` | `--metrics-port` | `DWARFBOT_METRICS_PORT` |
| `discord_token` | `--discord-token` | `DWARFBOT_DISCORD_TOKEN` |
| `discord_channels` | `--discord-channels` | `DWARFBOT_DISCORD_CHANNELS` |
| `discord_admin_role` | `--discord-admin-role` | `DWARFBOT_DISCORD_ADMIN_ROLE` |

### 3. Removed `go-homedir` dependency

Replaced `github.com/mitchellh/go-homedir` with stdlib
`os.UserHomeDir()` (available since Go 1.12; project uses 1.25).
Home directory resolution error is handled gracefully (skips
config path if unknown).

### 4. Container UID updated (`Containerfile`)

Changed `USER 1000` to `USER 1001` following the OpenShift
operator convention (used by pagerduty-operator,
configure-alertmanager-operator, certman-operator, and others
in the `openshift/` org).

No `user_setup` script needed since dwarfbot has zero filesystem
writes at runtime.

### 5. Tests added (`cmd/root_test.go`)

- `TestInitConfig_NoConfigFile`: Verifies graceful fallback when
  no config file exists
- `TestInitConfig_EnvVarPrefix`: Verifies `DWARFBOT_NAME` and
  `DWARFBOT_TOKEN` env vars are read correctly
- `TestInitConfig_DiscordEnvVars`: Verifies Discord-specific
  env vars (`DWARFBOT_DISCORD_TOKEN`, etc.)

## Verification

- `make ci` passes all tests
- Container builds: `podman build -f Containerfile -t dwarfbot:test .`
- Read-only filesystem works:

  ```sh
  podman run --read-only --rm \
    -e DWARFBOT_DISCORD_TOKEN=fake \
    -e DWARFBOT_DISCORD_CHANNELS=123 \
    -e DWARFBOT_NAME=testbot \
    dwarfbot:test
  ```

- Help works: `podman run --read-only --rm dwarfbot:test --help`
- Local dev with config file unchanged:
  `./out/dwarfbot --config ~/.dwarfbot.yaml`

## Post-Mortem (PR #4 Review)

_Lessons captured from PR #4 Copilot code review._

### What Went Well

- `errors.As()` for `viper.ConfigFileNotFoundError` cleanly
  distinguished "missing file" from "parse error"
- Environment variable prefix prevented collisions with
  other tools' env vars

### What Went Wrong

- **Silent backward-incompatible env var change** (Copilot
  #1): Adding `viper.SetEnvPrefix("DWARFBOT")` silently
  broke any existing deployments using unprefixed env vars
  like `DISCORD_TOKEN`. The plan doc listed the new prefixed
  names but didn't call out the breaking change or provide
  migration guidance. Caught by Copilot review.

### Lessons Learned

- Changes to environment variable names are a public API
  change — document the migration path and consider
  supporting both old and new names during a transition
  period via `viper.BindEnv()`
