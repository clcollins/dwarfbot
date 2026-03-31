# Plan: Add Discord Channel Support

## Context

Dwarfbot previously only supported Twitch IRC. This update
adds Discord as a second chat platform, running alongside
Twitch. Either or both platforms can be enabled independently
via configuration, allowing Discord-only, Twitch-only, or
dual-platform operation.

## Architecture

### ChatPlatform Interface

A new `ChatPlatform` interface (`pkg/dwarfbot/platform.go`)
abstracts the differences between chat platforms:

```go
type ChatPlatform interface {
    SendMessage(channel, msg string) error
    IsAdmin(channel, user string) bool
    BotName() string
    BotChannels() []string
    Shutdown(exitCode int)
}
```

Both `DwarfBot` (Twitch) and `DiscordBot` implement this
interface. Command handlers (`parseCommand`, `ping`,
`channels`, etc.) were refactored to accept `ChatPlatform`
instead of `*DwarfBot`, making them platform-agnostic.

### Discord Bot

The `DiscordBot` struct (`pkg/dwarfbot/discord.go`) uses the
[discordgo][discordgo-url] library (v0.29.0) to:

[discordgo-url]: https://github.com/bwmarrin/discordgo

- Connect to Discord via WebSocket gateway
- Listen for messages in configured channel IDs
- Parse commands using the same `!dwarfbot <cmd> <args>`
  format as Twitch
- Route commands through the shared `parseCommand()` pipeline
- Send responses back to the originating channel

### Admin Permissions

- **Twitch**: Admin is the channel owner
  (userName == channelName) — unchanged
- **Discord**: Admin requires a configurable Discord role
  (default: `dwarfbot-admin`)
  - The bot checks if the message author has a role matching
    the configured name
  - Role matching is case-insensitive

### Platform Selection

In `cmd/root.go`, the bot determines which platforms to start
based on config:

- **Twitch**: Enabled when `token` and `channels` are both set
- **Discord**: Enabled when `discord_token` and `discord_channels`
  are both set
- **Error**: If neither platform is configured, the bot exits
  with an error
- **Discord-only mode**: When only Discord is configured, the
  bot runs until interrupted (SIGINT/SIGTERM)

## New Files

| File | Purpose |
| ---- | ------- |
| `pkg/dwarfbot/platform.go` | ChatPlatform interface |
| `pkg/dwarfbot/discord.go` | DiscordBot implementation |
| `pkg/dwarfbot/platform_test.go` | Mock and interface tests |

## Modified Files

| File | Changes |
| ---- | ------- |
| `pkg/dwarfbot/commands.go` | Use `ChatPlatform` interface |
| `pkg/dwarfbot/dwarfbot.go` | Add `ChatPlatform` methods |
| `cmd/root.go` | Discord flags, dual-platform startup |
| `cmd/root_test.go` | Tests for new Discord flags |
| `go.mod` / `go.sum` | Added `discordgo` v0.29.0 |

## Configuration

### Config File (`~/.dwarfbot.yaml`)

```yaml
# Bot identity
name: dwarfbot

# Twitch configuration (optional)
token: "your_twitch_oauth_token"
channels:
  - hammerdwarf

# Discord configuration (optional)
discord_token: "your_discord_bot_token"
discord_channels:
  - "123456789012345678"  # Discord channel ID
discord_admin_role: "dwarfbot-admin"
```

### CLI Flags

| Flag | Description | Default |
| ---- | ----------- | ------- |
| `--discord-token` | Discord bot token | (none) |
| `--discord-channels` | Channel IDs to listen in | (none) |
| `--discord-admin-role` | Role for admin commands | `dwarfbot-admin` |

### Environment Variables

Viper supports environment variable overrides:

- `DISCORD_TOKEN`
- `DISCORD_CHANNELS`
- `DISCORD_ADMIN_ROLE`

## Discord Bot Setup

To use the Discord integration:

1. Create a Discord application at the
   [Discord Developer Portal][discord-dev]
2. Create a bot user and copy the token
3. Enable the **Message Content Intent** under Privileged
   Gateway Intents
4. Invite the bot to your server with the `bot` scope and
   `Send Messages` + `Read Message History` permissions
5. Get the channel ID(s) where the bot should listen
   (right-click channel > Copy Channel ID, with Developer
   Mode enabled)
6. Create a role named `dwarfbot-admin` (or your chosen name)
   and assign it to users who should have admin privileges
7. Add the config to `~/.dwarfbot.yaml`

[discord-dev]: https://discord.com/developers/applications

## Verification

- `go build ./...` compiles cleanly
- `go test ./...` all tests pass
- `go vet ./...` passes
- Manual: Run with Discord-only config, send
  `!dwarfbot ping` in channel
- Manual: Run with both platforms, verify simultaneous
  operation
- Manual: Verify admin commands only work for users with
  the configured role

## Post-Mortem (PR #3 Review)

_Lessons captured from PR #3 Copilot code review. Cluster
deployment verification is separate._

### What Went Well

- ChatPlatform interface cleanly abstracted both platforms
  with no platform-specific leakage into command handlers
- Mock platform enabled thorough command testing without
  real network connections
- Dual-platform startup logic correctly handled all
  combinations (both, either, neither)

### What Went Wrong

- **Shutdown no-op in Discord-only mode** (Copilot #1):
  `DiscordBot.Shutdown()` did nothing when `exitFunc` was nil,
  meaning the `!dwarfbot shutdown` admin command wouldn't
  actually stop the bot. Caught by Copilot review; fixed by
  ensuring `exitFunc` always defaults to `os.Exit`.

- **Data race on adminRoleCache** (Copilot #13): The
  `adminRoleCache` map was accessed without synchronization.
  Discord handlers run concurrently, so simultaneous admin
  checks could panic. Caught by Copilot review.

- **Interface violation after Die() signature change**
  (Copilot #17): `DwarfBot.Die` was changed to accept an exit
  code parameter but the `Bot` interface wasn't updated,
  breaking the interface contract. Caught by Copilot review.

- **Nil conn panic in Disconnect()** (Copilot #37):
  `Disconnect()` unconditionally called `db.conn.Close()`
  without nil-checking, causing a panic if called before
  `Connect()`. Caught by Copilot review.

- **Build failure on clean checkout** (Copilot #21/#22):
  `go build -o out/dwarfbot` would fail because the `out/`
  directory didn't exist. Applied to both Makefile and
  Containerfile. Caught by Copilot review; fixed by adding
  `mkdir -p out`.

- **Flag type mismatch** (Copilot #20): `channels` defined
  as `StringP` but read via `viper.GetStringSlice()`. Type
  mismatch could cause unexpected behavior depending on how
  the value was provided. Caught by Copilot review.

### Lessons Learned

- Always nil-check `net.Conn` before calling `Close()` —
  lifecycle ordering isn't guaranteed across all code paths
- When changing function signatures on concrete types, grep
  for interface definitions that reference them
- Maps shared across goroutines need synchronization even if
  concurrent access seems unlikely — Discord's event handlers
  run in goroutines by default
- Build targets that write to directories must create those
  directories first (`mkdir -p`)
