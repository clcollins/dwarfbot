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

## Post-Mortem (PR #8 Review)

_Lessons captured from PR #8 Copilot code review._

### What Went Well

- `Stop()` + connection close pattern cleanly unblocked
  `ReadLine()` without needing context plumbing
- Mutex-protected `stopped` flag prevented race conditions
  on the shutdown signal itself
- 5-second shutdown wait in signal handler gave the goroutine
  time to finish without hanging indefinitely

### What Went Wrong

- **Disconnect reason overwritten during shutdown** (Copilot
  #1/#2): `Stop()` set `lastDisconnectReason = "shutdown"`,
  but `HandleChat()` overwrote it with `"read_error"` when
  the closed connection caused `ReadLine()` to fail. This
  meant shutdown metrics recorded the wrong disconnect
  reason. Caught by Copilot review.

- **Conn accessed without mutex** (Copilot #7/#9): `Stop()`
  reads `db.conn` under `mu`, but `Connect()` and
  `Disconnect()` read/write `db.conn` without the mutex,
  creating a data race. Caught by Copilot review.

- **Stop during retry backoff not interruptible** (Copilot
  #3/#8): If `Stop()` was called during `Connect()`'s retry
  backoff sleep (up to ~10s), the goroutine wouldn't notice
  until the sleep completed, exceeding the 5s shutdown
  window. Should have used a `select` on `time.After()` vs
  the stop channel. Caught by Copilot review.

- **Missing IRC QUIT message** (Copilot #12): The PR
  description mentioned sending a QUIT message to IRC on
  shutdown, but the implementation only closes the connection
  without writing QUIT. Caught by Copilot review.

### Lessons Learned

- When adding a "reason" field that can be set from multiple
  code paths, establish precedence rules — shutdown reason
  should not be overwritable by error-handling paths
- All reads/writes of shared mutable state (`conn`,
  `lastDisconnectReason`) must be under the same mutex, not
  just the flag that gates them
- Interruptible sleeps require `select` with a stop channel
  — `time.Sleep()` in a retry loop is not cancellable
- PR descriptions are reviewed for accuracy — if the
  description claims a behavior, the code must implement it
