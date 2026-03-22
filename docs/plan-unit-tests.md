# Plan: Add Comprehensive Unit Tests

## Context

Dwarfbot had zero test coverage. This PR adds comprehensive unit tests for all
packages, covering the IRC protocol handling, command parsing and routing,
helper functions, and CLI flag configuration.

## Testing Strategy

### Key Challenge: Raw TCP Connections

DwarfBot writes directly to `net.Conn` for IRC communication. Rather than
introducing heavy mocking frameworks or major refactoring, we use Go's
`net.Pipe()` to create in-memory connection pairs. This lets us:

- Capture everything the bot writes (PRIVMSG, JOIN, PART, etc.)
- Feed simulated IRC lines into the bot's HandleChat loop
- Test the full command pipeline end-to-end

### Refactoring for Testability

One minimal change was needed: `Die()` called `os.Exit()` directly, which
would kill the test process. An `exitFunc` field was added to DwarfBot
(defaults to `os.Exit` when nil) that tests can replace with a recorder.

## Test Files Created

### `pkg/dwarfbot/dwarfbot_test.go`
- **Regex validation**: `msgRegex` and `cmdRegex` against valid/invalid IRC messages
- **Say()**: Verifies PRIVMSG format, empty message error, closed connection error
- **Authenticate()**: Verifies PASS and NICK commands sent
- **JoinChannel()**: Verifies JOIN format, lowercase enforcement, empty channel no-op
- **PartChannel()**: Verifies PART format, empty channel no-op
- **Die()**: Verifies exitFunc is called with correct exit code
- **Disconnect()**: Verifies clean connection close
- **HandleChat()**: End-to-end tests for PING/PONG, PRIVMSG commands, wrong bot alias, verbose mode, admin shutdown
- **Aliases**: Verifies bot alias list

### `pkg/dwarfbot/commands_test.go`
- **contains()**: Present/absent items, empty slice, case sensitivity
- **reContains()**: Matching/non-matching patterns, empty slice
- **ping()**: Default "Pong" response, "Heyo" response, extended "heyooo" response
- **channels()**: Channel list formatting with and without extra channels
- **parseCommand()**: Routing to ping, channels, unknown commands
- **parseAdminCommand()**: Admin shutdown (owner), non-admin rejection, unknown admin commands

### `cmd/root_test.go`
- Root command exists and is named correctly
- All expected flags exist with correct shorthands
- Default values for server, port, verbose flags

## Coverage

| Package | Coverage |
|---------|----------|
| `dwarfbot/pkg/dwarfbot` | 74.4% |
| `dwarfbot/cmd` | 37.5% |

The remaining uncovered code is primarily:
- `Start()` — infinite reconnection loop, tested implicitly via HandleChat
- `Connect()` — requires real TCP server, tested indirectly
- `initConfig()` — requires filesystem/viper setup
- `Execute()` / `Run` closure — requires full CLI invocation

## Running Tests

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run with coverage
go test -cover ./...

# Run specific package
go test ./pkg/dwarfbot/...
```
