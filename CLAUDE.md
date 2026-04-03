# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repo layout

```
go-src/    Go source for the timetracker CLI binary
skill/     Claude Code skill files (SKILL.md prompt, export.py, pull_calendar.swift, tt.py)
           Also contains compiled binaries (gitignored): timetracker, pull_calendar
```

`~/.claude/skills/time-entry` is a symlink to `skill/` — the skill is live as soon as files change here.

## Build & test

All commands run from `go-src/`:

```bash
# Build (output goes into skill/ so it's immediately usable)
go build -o ../skill/timetracker .

# Run all tests
go test ./...

# Run a single package's tests
go test ./cmd/...

# Run one test by name
go test ./cmd/... -run TestResolveSelector
```

## Architecture

The binary is a thin CLI shell over three internal packages:

- **`internal/models`** — `TimeEntry` struct, JSON marshal/unmarshal, filter/sort helpers (`FilterByWeek`, `HoursByProject`, `GenerateWeeklySummary`, etc.). No I/O.
- **`internal/storage`** — reads and writes the JSON array on disk. Load/Save/AddEntry/UpdateEntry/DeleteEntry. Calls `models` only.
- **`internal/config`** — reads/writes `~/.config/timetracker.json`. Holds accounts list, data file path, backup path, mail app, etc. Loaded once at startup in `main.go` and passed into commands.
- **`cmd/commands.go`** — all CLI command definitions wired together with `urfave/cli/v3`. Also contains pure helper functions that are unit-tested directly: `sortEntries`, `filterByDateRange`, `parseRelativeDate`, `currentWeekMonday`, `entrySortKey`, `resolveSelector`.
- **`cmd/tui_bubbletea.go`** — the interactive `view` command, implemented with Bubble Tea. Self-contained; no shared state with commands.go.

### Output convention

Every command outputs **JSON by default**. Pass `--human` to get tabular text. The `printJSON()` helper in `commands.go` is the single output path for all machine-readable output.

### Entry selector pattern

`resolveSelector(entries, selector)` is the shared way to target a single entry — used by `amend show` and `amend update`. Accepts: `last`, `first`, `N` (Nth from end), or a hex ID prefix.

### Data file

Entries are stored as a flat JSON array at the path in `config.DataFile` (defaults to OneDrive). The format is append-only in practice; `storage.Save` always writes the full array.

## Code conventions

- All functions need comments — concise but complete.
- Minimise external dependencies; the only intentional ones are `urfave/cli/v3` and `charmbracelet/bubbletea` + `lipgloss`.
- Test behaviour, not implementation. Pure helpers in `cmd/` are tested directly in `commands_test.go`; storage and model tests use temp files / in-memory data.
- `AGENTS.md` in `go-src/` mirrors these conventions and is read by agent sessions scoped to that directory.
