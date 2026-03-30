# Plan: Platform Resilience, Prometheus Metrics, and Alerting

## Context

DwarfBot crashed the entire process when a single platform failed
in dual-platform mode. If Discord's token was bad, Twitch died too
(and vice versa). There were no metrics, health checks, or alerting.
This change adds graceful degradation, comprehensive Prometheus
metrics, and SLO-based alerting rules.

## Lessons from Prior Plans

- **plan-unit-tests.md**: Injectable `exitFunc` for testability;
  `net.Pipe()` for IRC testing; mockPlatform for commands
- **plan-discord-support.md**: ChatPlatform interface abstraction;
  platforms loosely coupled via interface
- **plan-container-non-root.md**: Config file optional, env vars
  work; read-only filesystem; UID 1001
- **plan-go-update.md**: Keep deps current; go vet must pass

## Changes Made

### Platform Resilience

Removed `log.Fatalf` from `Connect()` and `Start()` — they now
return errors instead. `cmd/root.go` handles platform failures
gracefully:

- If Discord fails to start: log warning, continue with Twitch
- If Twitch exhausts retries: log error, continue with Discord
- If both fail or neither configured: exit
- If the last running platform fails: exit

### PlatformMetrics Interface

New `PlatformMetrics` interface in `pkg/dwarfbot/platform.go`
decouples metric recording from metric implementation. Both
`DwarfBot` and `DiscordBot` accept an optional `Metrics` field
(nil means no metrics). ChatPlatform interface unchanged.

### Prometheus Metrics (`pkg/metrics/`)

Uses custom `prometheus.Registry` (not global) for test isolation.

**Connection**: `dwarfbot_platform_connected`,
`dwarfbot_platform_connection_attempts_total`,
`dwarfbot_platform_disconnections_total`,
`dwarfbot_platform_connection_duration_seconds`

**Config**: `dwarfbot_platform_token_present`,
`dwarfbot_platform_configured`

**Messages**: `dwarfbot_messages_received_total`,
`dwarfbot_messages_sent_total`,
`dwarfbot_commands_processed_total`

**App**: `dwarfbot_uptime_seconds` (GaugeFunc),
`dwarfbot_info` (version/go_version), Go runtime + process
collectors

### Metrics HTTP Server

Serves `/metrics` (Prometheus) and `/healthz` (liveness) on
configurable port (default 8080, `--metrics-port` flag,
`DWARFBOT_METRICS_PORT` env var).

### Alerting Rules (`deploy/prometheus-rules.yaml`)

PrometheusRule CR for OpenShift with:

- `DwarfBotPlatformDown` — disconnected > 5m (warning)
- `DwarfBotAllPlatformsDown` — all down > 2m (critical)
- `DwarfBotHighMessageFailureRate` — > 10% failure (warning)

**SLO** (99.9% uptime per platform):

- Recording rules for 5m, 1h, 6h availability windows
- `DwarfBotPlatformBurnRateFast` — 14.4x burn over 1h (critical)
- `DwarfBotPlatformBurnRateSlow` — 6x burn over 6h (warning)

## Files

| File | Change |
| --- | --- |
| `CLAUDE.md` | Add plan document protection rule |
| `pkg/metrics/metrics.go` | Metric definitions |
| `pkg/metrics/recorder.go` | PlatformMetrics implementation |
| `pkg/metrics/server.go` | HTTP server for /metrics, /healthz |
| `pkg/metrics/*_test.go` | Tests for all metrics code |
| `pkg/dwarfbot/platform.go` | PlatformMetrics interface |
| `pkg/dwarfbot/mock_metrics_test.go` | Mock recorder for tests |
| `pkg/dwarfbot/dwarfbot.go` | Connect/Start return error, hooks |
| `pkg/dwarfbot/discord.go` | Metric hooks |
| `pkg/dwarfbot/commands.go` | Command metrics via variadic param |
| `cmd/root.go` | Resilience rewrite, metrics server |
| `deploy/prometheus-rules.yaml` | Alerting rules |
| `Containerfile` | EXPOSE 8080 |

## Verification

- `make ci` passes all tests
- `go test -race ./...` — no data races
- `curl localhost:8080/metrics` returns `dwarfbot_*` metrics
- `curl localhost:8080/healthz` returns "ok"
- Start with only Discord token: Twitch warning, Discord works
- Start with only Twitch token: Discord warning, Twitch works
- Start with neither: fatal exit
