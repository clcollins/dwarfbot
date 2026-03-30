# Plan: Graceful Twitch Shutdown

## Context

When the process received SIGINT/SIGTERM, the Twitch bot goroutine
didn't get a clean shutdown signal. `DwarfBot.Start()` runs an
infinite reconnect loop, and `HandleChat()` blocks indefinitely on
`tp.ReadLine()`. The deferred `Disconnect()` in `Start()` never
ran because the goroutine was killed when the process exited.

Discord didn't have this problem because its cleanup runs via
`defer discordBot.Stop()` in the main goroutine.

## Lessons from Prior Plans

- **plan-unit-tests.md**: Tests use `conn.Close()` from outside
  to unblock `HandleChat()` — the proven shutdown mechanism
- **plan-resilience-and-metrics.md**: `PlatformMetrics` nil-guard
  pattern; `mockMetricsRecorder` for test assertions

## Changes Made

### `Stop()` method on DwarfBot

Added a mutex-protected `stopped` flag and a `Stop()` method that
sets it and closes the connection. Closing the connection
immediately unblocks any pending `ReadLine()` in `HandleChat()`.

### `Start()` checks `stopped` flag

The reconnect loop now checks `isStopped()` before reconnecting.
When `HandleChat()` returns an error after `Stop()` was called,
`Start()` returns nil (clean shutdown) instead of reconnecting.

### `cmd/root.go` calls `Stop()` on signal

The signal handler now calls `twitchBot.Stop()` and waits up to
5 seconds for the goroutine to finish before proceeding with
metrics server shutdown.

### Why not context.Context?

`ReadLine()` doesn't accept a context. You'd still need to close
the conn to unblock it. The `Stop()` approach achieves the same
result with no interface changes and minimal code.
