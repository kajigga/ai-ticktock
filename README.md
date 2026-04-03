# ai-ticktock

A time-tracking system built around a Claude Code skill. Log, view, and report on work hours via natural language — Claude handles the interface, a local Go binary handles the data.

## Structure

```
go-src/      Go source for the timetracker CLI binary
skill/       Claude Code skill files (SKILL.md, export script, Swift calendar helper)
```

## How it works

The `time-entry` Claude Code skill drives everything. You talk to Claude; Claude calls the `timetracker` binary to read and write a local JSON file. The binary is the only thing that touches data — Claude never guesses IDs or formats dates.

## Building the binary

```bash
cd go-src
go build -o ~/.claude/skills/time-entry/timetracker .
```

The binary must live at `~/.claude/skills/time-entry/timetracker` for the skill to find it.

## Installing the skill

Copy `skill/SKILL.md` to `~/.claude/skills/time-entry/SKILL.md` and register it in your Claude Code user settings.

## Commands

| Command | Description |
|---|---|
| `add` | Add a single time entry |
| `batch` | Add multiple entries from a JSON array |
| `list` | List entries (`--from`, `--to`, `--type`, `--notes`, `--limit`, `--sort`, `--reverse`) |
| `search <keyword>` | Full-text search across project + notes |
| `day [today\|yesterday\|-N\|DATE]` | Daily summary |
| `weekly [--last\|--week DATE]` | Weekly summary |
| `range --from DATE --to DATE` | Arbitrary date range totals |
| `amend show <last\|first\|N\|ID>` | Show entry details |
| `amend update <selector> field=value …` | Update an entry |
| `view` | Interactive TUI |
| `edit` | Open entry in $EDITOR |
| `delete` | Remove an entry by ID |
| `export` | Export to CSV / JSON / Markdown |
| `accounts` | Manage project accounts |
| `backup` | Daily backup (zip) |
| `config` | Manage configuration |

All commands output JSON by default. Pass `--human` for tabular text.

## Data format

Entries are stored as a JSON array at the path set in `~/.config/timetracker.json → data_file`.

## Running tests

```bash
cd go-src
go test ./...
```
