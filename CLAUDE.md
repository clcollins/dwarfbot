# DwarfBot - Claude Code Instructions

## CI Requirements

All CI tests must pass locally before creating a commit. Run `make ci-all` (which builds the CI container and runs all checks inside it) and verify all checks pass before committing changes. For a quick Go-only check, use `make ci` (which executes `fmt`, `vet`, `go-test`, and `build`).

## Plan Documents

Plan documents in `docs/plans/` must never be overwritten or deleted. They may only be updated with lessons learned, post-mortem review (PMR) notes, or lint fixes.
