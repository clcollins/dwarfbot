# Plan: Standardize Config Key Naming Convention

## Context

Twitch-related config keys (`token`, `channels`, `server`, `port`)
lacked a provider prefix, while Discord keys already followed the
`DWARFBOT_DISCORD_<ITEM>` pattern. This inconsistency made it
unclear which platform a setting belonged to. This change renames
all Twitch-specific keys to follow the standard
`DWARFBOT_<PROVIDER>_<ITEM>` format and adds a CI test to prevent
naming convention regressions.

## Lessons from Prior Plans

- **plan-container-non-root.md**: Adding `viper.SetEnvPrefix("DWARFBOT")`
  silently broke unprefixed env vars. This time the breaking change
  is intentional and documented (clean break, no backward compat).
- **plan-discord-support.md**: Always check interface definitions
  when changing signatures. Not applicable here (no interface changes).

## Rename Mapping

| Old Config Key | New Config Key | Old CLI Flag | New CLI Flag | Old Env Var | New Env Var |
| --- | --- | --- | --- | --- | --- |
| `token` | `twitch_token` | _(none)_ | `--twitch-token` | `DWARFBOT_TOKEN` | `DWARFBOT_TWITCH_TOKEN` |
| `channels` | `twitch_channels` | `--channels` / `-c` | `--twitch-channels` | `DWARFBOT_CHANNELS` | `DWARFBOT_TWITCH_CHANNELS` |
| `server` | `twitch_server` | `--server` / `-s` | `--twitch-server` | `DWARFBOT_SERVER` | `DWARFBOT_TWITCH_SERVER` |
| `port` | `twitch_port` | `--port` / `-p` | `--twitch-port` | `DWARFBOT_PORT` | `DWARFBOT_TWITCH_PORT` |

### Unchanged (general/cross-cutting settings)

- `name` / `--name` / `-n` / `DWARFBOT_NAME` (used by both platforms)
- `verbose` / `--verbose` / `-v` / `DWARFBOT_VERBOSE`
- `metrics_port` / `--metrics-port` / `DWARFBOT_METRICS_PORT`

### Already standard (no changes)

- `discord_token` / `--discord-token` / `DWARFBOT_DISCORD_TOKEN`
- `discord_channels` / `--discord-channels` / `DWARFBOT_DISCORD_CHANNELS`
- `discord_admin_role` / `--discord-admin-role` / `DWARFBOT_DISCORD_ADMIN_ROLE`

## Design Decisions

1. **`name` stays general**: The bot display name is shared across
   both Twitch and Discord platforms.
2. **Clean break, no backward compat**: Old env var names stop
   working immediately. No `viper.BindEnv()` fallback.
3. **Short flags removed** for renamed flags (`-s`, `-p`, `-c`):
   ambiguous when combined with provider prefix. `-v` (verbose) and
   `-n` (name) are unchanged general flags and keep their shorthand.
4. **Added `--twitch-token` CLI flag**: Previously token had no CLI
   flag (env/config only), but `--discord-token` exists, so added
   for consistency.

## Changes Made

### 1. Flag registration and viper bindings (`cmd/root.go`)

Renamed Twitch flag definitions in `init()`:

- `--server` / `-s` -> `--twitch-server` (no short flag)
- `--port` / `-p` -> `--twitch-port` (no short flag)
- `--channels` / `-c` -> `--twitch-channels` (no short flag)
- Added `--twitch-token` (new flag)

Updated `viper.BindPFlag()` keys to match.

### 2. Viper Get calls (`cmd/root.go`)

Updated `Run` function to use new config key names:

- `viper.GetString("token")` -> `viper.GetString("twitch_token")`
- `viper.GetStringSlice("channels")` -> `viper.GetStringSlice("twitch_channels")`
- `viper.GetString("server")` -> `viper.GetString("twitch_server")`
- `viper.GetString("port")` -> `viper.GetString("twitch_port")`

### 3. Tests updated (`cmd/root_test.go`)

- Flag name table updated with new names and removed short flags
- Individual default tests renamed (e.g., `TestServerFlagDefault`
  -> `TestTwitchServerFlagDefault`)
- Added `TestTwitchTokenFlagDefault`
- Env var prefix test updated to use `DWARFBOT_TWITCH_TOKEN`
- Flag usage text test updated with new flag names

### 4. CI naming convention test (`cmd/root_test.go`)

Added `TestConfigKeyNamingConvention` that validates all non-general
flags follow the `<provider>-<item>` naming convention. General
flags (`config`, `verbose`, `name`, `metrics-port`) are exempted.
All other flags must start with a known provider prefix (`twitch-`
or `discord-`). Runs as part of `make ci` via `go test`.

### 5. Prior plan doc updated (`docs/plan-container-non-root.md`)

Updated config table to reflect new key names with a note
referencing this plan document.

## Breaking Changes

1. **Env vars**: `DWARFBOT_TOKEN`, `DWARFBOT_CHANNELS`,
   `DWARFBOT_SERVER`, `DWARFBOT_PORT` no longer work. Use
   `DWARFBOT_TWITCH_TOKEN`, `DWARFBOT_TWITCH_CHANNELS`,
   `DWARFBOT_TWITCH_SERVER`, `DWARFBOT_TWITCH_PORT`.
2. **CLI flags**: `--server`/`-s`, `--port`/`-p`,
   `--channels`/`-c` removed. Use `--twitch-server`,
   `--twitch-port`, `--twitch-channels`.
3. **Config file keys**: `token`, `channels`, `server`, `port`
   in `.dwarfbot.yaml` must become `twitch_token`,
   `twitch_channels`, `twitch_server`, `twitch_port`.

## Migration Guide

Update environment variables:

```sh
# Before
export DWARFBOT_TOKEN=oauth:abc123
export DWARFBOT_CHANNELS=mychannel
export DWARFBOT_SERVER=irc.chat.twitch.tv
export DWARFBOT_PORT=6667

# After
export DWARFBOT_TWITCH_TOKEN=oauth:abc123
export DWARFBOT_TWITCH_CHANNELS=mychannel
export DWARFBOT_TWITCH_SERVER=irc.chat.twitch.tv
export DWARFBOT_TWITCH_PORT=6667
```

Update config file (`~/.dwarfbot.yaml`):

```yaml
# Before
token: oauth:abc123
channels:
  - mychannel

# After
twitch_token: oauth:abc123
twitch_channels:
  - mychannel
```

## Verification

- `make ci` passes (fmt, vet, test, build)
- `TestConfigKeyNamingConvention` catches non-compliant flags
- Existing tests updated and passing with new key names

## Post-Mortem

_To be filled after PR review._

### What Went Well

### What Went Wrong

### Lessons Learned
