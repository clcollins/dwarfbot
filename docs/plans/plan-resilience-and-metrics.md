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

## Post-Mortem (PR #7 Review)

_Lessons captured from PR #7 Copilot code review and PR #8
follow-up fix. Cluster deployment verification is separate._

### What Went Well

- PlatformMetrics interface with nil-guard pattern cleanly
  separated metric recording from platform code
- Custom prometheus.Registry (not global) enabled isolated
  testing without metric pollution between tests
- Resilience logic correctly degraded to single-platform
  mode when one platform failed

### What Went Wrong

- **PromQL failure ratio wrong** (Copilot #4/#28): The
  `DwarfBotHighMessageFailureRate` alert had two bugs:
  (1) the `result` label wasn't aggregated away so the
  division only matched failure/failure instead of
  failure/total, and (2) the denominator used `> 0` which
  produces a boolean (0/1) instead of the actual rate. Both
  would cause the alert to either never fire or produce
  `+Inf`. Caught by Copilot review.

- **BotName used as platform label** (Copilot #3):
  `RecordCommandProcessed` was passed `platform.BotName()`
  (e.g., "dwarfbot") instead of the platform identifier
  ("twitch"/"discord"), mislabeling the
  `dwarfbot_commands_processed_total` metric series. Caught
  by Copilot review.

- **Unbounded label cardinality** (Copilot #22): Raw user
  command strings were used as Prometheus label values.
  Since commands come from user input, this creates unbounded
  cardinality which can significantly increase Prometheus
  memory usage. Caught by Copilot review.

- **SLO alerts fire for unconfigured platforms** (Copilot
  #15/#16): Burn-rate alerts weren't gated on
  `dwarfbot_platform_configured == 1`, so they would fire
  continuously for intentionally-unused platforms in
  single-platform deployments. Caught by Copilot review.

- **Impossible alert condition** (Copilot #17):
  `DwarfBotTokenMissing` required
  `token_present == 0 AND configured == 1`, but
  `configured` is only set to 1 when a token is present,
  making the condition impossible. Additionally, the alert
  was listed in the plan doc but never defined in the rules
  file (Copilot #21/#25/#29). Caught by Copilot review.

- **Connection leak on HandleChat error** (Copilot #20):
  When `HandleChat()` returned an error, `Start()` looped
  back to `Connect()` without closing the existing
  connection, leaking the previous `net.Conn` and leaving
  `dwarfbot_platform_connected` stuck at 1. Caught by
  Copilot review.

- **Disconnect not idempotent** (Copilot #23):
  `Disconnect()` closed the conn but never set
  `db.conn = nil`, so double-calling it produced
  double-close errors and duplicate disconnect metrics.
  Caught by Copilot review.

- **Invalid Go syntax** (Copilot #1):
  `for attempt := range maxRetries` doesn't compile in Go
  (range can't iterate over an int). Should have been a
  conventional counted loop. Caught by Copilot review.

- **No Twitch graceful shutdown** (Copilot #30, fixed in
  PR #8): The Twitch bot goroutine had no coordinated
  shutdown path — SIGINT/SIGTERM killed the process without
  running deferred `Disconnect()` or recording shutdown
  metrics. This was significant enough to warrant a
  follow-up PR (#8) implementing `Stop()`.

### Lessons Learned

- PromQL vector matching is label-aware by default — always
  verify that numerator and denominator have compatible label
  sets by using `sum by(...)` or `without(...)` on both sides
- Never use raw user input as Prometheus label values —
  normalize to a fixed, low-cardinality set
- Alert rules for optional components must be gated on a
  "component enabled" signal to avoid false-positive alerts
  in partial deployments
- Validate alert conditions are satisfiable: if the
  prerequisite for label A being 1 also forces label B to be
  non-zero, the combined condition is tautological or
  impossible
- `Disconnect()` must be idempotent — set `conn = nil` after
  closing to prevent double-close errors
- Plan docs that list alert names must match the actual rules
  file — treat the rules YAML as the source of truth
