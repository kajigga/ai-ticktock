package main

import (
	"context"
	"fmt"
	"os"

	"timetracker/internal/config"
	"timetracker/internal/storage"

	"timetracker/cmd"

	"github.com/urfave/cli/v3"
)

func main() {
	ctx := context.Background()

	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	s := storage.New(cfg.DataFile)

	app := &cli.Command{
		Name:  "timetracker",
		Usage: "Time tracking CLI - track your work hours",
		Description: `A simple time tracking application that stores your time entries locally in JSON.

QUICK START:
  timetracker add -p <project> -h <hours> -n "<notes>"    Add a time entry
  timetracker list                                         View recent entries
  timetracker weekly                                       Show weekly summary
  timetracker weekly --last                                Show last week
  timetracker day yesterday                                Show yesterday's summary
  timetracker search <keyword>                             Search entries
  timetracker range --from DATE --to DATE                  Date range summary
  timetracker view                                         Interactive entry viewer
  timetracker export                                       Export to CSV/JSON/Markdown

COMMANDS:
  add        Add a new time entry
  list       List time entries (filters: --from --to --type --limit --notes)
  search     Full-text search across project and notes
  day        Daily summary (accepts: today, yesterday, -N, YYYY-MM-DD)
  weekly     Show weekly hours summary (--last for previous week)
  range      Arbitrary date range summary
  view       Interactive entry viewer (bubbletea TUI)
  edit       Edit an entry in your $EDITOR
  delete     Delete an entry by ID
  export     Export entries to CSV, JSON, or Markdown
  accounts   Manage project accounts

For more info on a command: timetracker <command> --help`,
		Commands: []*cli.Command{
			cmd.AddCommand(s),
			cmd.ListCommand(s),
			cmd.SearchCommand(s),
			cmd.DayCommand(s),
			cmd.WeeklyCommand(s),
			cmd.RangeCommand(s),
			cmd.ViewCommand(s, cfg),
			cmd.EditCommand(s, cfg),
			cmd.DeleteCommand(s),
			cmd.ExportCommand(s),
			cmd.AccountsCommand(cfg),
			cmd.BatchCommand(s),
			cmd.AmendCommand(s),
			cmd.BackupCommand(s, cfg),
			cmd.ConfigCommand(cfg),
		},
	}

	if err := app.Run(ctx, os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
