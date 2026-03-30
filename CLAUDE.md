# DwarfBot - Claude Code Instructions

## CI Requirements

All CI tests must pass locally before creating a commit. Run `make ci` (which executes `fmt`, `vet`, `test`, and `build`) and verify all checks pass before committing changes.

## Plan Documents

Plan documents in `docs/` must never be overwritten or deleted. They may only be updated with lessons learned, post-mortem review (PMR) notes, or lint fixes.
