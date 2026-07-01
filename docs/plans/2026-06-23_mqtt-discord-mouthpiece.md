# MQTT-to-Discord Mouthpiece Bridge

**Plan ID:** 2026-06-23-dwarfbot-mqtt-mouthpiece
**Status:** Implementing
**Created:** 2026-06-23
**Related plans:** All docs under `docs/` are historical predecessors. `docs/plan-discord-support.md` is legacy/historical only and NOT authoritative for the current config interface.

## Summary

This plan adds an MQTT subscriber bridge to dwarfbot that forwards messages from a Mosquitto broker to Discord as batched digests. The feature is off by default, uses operator-configurable topic filters, and enforces rate caps and buffer limits to prevent Discord rate-limiting.

Key design decisions:

- MQTT is an input source, NOT a chat platform -- it does not satisfy the "at least one platform" requirement
- Discord fan-out via injected callback (`func(channelID, msg string) error`) -- no circular import between `pkg/mqtt` and `pkg/dwarfbot`
- Drop-oldest buffer policy for live debug view (keep most recent messages)
- Bridge self-heals on failure with sequential backoff and Discord notification
- Bridge lifecycle managed internally -- no `mqttErrCh` in root.go `select`; root.go only calls `Stop()` on shutdown
- New MQTT metrics registered by `pkg/mqtt` itself, accepting `*prometheus.Registry`
- `SetConfigMetrics` refactored from hardcoded two-platform signature to `[]SourceConfig`

## Implementation

### New package: `pkg/mqtt`

Files:

- `bridge.go` -- Bridge struct, NewBridge, Start/Stop, Enable/Disable/Status, flush loop, reconnect loop, MQTT message handler
- `buffer.go` -- Bounded ring buffer with drop-oldest, Message type, TruncatePayload
- `config.go` -- Config struct, ValidateConfig
- `format.go` -- FormatDigest (chunking + rate cap + suppression notice)
- `metrics.go` -- BridgeMetrics struct registered on prometheus.Registry
- `bridge_test.go` -- 38 unit tests covering config validation, buffer, truncation, formatting, metrics, bridge lifecycle

### Modified files

- `cmd/root.go` -- MQTT config flags, validation, lifecycle wiring (start after Discord, Stop on shutdown), admin command registration, SetConfigMetrics call updated
- `cmd/root_test.go` -- Added `mqtt-` to provider prefix list for flag naming convention test
- `pkg/dwarfbot/commands.go` -- Added `RegisterMQTTHandler`, `getMQTTHandler`, `mqtt` case in `parseAdminCommand`, added `mqtt` to `knownCommands`
- `pkg/metrics/metrics.go` -- Refactored `SetConfigMetrics` to accept `[]SourceConfig` instead of hardcoded two-platform params
- `pkg/metrics/metrics_test.go` -- Updated all `SetConfigMetrics` tests for new signature, added `TestSetConfigMetrics_InitializesConnectedGauge`
- `README.md` -- Added MQTT Bridge Settings section with config table and admin command docs
- `go.mod` / `go.sum` -- Added `github.com/eclipse/paho.mqtt.golang v1.5.1`

### Config keys

| Config Key | Default | Description |
| --- | --- | --- |
| `mqtt_enabled` | `false` | Master switch |
| `mqtt_broker` | (empty) | Broker URL |
| `mqtt_username` | (empty) | MQTT user |
| `mqtt_password` | (empty) | From Secret |
| `mqtt_client_id` | `dwarfbot` | Client ID |
| `mqtt_topics` | (empty) | Topic filters |
| `mqtt_discord_channels` | falls back to `discord_channels` | Discord targets |
| `mqtt_flush_seconds` | `30` | Flush interval [5, 86400] |
| `mqtt_max_buffer` | `500` | Buffer cap |
| `mqtt_max_payload_bytes` | `256` | Payload truncation |
| `mqtt_max_posts_per_flush` | `5` | Outbound rate cap |

### Admin commands

- `!dwarfbot mqtt on` -- enable forwarding (admin only)
- `!dwarfbot mqtt off` -- disable forwarding (admin only)
- `!dwarfbot mqtt status` -- report enabled/connected/buffer depth/topics

### Prometheus metrics (registered by pkg/mqtt on existing registry)

- `dwarfbot_mqtt_messages_received_total`
- `dwarfbot_mqtt_messages_forwarded_total`
- `dwarfbot_mqtt_messages_dropped_total`
- `dwarfbot_mqtt_messages_suppressed_total`
- `dwarfbot_mqtt_buffer_depth`
- `dwarfbot_mqtt_bridge_enabled`
- `dwarfbot_mqtt_connected`

## Addendum

### Rev 1 to Rev 2 (Addenda 1-14)

See the full Rev 2 plan for the complete list of changes from the Master Control review of Rev 1. All 13 concerns were addressed and none were declined.

### Addendum 15 -- SetConfigMetrics refactor touch points (MC review of Rev 2, Note 1)

The `SetConfigMetrics` refactor to a non-hardcoded source-config shape touches, at minimum:

- `pkg/metrics/metrics.go` -- the `SetConfigMetrics` method definition (refactored from `(twitchToken, discordToken string, twitchChannels, discordChannels []string)` to `(sources []SourceConfig)`)
- `cmd/root.go` -- the call site (updated to pass `[]metrics.SourceConfig`)
- `pkg/metrics/metrics_test.go` -- all three existing `SetConfigMetrics` tests updated for new signature, plus one new test `TestSetConfigMetrics_InitializesConnectedGauge`

All three were updated in the same PR.

### Addendum 16 -- MQTT metrics registered by pkg/mqtt (MC review of Rev 2, Note 2)

MQTT-specific metrics (buffer depth, dropped, suppressed, bridge enabled, mqtt connected) are NOT additions to the `PlatformMetrics` interface or `Recorder`. They are separate Prometheus collectors registered directly on the existing `*prometheus.Registry` by `pkg/mqtt`. The `NewBridgeMetrics` function accepts a `prometheus.Registerer` and registers all MQTT-specific collectors. This keeps the bridge self-contained and avoids polluting the platform metrics interface with bridge-internal concerns.

### Addendum 17 -- Bridge self-heals on failure (MC review of Rev 2, Note 3)

When the MQTT bridge loses its broker connection:

1. Log the error
2. Post to Discord (via the poster callback) that the bridge has gone down
3. Attempt to reconnect with sequential backoff (5s x attempt, up to 10 attempts)
4. On reconnect, resubscribe to all configured topics
5. If all reconnect attempts fail, post to Discord that the bridge is offline

The bridge never triggers a dwarfbot shutdown. There is no `mqttErrCh` in the `root.go` `select` -- the bridge manages its own lifecycle internally. `root.go` only calls `bridge.Stop()` during the shutdown signal handler.

## Lessons Learned

Updated 2026-07-01 after production deployment revealed a reconnection storm and Discord notification flood (PR #17, PR #18).

### Bug 1: Dual competing reconnection mechanisms

Paho's `SetAutoReconnect(true)` and the bridge's own `reconnectLoop()` both fired on connection loss. Each created connections with the same ClientID, causing Mosquitto to kill the older connection — which triggered another `onConnectionLost`, creating an infinite cascade. **Lesson:** When implementing your own reconnection logic with backoff, always disable the library's built-in auto-reconnect. Two reconnect paths with the same ClientID will fight.

### Bug 2: New client created on every reconnect

`connect()` called the client factory every time, creating a fresh paho client without disconnecting the old one. The dereferenced client kept its connection alive, so two clients with the same ClientID existed simultaneously. **Lesson:** Reuse the existing client object for reconnection — paho's `Connect()` works on a disconnected client. Only create the client once.

### Bug 3: No guard against duplicate reconnectLoop goroutines

`onConnectionLost` spawned `go reconnectLoop()` unconditionally. Multiple concurrent loops amplified the ClientID collision problem. **Lesson:** Any callback that spawns a goroutine needs a guard to prevent duplicates, especially callbacks that can fire rapidly (like connection-lost handlers).

### Bug 4: Discord notification spam

`notifyDiscord()` was called from `onConnectionLost` with no rate limiting. In the reconnect storm this fired dozens of times per second. **Lesson:** Any notification path reachable from a retry loop or error callback needs throttling. A 5-minute cooldown with an injectable clock (`nowFunc`) for testability is the right pattern.

### Bug 5: b.client field unprotected by mutex

`b.client` was read and written from multiple goroutines (`connect()`, `Stop()`, `subscribe()`) without mutex protection. **Lesson:** Any field accessed from callbacks, goroutines, and the main path needs synchronization. Use minimal lock scope — snapshot the reference under the lock, then use the local copy for blocking I/O.

### Bug 6: subscribe() errors silently ignored

`subscribe()` logged errors but returned nothing. The bridge could report "connected" while having no active subscriptions. **Lesson:** Internal methods that can fail should return errors so callers can react. Silent logging is not error handling.

### Bug 7: Viper GetStringSlice does not split env var commas

`viper.GetStringSlice("mqtt_topics")` with env var `DWARFBOT_MQTT_TOPICS=home/#,ai/#,system/#` treated the entire value as one string. Mosquitto rejected the invalid topic filter containing commas, causing immediate EOF disconnection. This is a well-known Viper limitation. **Lesson:** When using Viper with env vars for string slices, add a wrapper that detects single-element slices containing commas and splits them. This affected all four `StringSlice` config keys (twitch channels, discord channels, mqtt topics, mqtt discord channels).
