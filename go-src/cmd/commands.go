package cmd

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"timetracker/internal/config"
	"timetracker/internal/models"
	"timetracker/internal/storage"

	"github.com/urfave/cli/v3"
)

// printJSON marshals v as indented JSON and writes it to stdout.
func printJSON(v interface{}) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
}

// sortEntries returns a new slice sorted by the given field.
// Default directions: date→asc, project→asc, hours→desc.
// --reverse inverts the direction.
func sortEntries(entries models.TimeEntries, by string, reverse bool) models.TimeEntries {
	sorted := make(models.TimeEntries, len(entries))
	copy(sorted, entries)

	var less func(i, j int) bool
	switch by {
	case "hours":
		less = func(i, j int) bool { return sorted[i].Hours > sorted[j].Hours }
	case "project":
		less = func(i, j int) bool { return sorted[i].Project < sorted[j].Project }
	default: // "date"
		less = func(i, j int) bool {
			if sorted[i].Date != sorted[j].Date {
				return sorted[i].Date < sorted[j].Date
			}
			return entrySortKey(sorted[i]) < entrySortKey(sorted[j])
		}
	}

	if reverse {
		orig := less
		less = func(i, j int) bool { return orig(j, i) }
	}

	sort.SliceStable(sorted, less)
	return sorted
}

// addEntryInput holds the parsed command-line arguments for adding an entry
type addEntryInput struct {
	Date     string
	Project  string
	WorkType string
	Hours    float64
	Notes    string
}

// Validate checks that all required fields are present and valid
func (a addEntryInput) Validate() string {
	if a.Date == "" {
		return "date is required (use -d or --date)"
	}
	if a.Project == "" {
		return "project is required (use -p or --project)"
	}
	if a.WorkType == "" {
		a.WorkType = "Billable"
	}
	if !models.IsValidWorkType(a.WorkType) {
		return "invalid work type: must be Billable, Non-Billable, or PTO"
	}
	if a.Hours <= 0 {
		return "hours must be greater than 0 (use -h or --hours)"
	}

	// Validate date format
	_, err := time.Parse("2006-01-02", a.Date)
	if err != nil {
		return "invalid date format: use YYYY-MM-DD"
	}

	return ""
}

// ToTimeEntry converts the input to a models.TimeEntry
func (a addEntryInput) ToTimeEntry() (models.TimeEntry, string) {
	entry := models.TimeEntry{
		ID:        models.GenerateID(),
		Date:      a.Date,
		Project:   a.Project,
		WorkType:  a.WorkType,
		Hours:     a.Hours,
		Notes:     a.Notes,
		CreatedAt: time.Now().Format(time.RFC3339)[:19],
	}

	// Calculate week start
	entry.SetDate(a.Date)

	return entry, ""
}

// AddCommand creates the CLI command for adding time entries
func AddCommand(s *storage.Storage) *cli.Command {
	return &cli.Command{
		Name:  "add",
		Usage: "Add a new time entry",
		Description: `Add a new time tracking entry. Outputs the created entry as JSON.
Examples:
  timetracker add -p GXO -h 1.5 -n "Oracle work"
  timetracker add -p IHG -h 2 -t Billable -d 2026-03-30`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "date",
				Aliases: []string{"d"},
				Usage:   "Date in YYYY-MM-DD format (default: today)",
			},
			&cli.StringFlag{
				Name:     "project",
				Aliases:  []string{"p"},
				Usage:    "Project/Account name (required)",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "type",
				Aliases: []string{"t"},
				Usage:   "Work type: Billable, Non-Billable, or PTO (default: Billable)",
			},
			&cli.Float64Flag{
				Name:     "hours",
				Aliases:  []string{"h"},
				Usage:    "Number of hours worked (required, can be decimal like 1.5)",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "notes",
				Aliases: []string{"n"},
				Usage:   "Notes/description of work done",
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			date := c.String("date")
			if date == "" {
				date = time.Now().Format("2006-01-02")
			}

			workType := c.String("type")
			if workType == "" {
				workType = "Billable"
			}

			input := addEntryInput{
				Date:     date,
				Project:  c.String("project"),
				WorkType: workType,
				Hours:    c.Float64("hours"),
				Notes:    c.String("notes"),
			}

			if err := input.Validate(); err != "" {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				os.Exit(1)
			}

			entry, err := input.ToTimeEntry()
			if err != "" {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				os.Exit(1)
			}

			if err := s.AddEntry(entry); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving entry: %v\n", err)
				os.Exit(1)
			}

			printJSON(map[string]interface{}{"status": "added", "entry": entry})
			return nil
		},
	}
}

// ListCommand creates the CLI command for listing time entries
func ListCommand(s *storage.Storage) *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List time entries",
		Description: `Display time entries with optional filters. Default output is JSON (machine-readable).
Use --human for tabular text output. Entry IDs are always included.
Sorting: --sort date (default, asc), --sort hours (desc), --sort project (asc). Use --reverse to flip.
Examples:
  timetracker list
  timetracker list --human
  timetracker list --week 2026-03-30
  timetracker list --from 2026-03-01 --to 2026-03-31
  timetracker list --project GXO --type Billable
  timetracker list --limit 10 --sort hours
  timetracker list --notes "oracle" --human`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "week", Usage: "Filter by week start date (YYYY-MM-DD)"},
			&cli.StringFlag{Name: "date", Usage: "Filter by specific date (YYYY-MM-DD)"},
			&cli.StringFlag{Name: "from", Usage: "Filter by start date inclusive (YYYY-MM-DD)"},
			&cli.StringFlag{Name: "to", Usage: "Filter by end date inclusive (YYYY-MM-DD)"},
			&cli.StringFlag{Name: "project", Usage: "Filter by project/account name (exact match)"},
			&cli.StringFlag{
				Name:    "type",
				Aliases: []string{"t"},
				Usage:   "Filter by work type: Billable, Non-Billable, or PTO",
			},
			&cli.StringFlag{Name: "notes", Usage: "Filter entries whose notes contain this string (case-insensitive)"},
			&cli.IntFlag{Name: "limit", Aliases: []string{"n"}, Usage: "Return at most N entries"},
			&cli.StringFlag{
				Name:  "sort",
				Usage: "Sort field: date (default), hours, or project",
				Value: "date",
			},
			&cli.BoolFlag{Name: "reverse", Usage: "Reverse the sort order"},
			&cli.BoolFlag{Name: "human", Usage: "Print human-readable tabular text instead of JSON"},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			entries, err := s.Load()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading entries: %v\n", err)
				os.Exit(1)
			}

			// Apply filters
			if week := c.String("week"); week != "" {
				entries = entries.FilterByWeek(week)
			}
			if date := c.String("date"); date != "" {
				filtered := models.TimeEntries{}
				for _, e := range entries {
					if e.Date == date {
						filtered = append(filtered, e)
					}
				}
				entries = filtered
			}
			if from, to := c.String("from"), c.String("to"); from != "" || to != "" {
				entries = filterByDateRange(entries, from, to)
			}
			if project := c.String("project"); project != "" {
				entries = entries.FilterByProject(project)
			}
			if wt := c.String("type"); wt != "" {
				filtered := models.TimeEntries{}
				for _, e := range entries {
					if strings.EqualFold(e.WorkType, wt) {
						filtered = append(filtered, e)
					}
				}
				entries = filtered
			}
			if notesKw := c.String("notes"); notesKw != "" {
				lower := strings.ToLower(notesKw)
				filtered := models.TimeEntries{}
				for _, e := range entries {
					if strings.Contains(strings.ToLower(e.Notes), lower) {
						filtered = append(filtered, e)
					}
				}
				entries = filtered
			}

			// Sort
			entries = sortEntries(entries, c.String("sort"), c.Bool("reverse"))

			// Limit
			if limit := c.Int("limit"); limit > 0 && len(entries) > limit {
				entries = entries[:limit]
			}

			if len(entries) == 0 {
				if c.Bool("human") {
					fmt.Println("No entries found")
				} else {
					printJSON(map[string]interface{}{"entries": []interface{}{}, "total_hours": 0, "entry_count": 0})
				}
				return nil
			}

			if c.Bool("human") {
				fmt.Printf("%-8s | %-10s | %-20s | %-15s | %-5s | %s\n", "ID", "Date", "Project", "Work Type", "Hours", "Notes")
				fmt.Println("-------------------------------------------------------------------------------------")
				for _, e := range entries {
					id := e.ID
					if len(id) > 8 {
						id = id[:8]
					}
					fmt.Printf("%-8s | %-10s | %-20s | %-15s | %-5.1f | %s\n", id, e.Date, e.Project, e.WorkType, e.Hours, e.Notes)
				}
				fmt.Printf("\nTotal: %.1f hours (%d entries)\n", entries.TotalHours(), len(entries))
			} else {
				printJSON(map[string]interface{}{
					"entries":     entries,
					"total_hours": entries.TotalHours(),
					"entry_count": len(entries),
				})
			}
			return nil
		},
	}
}

// parseRelativeDate resolves a date argument that may be a YYYY-MM-DD literal,
// "today", "yesterday", or a negative offset like "-1" or "-7".
func parseRelativeDate(arg string) (string, error) {
	switch {
	case arg == "" || arg == "today":
		return time.Now().Format("2006-01-02"), nil
	case arg == "yesterday" || arg == "-1":
		return time.Now().AddDate(0, 0, -1).Format("2006-01-02"), nil
	case strings.HasPrefix(arg, "-"):
		var n int
		if _, err := fmt.Sscanf(arg, "-%d", &n); err == nil && n > 0 {
			return time.Now().AddDate(0, 0, -n).Format("2006-01-02"), nil
		}
		return "", fmt.Errorf("invalid offset %q (use -1, -2, … for days ago)", arg)
	default:
		if _, err := time.Parse("2006-01-02", arg); err != nil {
			return "", fmt.Errorf("invalid date %q: use YYYY-MM-DD, today, yesterday, or -N", arg)
		}
		return arg, nil
	}
}

// DayCommand creates the CLI command for showing a daily summary
func DayCommand(s *storage.Storage) *cli.Command {
	return &cli.Command{
		Name:  "day",
		Usage: "Show summary for a specific date",
		Description: `Display total hours and per-project breakdown for a single date.
Defaults to today if no date is provided. Accepts relative shorthands.
Examples:
  timetracker day
  timetracker day yesterday
  timetracker day -1
  timetracker day -3
  timetracker day 2026-04-01`,
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "human", Usage: "Print human-readable text instead of JSON"},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			date, err := parseRelativeDate(c.Args().First())
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			entries, err := s.Load()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading entries: %v\n", err)
				os.Exit(1)
			}

			var dayEntries models.TimeEntries
			for _, e := range entries {
				if e.Date == date {
					dayEntries = append(dayEntries, e)
				}
			}

			if len(dayEntries) == 0 {
				if c.Bool("human") {
					fmt.Printf("No entries for %s\n", date)
				} else {
					printJSON(map[string]interface{}{"date": date, "total_hours": 0, "billable": 0, "non_billable": 0, "pto": 0, "by_project": map[string]float64{}, "entries": []interface{}{}})
				}
				return nil
			}

			byType := dayEntries.HoursByWorkType()
			byProject := dayEntries.HoursByProject()

			if c.Bool("human") {
				fmt.Printf("Date: %s\n", date)
				fmt.Printf("Total: %.1fh\n\n", dayEntries.TotalHours())
				fmt.Println("By project:")
				type kv struct {
					k string
					v float64
				}
				var pairs []kv
				for k, v := range byProject {
					pairs = append(pairs, kv{k, v})
				}
				sort.Slice(pairs, func(i, j int) bool { return pairs[i].v > pairs[j].v })
				for _, p := range pairs {
					fmt.Printf("  %-22s %.1fh\n", p.k, p.v)
				}
				fmt.Println("\nEntries:")
				for _, e := range dayEntries {
					notes := ""
					if e.Notes != "" {
						notes = " — " + e.Notes
					}
					fmt.Printf("  %.1fh  %-22s %s%s\n", e.Hours, e.Project, e.WorkType, notes)
				}
			} else {
				printJSON(map[string]interface{}{
					"date":         date,
					"total_hours":  dayEntries.TotalHours(),
					"billable":     byType["Billable"],
					"non_billable": byType["Non-Billable"],
					"pto":          byType["PTO"],
					"by_project":   byProject,
					"entries":      dayEntries,
				})
			}
			return nil
		},
	}
}

// currentWeekMonday returns the Monday of the current week as YYYY-MM-DD.
func currentWeekMonday(ref time.Time) string {
	weekday := int(ref.Weekday())
	daysToMonday := (weekday + 6) % 7
	return ref.AddDate(0, 0, -daysToMonday).Format("2006-01-02")
}

// WeeklyCommand creates the CLI command for showing weekly summary
func WeeklyCommand(s *storage.Storage) *cli.Command {
	return &cli.Command{
		Name:  "weekly",
		Usage: "Show weekly summary",
		Description: `Display a summary of hours for a specific week.
Shows total hours, breakdown by work type (Billable/Non-Billable/PTO),
and hours per project.
Examples:
  timetracker weekly
  timetracker weekly --last
  timetracker weekly --week 2026-03-30`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "week",
				Usage: "Week start date (YYYY-MM-DD, default: current week's Monday)",
			},
			&cli.BoolFlag{
				Name:  "last",
				Usage: "Show last week instead of the current week",
			},
			&cli.BoolFlag{Name: "human", Usage: "Print human-readable text instead of JSON"},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			weekStart := c.String("week")
			if weekStart == "" {
				today := time.Now()
				if c.Bool("last") {
					weekStart = currentWeekMonday(today.AddDate(0, 0, -7))
				} else {
					weekStart = currentWeekMonday(today)
				}
			}

			entries, err := s.Load()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading entries: %v\n", err)
				os.Exit(1)
			}

			summary := entries.GenerateWeeklySummary(weekStart)

			if c.Bool("human") {
				fmt.Printf("Week of %s\n", weekStart)
				fmt.Printf("Total Hours: %.1f\n", summary.TotalHours)
				fmt.Printf("Billable: %.1f | Non-Billable: %.1f | PTO: %.1f\n",
					summary.BillableHours, summary.NonBillableHours, summary.PTOHours)
				fmt.Println("\nBy Project:")
				type kv struct {
					k string
					v float64
				}
				var pairs []kv
				for k, v := range summary.HoursByProject {
					pairs = append(pairs, kv{k, v})
				}
				sort.Slice(pairs, func(i, j int) bool { return pairs[i].v > pairs[j].v })
				for _, p := range pairs {
					fmt.Printf("  %-22s %.1f\n", p.k, p.v)
				}
			} else {
				printJSON(map[string]interface{}{
					"week_start":   weekStart,
					"total_hours":  summary.TotalHours,
					"billable":     summary.BillableHours,
					"non_billable": summary.NonBillableHours,
					"pto":          summary.PTOHours,
					"entry_count":  summary.EntryCount,
					"by_project":   summary.HoursByProject,
				})
			}

			return nil
		},
	}
}

// SearchCommand creates the CLI command for full-text search across entries
func SearchCommand(s *storage.Storage) *cli.Command {
	return &cli.Command{
		Name:  "search",
		Usage: "Search entries by keyword (project name and notes)",
		Description: `Case-insensitive search across project name and notes fields.
Optional filters narrow the search scope.
Examples:
  timetracker search oracle
  timetracker search "schema work" --from 2026-03-01
  timetracker search GXO --type Billable --limit 20`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "from", Usage: "Only search entries on or after this date (YYYY-MM-DD)"},
			&cli.StringFlag{Name: "to", Usage: "Only search entries on or before this date (YYYY-MM-DD)"},
			&cli.StringFlag{
				Name:    "type",
				Aliases: []string{"t"},
				Usage:   "Filter by work type: Billable, Non-Billable, or PTO",
			},
			&cli.IntFlag{Name: "limit", Aliases: []string{"n"}, Usage: "Return at most N results"},
			&cli.BoolFlag{Name: "human", Usage: "Print human-readable tabular text instead of JSON"},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			if !c.Args().Present() {
				fmt.Fprintln(os.Stderr, "Usage: timetracker search <keyword> [flags]")
				os.Exit(1)
			}
			keyword := strings.ToLower(strings.Join(c.Args().Slice(), " "))

			entries, err := s.Load()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading entries: %v\n", err)
				os.Exit(1)
			}

			if from, to := c.String("from"), c.String("to"); from != "" || to != "" {
				entries = filterByDateRange(entries, from, to)
			}
			if wt := c.String("type"); wt != "" {
				filtered := models.TimeEntries{}
				for _, e := range entries {
					if strings.EqualFold(e.WorkType, wt) {
						filtered = append(filtered, e)
					}
				}
				entries = filtered
			}

			var matches models.TimeEntries
			for _, e := range entries {
				if strings.Contains(strings.ToLower(e.Project), keyword) ||
					strings.Contains(strings.ToLower(e.Notes), keyword) {
					matches = append(matches, e)
				}
			}

			if limit := c.Int("limit"); limit > 0 && len(matches) > limit {
				matches = matches[:limit]
			}

			if len(matches) == 0 {
				if c.Bool("human") {
					fmt.Printf("No entries matching %q\n", keyword)
				} else {
					printJSON(map[string]interface{}{"keyword": keyword, "entries": []interface{}{}, "total_hours": 0, "entry_count": 0})
				}
				return nil
			}

			if c.Bool("human") {
				fmt.Printf("%-8s | %-10s | %-20s | %-15s | %-5s | %s\n", "ID", "Date", "Project", "Work Type", "Hours", "Notes")
				fmt.Println("-------------------------------------------------------------------------------------")
				for _, e := range matches {
					id := e.ID
					if len(id) > 8 {
						id = id[:8]
					}
					fmt.Printf("%-8s | %-10s | %-20s | %-15s | %-5.1f | %s\n", id, e.Date, e.Project, e.WorkType, e.Hours, e.Notes)
				}
				fmt.Printf("\nTotal: %.1f hours (%d entries)\n", matches.TotalHours(), len(matches))
			} else {
				printJSON(map[string]interface{}{
					"keyword":     keyword,
					"entries":     matches,
					"total_hours": matches.TotalHours(),
					"entry_count": len(matches),
				})
			}
			return nil
		},
	}
}

// RangeCommand creates the CLI command for summarizing an arbitrary date range
func RangeCommand(s *storage.Storage) *cli.Command {
	return &cli.Command{
		Name:  "range",
		Usage: "Summarize hours for an arbitrary date range",
		Description: `Show totals and per-project breakdown for a custom date range.
Examples:
  timetracker range --from 2026-03-01 --to 2026-03-31
  timetracker range --from 2026-01-01 --to 2026-03-31`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "from",
				Required: true,
				Usage:    "Start date inclusive (YYYY-MM-DD)",
			},
			&cli.StringFlag{
				Name:     "to",
				Required: true,
				Usage:    "End date inclusive (YYYY-MM-DD)",
			},
			&cli.BoolFlag{Name: "human", Usage: "Print human-readable text instead of JSON"},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			from, to := c.String("from"), c.String("to")

			entries, err := s.Load()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading entries: %v\n", err)
				os.Exit(1)
			}

			entries = filterByDateRange(entries, from, to)
			if len(entries) == 0 {
				if c.Bool("human") {
					fmt.Printf("No entries from %s to %s\n", from, to)
				} else {
					printJSON(map[string]interface{}{"from": from, "to": to, "total_hours": 0, "entry_count": 0, "by_project": map[string]float64{}})
				}
				return nil
			}

			byType := entries.HoursByWorkType()
			byProject := entries.HoursByProject()

			if c.Bool("human") {
				total := entries.TotalHours()
				fmt.Printf("Period: %s to %s\n", from, to)
				fmt.Printf("Total:        %.1fh  (%d entries)\n", total, len(entries))
				fmt.Printf("Billable:     %.1fh\n", byType["Billable"])
				fmt.Printf("Non-Billable: %.1fh\n", byType["Non-Billable"])
				fmt.Printf("PTO:          %.1fh\n", byType["PTO"])
				fmt.Println("\nBy Project:")
				type kv struct {
					k string
					v float64
				}
				var pairs []kv
				for k, v := range byProject {
					pairs = append(pairs, kv{k, v})
				}
				sort.Slice(pairs, func(i, j int) bool { return pairs[i].v > pairs[j].v })
				for _, p := range pairs {
					pct := 0.0
					if total > 0 {
						pct = p.v / total * 100
					}
					fmt.Printf("  %-22s %.1fh  (%.0f%%)\n", p.k, p.v, pct)
				}
			} else {
				printJSON(map[string]interface{}{
					"from":         from,
					"to":           to,
					"total_hours":  entries.TotalHours(),
					"billable":     byType["Billable"],
					"non_billable": byType["Non-Billable"],
					"pto":          byType["PTO"],
					"entry_count":  len(entries),
					"by_project":   byProject,
				})
			}
			return nil
		},
	}
}

// EditCommand creates the CLI command for editing entries (opens in $EDITOR)
func EditCommand(s *storage.Storage, cfg *config.Config) *cli.Command {
	return &cli.Command{
		Name:  "edit",
		Usage: "Edit an entry in your default editor",
		Description: `Edit a time entry by opening it in your configured editor ($EDITOR).
The entry is shown as JSON which you can modify. Save and exit to update.
To find the entry ID, use 'timetracker list' first.
Examples:
  timetracker edit abc123def456
  timetracker edit --id abc123def456`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "id",
				Usage: "Entry ID to edit (run 'timetracker list' to see IDs)",
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			id := c.String("id")
			if id == "" && c.Args().Present() {
				id = c.Args().First()
			}

			if id == "" {
				fmt.Println("Usage: timetracker edit <entry-id> or timetracker edit --id <id>")
				fmt.Println("Use 'timetracker list' to see entry IDs")
				os.Exit(1)
			}

			entry, err := s.GetEntryByID(id)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: entry not found\n")
				os.Exit(1)
			}

			// Convert to JSON
			jsonData, err := entry.ToJSON()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			// Write to temp file
			tmpFile, err := os.CreateTemp("", "timetracker-edit-*.json")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating temp file: %v\n", err)
				os.Exit(1)
			}
			tmpPath := tmpFile.Name()
			defer os.Remove(tmpPath)

			if _, err := tmpFile.Write([]byte(jsonData)); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing temp file: %v\n", err)
				os.Exit(1)
			}
			tmpFile.Close()

			// Open in editor
			editor := cfg.GetEditor()

			cmd := exec.Command(editor, tmpPath)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			if err := cmd.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Error running editor: %v\n", err)
				os.Exit(1)
			}

			// Read back the modified file
			updatedData, err := os.ReadFile(tmpPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading modified file: %v\n", err)
				os.Exit(1)
			}

			// Parse and save
			updatedEntry, err := models.ParseTimeEntryJSON(string(updatedData))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing updated entry: %v\n", err)
				os.Exit(1)
			}

			if err := s.UpdateEntry(*updatedEntry); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving entry: %v\n", err)
				os.Exit(1)
			}

			printJSON(map[string]interface{}{"status": "updated", "id": id})
			return nil
		},
	}
}

// DeleteCommand creates the CLI command for deleting entries
func DeleteCommand(s *storage.Storage) *cli.Command {
	return &cli.Command{
		Name:  "delete",
		Usage: "Delete a time entry",
		Description: `Remove a time entry permanently. You must provide the entry ID.
Use 'timetracker list' first to see available entry IDs.
Example:
  timetracker delete abc123def456`,
		Action: func(ctx context.Context, c *cli.Command) error {
			if !c.Args().Present() {
				fmt.Println("Usage: timetracker delete <entry-id>")
				fmt.Println("Use 'timetracker list' to see entry IDs")
				os.Exit(1)
			}

			id := c.Args().First()

			if err := s.DeleteEntry(id); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			printJSON(map[string]interface{}{"status": "deleted", "id": id})
			return nil
		},
	}
}

// AccountsCommand creates the CLI command for managing accounts
func AccountsCommand(cfg *config.Config) *cli.Command {
	return &cli.Command{
		Name:  "accounts",
		Usage: "Manage project accounts",
		Description: `Add, remove, and list project/account names used when logging time entries.
Accounts are stored in ~/.config/timetracker.json and shown as options when adding entries.
Examples:
  timetracker accounts list
  timetracker accounts add "Acme Corp"
  timetracker accounts remove "Old Client"`,
		Commands: []*cli.Command{
			{
				Name:        "list",
				Usage:       "List all saved project accounts",
				Description: "Prints the full list of accounts from ~/.config/timetracker.json, numbered for reference.",
				Action: func(ctx context.Context, c *cli.Command) error {
					config, err := config.LoadConfig()
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error: %v\n", err)
						os.Exit(1)
					}

					accounts := config.Accounts
					if accounts == nil {
						accounts = []string{}
					}
					printJSON(map[string]interface{}{"accounts": accounts})
					return nil
				},
			},
			{
				Name:  "add",
				Usage: "Add a new project account",
				Description: `Add a project or account name to the saved list.
The name is appended to the accounts list in ~/.config/timetracker.json.
Example:
  timetracker accounts add "Acme Corp"`,
				Action: func(ctx context.Context, c *cli.Command) error {
					if !c.Args().Present() {
						fmt.Fprintln(os.Stderr, "Usage: timetracker accounts add <name>")
						os.Exit(1)
					}

					name := c.Args().First()
					cfg.AddAccount(name)

					if err := config.SaveConfig(cfg); err != nil {
						fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
						os.Exit(1)
					}

					printJSON(map[string]interface{}{"status": "added", "account": name, "accounts": cfg.Accounts})
					return nil
				},
			},
			{
				Name:  "remove",
				Usage: "Remove a project account",
				Description: `Remove a project or account name from the saved list.
The name must match exactly (case-sensitive). Use 'accounts list' to see exact names.
Example:
  timetracker accounts remove "Old Client"`,
				Action: func(ctx context.Context, c *cli.Command) error {
					if !c.Args().Present() {
						fmt.Fprintln(os.Stderr, "Usage: timetracker accounts remove <name>")
						os.Exit(1)
					}

					name := c.Args().First()
					if !cfg.HasAccount(name) {
						fmt.Fprintf(os.Stderr, "Error: account '%s' not found\n", name)
						os.Exit(1)
					}

					cfg.RemoveAccount(name)

					if err := config.SaveConfig(cfg); err != nil {
						fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
						os.Exit(1)
					}

					printJSON(map[string]interface{}{"status": "removed", "account": name, "accounts": cfg.Accounts})
					return nil
				},
			},
		},
	}
}

// ExportCommand creates the CLI command for exporting time entries
func ExportCommand(s *storage.Storage) *cli.Command {
	return &cli.Command{
		Name:  "export",
		Usage: "Export time entries to file",
		Description: `Export time entries to CSV, JSON, or Markdown format.
The default format is CSV.
Examples:
  timetracker export -o export.csv
  timetracker export --week 2026-03-30 -o week.csv
  timetracker export --from 2026-03-01 --to 2026-03-31 -f json -o march.json
  timetracker export -f markdown -o summary.md`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "week",
				Usage: "Export single week by start date (YYYY-MM-DD)",
			},
			&cli.StringFlag{
				Name:  "from",
				Usage: "Export start date (YYYY-MM-DD)",
			},
			&cli.StringFlag{
				Name:  "to",
				Usage: "Export end date (YYYY-MM-DD)",
			},
			&cli.StringFlag{
				Name:    "format",
				Aliases: []string{"f"},
				Usage:   "Export format: csv, json, or markdown (default: csv)",
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "Output file path (print to stdout if not specified)",
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			entries, err := s.Load()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading entries: %v\n", err)
				os.Exit(1)
			}

			// Apply date filters
			week := c.String("week")
			from := c.String("from")
			to := c.String("to")

			if week != "" {
				entries = entries.FilterByWeek(week)
			} else if from != "" || to != "" {
				entries = filterByDateRange(entries, from, to)
			}

			if len(entries) == 0 {
				fmt.Println("No entries to export")
				return nil
			}

			format := c.String("format")
			if format == "" {
				format = "csv"
			}
			outputPath := c.String("output")

			var output []byte

			switch format {
			case "json":
				output, err = exportToJSON(entries)
			case "markdown":
				output, err = exportToMarkdown(entries)
			case "csv":
				output, err = exportToCSV(entries)
			default:
				fmt.Fprintf(os.Stderr, "Error: unknown format '%s'. Use csv, json, or markdown\n", format)
				os.Exit(1)
			}

			if err != nil {
				fmt.Fprintf(os.Stderr, "Error generating export: %v\n", err)
				os.Exit(1)
			}

			if outputPath == "" {
				fmt.Print(string(output))
			} else {
				if err := os.WriteFile(outputPath, output, 0644); err != nil {
					fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
					os.Exit(1)
				}
				printJSON(map[string]interface{}{"status": "exported", "file": outputPath, "format": format, "entry_count": len(entries)})
			}

			return nil
		},
	}
}

// filterByDateRange filters entries by date range (inclusive)
func filterByDateRange(entries models.TimeEntries, from, to string) models.TimeEntries {
	var result models.TimeEntries

	for _, e := range entries {
		if from != "" && e.Date < from {
			continue
		}
		if to != "" && e.Date > to {
			continue
		}
		result = append(result, e)
	}

	return result
}

// exportToCSV converts entries to CSV format
func exportToCSV(entries models.TimeEntries) ([]byte, error) {
	var lines []string
	lines = append(lines, "Date,Project,Work Type,Hours,Notes")

	for _, e := range entries {
		// Escape quotes in notes
		notes := e.Notes
		if strings.Contains(notes, "\"") || strings.Contains(notes, ",") {
			notes = "\"" + strings.ReplaceAll(notes, "\"", "\"\"") + "\""
		}
		lines = append(lines, fmt.Sprintf("%s,%s,%s,%.1f,%s", e.Date, e.Project, e.WorkType, e.Hours, notes))
	}

	return []byte(strings.Join(lines, "\n")), nil
}

// exportToJSON exports entries to JSON with metadata
func exportToJSON(entries models.TimeEntries) ([]byte, error) {
	type exportData struct {
		Exported   string             `json:"exported"`
		TotalHours float64            `json:"total_hours"`
		EntryCount int                `json:"entry_count"`
		Entries    models.TimeEntries `json:"entries"`
	}

	data := exportData{
		Exported:   time.Now().Format("2006-01-02"),
		TotalHours: entries.TotalHours(),
		EntryCount: len(entries),
		Entries:    entries,
	}

	return json.MarshalIndent(data, "", "  ")
}

// exportToMarkdown creates a human-readable summary in markdown format
func exportToMarkdown(entries models.TimeEntries) ([]byte, error) {
	var lines []string

	// Sort entries by date first
	sortedEntries := make([]models.TimeEntry, len(entries))
	copy(sortedEntries, entries)
	for i := 0; i < len(sortedEntries)-1; i++ {
		for j := i + 1; j < len(sortedEntries); j++ {
			if sortedEntries[j].Date < sortedEntries[i].Date {
				sortedEntries[i], sortedEntries[j] = sortedEntries[j], sortedEntries[i]
			}
		}
	}

	// Calculate date range
	startDate := sortedEntries[0].Date
	endDate := sortedEntries[len(sortedEntries)-1].Date

	// Unique weeks
	uniqueWeeks := make(map[string]bool)
	weekStarts := []string{}
	for _, e := range entries {
		ws := e.WeekStart()
		if !uniqueWeeks[ws] {
			uniqueWeeks[ws] = true
			weekStarts = append(weekStarts, ws)
		}
	}
	for i := 0; i < len(weekStarts)-1; i++ {
		for j := i + 1; j < len(weekStarts); j++ {
			if weekStarts[j] < weekStarts[i] {
				weekStarts[i], weekStarts[j] = weekStarts[j], weekStarts[i]
			}
		}
	}

	// Calculate metrics
	totalHours := entries.TotalHours()
	billableHours := entries.HoursByWorkType()["Billable"]
	nonBillableHours := entries.HoursByWorkType()["Non-Billable"]
	ptoHours := entries.HoursByWorkType()["PTO"]

	// Days worked (unique dates)
	uniqueDates := make(map[string]bool)
	for _, e := range entries {
		uniqueDates[e.Date] = true
	}
	daysWorked := len(uniqueDates)

	// Standard hours (40 per week)
	standardHours := float64(len(weekStarts)) * 40.0

	// Utilization rate
	utilizationRate := 0.0
	if standardHours > 0 {
		utilizationRate = (billableHours / standardHours) * 100
	}

	// Billable utilization (billable vs non-PTO worked)
	workedHours := billableHours + nonBillableHours
	billableUtilization := 0.0
	if workedHours > 0 {
		billableUtilization = (billableHours / workedHours) * 100
	}

	// === HEADER ===
	lines = append(lines, "# Time Tracking Report")
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("**Period:** %s to %s", formatDate(startDate), formatDate(endDate)))
	lines = append(lines, fmt.Sprintf("**Generated:** %s", time.Now().Format("January 2, 2006")))
	lines = append(lines, "")

	// === KEY METRICS ===
	lines = append(lines, "## Key Metrics")
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("| Metric | Value |"))
	lines = append(lines, "|--------|-------|")
	lines = append(lines, fmt.Sprintf("| Weeks Tracked | %d |", len(weekStarts)))
	lines = append(lines, fmt.Sprintf("| Days Worked | %d |", daysWorked))
	lines = append(lines, fmt.Sprintf("| Total Hours | %.1f |", totalHours))
	lines = append(lines, fmt.Sprintf("| Standard Hours (40/wk) | %.1f |", standardHours))
	lines = append(lines, fmt.Sprintf("| Utilization Rate | %.1f%% |", utilizationRate))
	lines = append(lines, fmt.Sprintf("| Billable Utilization | %.1f%% |", billableUtilization))
	lines = append(lines, "")

	// === HOURS BREAKDOWN ===
	lines = append(lines, "## Hours Breakdown")
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("- **Billable:** %.1f hours", billableHours))
	lines = append(lines, fmt.Sprintf("- **Non-Billable:** %.1f hours", nonBillableHours))
	lines = append(lines, fmt.Sprintf("- **PTO:** %.1f hours", ptoHours))
	lines = append(lines, fmt.Sprintf("- **Total:** %.1f hours", totalHours))
	lines = append(lines, "")

	// === CLIENT DISTRIBUTION ===
	lines = append(lines, "## Client Distribution")
	lines = append(lines, "")
	lines = append(lines, "| Project | Hours | Percentage |")
	lines = append(lines, "|---------|-------|------------|")

	hoursByProject := entries.HoursByProject()
	// Sort by hours descending
	type projectHours struct {
		name  string
		hours float64
	}
	var projectList []projectHours
	for name, hours := range hoursByProject {
		projectList = append(projectList, projectHours{name, hours})
	}
	for i := 0; i < len(projectList)-1; i++ {
		for j := i + 1; j < len(projectList); j++ {
			if projectList[j].hours > projectList[i].hours {
				projectList[i], projectList[j] = projectList[j], projectList[i]
			}
		}
	}

	for _, ph := range projectList {
		pct := 0.0
		if totalHours > 0 {
			pct = (ph.hours / totalHours) * 100
		}
		lines = append(lines, fmt.Sprintf("| %s | %.1f | %.1f%% |", ph.name, ph.hours, pct))
	}
	lines = append(lines, "")

	// === WEEKLY COMPARISON ===
	if len(weekStarts) > 1 {
		lines = append(lines, "## Weekly Comparison")
		lines = append(lines, "")
		lines = append(lines, "| Week | Total | Billable | PTO | Utilization |")
		lines = append(lines, "|------|-------|----------|-----|-------------|")

		for _, weekStart := range weekStarts {
			weekEntries := entries.FilterByWeek(weekStart)
			ws := weekEntries.GenerateWeeklySummary(weekStart)
			weekUtil := 0.0
			if standardHours/float64(len(weekStarts)) > 0 {
				weekUtil = (ws.BillableHours / (standardHours / float64(len(weekStarts)))) * 100
			}
			lines = append(lines, fmt.Sprintf("| %s | %.1f | %.1f | %.1f | %.1f%% |",
				weekStart, ws.TotalHours, ws.BillableHours, ws.PTOHours, weekUtil))
		}
		lines = append(lines, "")
	}

	// === DAILY LOG (events table) ===
	lines = append(lines, "## Daily Log")
	lines = append(lines, "")
	lines = append(lines, "| Date | Project | Type | Hours | Notes |")
	lines = append(lines, "|------|---------|------|-------|-------|")

	for _, e := range sortedEntries {
		notes := e.Notes
		if len(notes) > 50 {
			notes = notes[:47] + "..."
		}
		lines = append(lines, fmt.Sprintf("| %s | %s | %s | %.1f | %s |", e.Date, e.Project, e.WorkType, e.Hours, notes))
	}
	lines = append(lines, "")

	// === WEEKLY SUMMARIES WITH WORK DESCRIPTION ===
	for _, weekStart := range weekStarts {
		weekEntries := entries.FilterByWeek(weekStart)
		ws := weekEntries.GenerateWeeklySummary(weekStart)

		lines = append(lines, fmt.Sprintf("## Week of %s", weekStart))
		lines = append(lines, "")

		// Week summary
		weekUtil := 0.0
		if standardHours/float64(len(weekStarts)) > 0 {
			weekUtil = (ws.BillableHours / 40.0) * 100
		}
		lines = append(lines, fmt.Sprintf("**Total:** %.1f hours | **Billable:** %.1f | **Utilization:** %.1f%%", ws.TotalHours, ws.BillableHours, weekUtil))
		lines = append(lines, "")

		// Work summary
		lines = append(lines, "### Work Completed")
		lines = append(lines, "")
		for _, e := range weekEntries {
			if e.Notes != "" {
				lines = append(lines, fmt.Sprintf("- **%s:** %.1fh - %s", e.Project, e.Hours, e.Notes))
			} else {
				lines = append(lines, fmt.Sprintf("- **%s:** %.1fh", e.Project, e.Hours))
			}
		}
		lines = append(lines, "")
	}

	return []byte(strings.Join(lines, "\n")), nil
}

// formatDate converts YYYY-MM-DD to a more readable format
func formatDate(dateStr string) string {
	if len(dateStr) != 10 {
		return dateStr
	}
	months := []string{"", "January", "February", "March", "April", "May", "June",
		"July", "August", "September", "October", "November", "December"}

	var day, month, year int
	fmt.Sscanf(dateStr, "%d-%d-%d", &year, &month, &day)

	if month >= 1 && month <= 12 {
		return fmt.Sprintf("%s %d, %d", months[month], day, year)
	}
	return dateStr
}

// BatchCommand creates the CLI command for adding multiple entries at once from JSON
func BatchCommand(s *storage.Storage) *cli.Command {
	return &cli.Command{
		Name:  "batch",
		Usage: "Add multiple time entries from a JSON array",
		Description: `Add multiple time entries in one command using a JSON array string.
Each item should have: date, project, hours, and optionally work_type (default: Billable) and notes.
Example:
  timetracker batch '[{"date":"2026-04-01","project":"GXO","hours":2},{"date":"2026-04-01","project":"IHG","hours":3,"notes":"work"}]'`,
		Action: func(ctx context.Context, c *cli.Command) error {
			if !c.Args().Present() {
				fmt.Fprintln(os.Stderr, "Usage: timetracker batch '<json array>'")
				os.Exit(1)
			}

			jsonInput := c.Args().First()

			var items []map[string]interface{}
			if err := json.Unmarshal([]byte(jsonInput), &items); err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing JSON: %v\n", err)
				os.Exit(1)
			}

			entries, err := s.Load()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading entries: %v\n", err)
				os.Exit(1)
			}

			now := time.Now().Format(time.RFC3339)[:19]

			var added models.TimeEntries
			for _, item := range items {
				date, _ := item["date"].(string)
				if date == "" {
					date = time.Now().Format("2006-01-02")
				}
				project, _ := item["project"].(string)
				workType, _ := item["work_type"].(string)
				if workType == "" {
					workType = "Billable"
				}
				var hours float64
				switch v := item["hours"].(type) {
				case float64:
					hours = v
				case int:
					hours = float64(v)
				}
				notes, _ := item["notes"].(string)

				entry := models.TimeEntry{
					ID:        models.GenerateID(),
					Project:   project,
					WorkType:  workType,
					Hours:     hours,
					Notes:     notes,
					CreatedAt: now,
				}
				entry.SetDate(date)

				entries = append(entries, entry)
				added = append(added, entry)
			}

			if err := s.Save(entries); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving entries: %v\n", err)
				os.Exit(1)
			}

			printJSON(map[string]interface{}{"status": "added", "entries": added, "count": len(added)})
			return nil
		},
	}
}

// entrySortKey returns the value used to order entries chronologically.
// Falls back to Date when CreatedAt is empty, so legacy entries without
// the field still sort sensibly relative to entries that have it.
func entrySortKey(e models.TimeEntry) string {
	if e.CreatedAt != "" {
		return e.CreatedAt
	}
	return e.Date
}

// resolveSelector finds a time entry by selector: "last", "first", integer N (Nth from end), or hex ID prefix
func resolveSelector(entries models.TimeEntries, selector string) (int, error) {
	if len(entries) == 0 {
		return -1, fmt.Errorf("no entries found")
	}

	switch selector {
	case "last":
		// Entry with highest sort key (CreatedAt, falling back to Date)
		best := 0
		for i, e := range entries {
			if entrySortKey(e) > entrySortKey(entries[best]) {
				best = i
			}
		}
		return best, nil
	case "first":
		// Entry with lowest sort key (CreatedAt, falling back to Date)
		best := 0
		for i, e := range entries {
			if entrySortKey(e) < entrySortKey(entries[best]) {
				best = i
			}
		}
		return best, nil
	default:
		// Try integer N (Nth from end)
		n := 0
		isInt := true
		for _, ch := range selector {
			if ch < '0' || ch > '9' {
				isInt = false
				break
			}
			n = n*10 + int(ch-'0')
		}
		if isInt && len(selector) > 0 {
			idx := len(entries) - n
			if idx < 0 || idx >= len(entries) {
				return -1, fmt.Errorf("index %d out of range (have %d entries)", n, len(entries))
			}
			return idx, nil
		}

		// Try hex ID prefix
		for i, e := range entries {
			if strings.HasPrefix(e.ID, selector) {
				return i, nil
			}
		}
		return -1, fmt.Errorf("no entry found matching selector %q", selector)
	}
}

// AmendCommand creates the CLI command for inspecting and updating individual entries
func AmendCommand(s *storage.Storage) *cli.Command {
	return &cli.Command{
		Name:  "amend",
		Usage: "Show or update a specific time entry",
		Commands: []*cli.Command{
			{
				Name:  "show",
				Usage: "Show details of a specific entry",
				Description: `Show full details of a time entry. Default output is JSON.
Selector: last, first, N (Nth from end), or hex ID prefix.
Examples:
  timetracker amend show last
  timetracker amend show first
  timetracker amend show 2
  timetracker amend show abc123
  timetracker amend show last --human`,
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "human", Usage: "Print human-readable text instead of JSON"},
				},
				Action: func(ctx context.Context, c *cli.Command) error {
					if !c.Args().Present() {
						fmt.Fprintln(os.Stderr, "Usage: timetracker amend show <selector>")
						fmt.Fprintln(os.Stderr, "Selector: last, first, N (Nth from end), or hex ID prefix")
						os.Exit(1)
					}

					entries, err := s.Load()
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error loading entries: %v\n", err)
						os.Exit(1)
					}

					idx, err := resolveSelector(entries, c.Args().First())
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error: %v\n", err)
						os.Exit(1)
					}

					e := entries[idx]
					if c.Bool("human") {
						fmt.Println("Entry:")
						fmt.Printf("  id:         %s\n", e.ID)
						fmt.Printf("  date:       %s\n", e.Date)
						fmt.Printf("  project:    %s\n", e.Project)
						fmt.Printf("  work_type:  %s\n", e.WorkType)
						fmt.Printf("  hours:      %.1f\n", e.Hours)
						fmt.Printf("  notes:      %s\n", e.Notes)
						if e.UpdatedAt != "" {
							fmt.Printf("  created_at: %s  updated_at: %s\n", e.CreatedAt, e.UpdatedAt)
						} else {
							fmt.Printf("  created_at: %s\n", e.CreatedAt)
						}
					} else {
						printJSON(e)
					}
					return nil
				},
			},
			{
				Name:  "update",
				Usage: "Update fields of a specific entry",
				Description: `Update one or more fields of an entry. Accepts the same selectors as 'amend show':
last, first, N (Nth from end), or hex ID prefix. Supported fields: date, project, work_type, hours, notes.
Examples:
  timetracker amend update last hours=3
  timetracker amend update 2 notes="updated notes"
  timetracker amend update abc123def456 project=GXO hours=3 notes="new notes"`,
				Action: func(ctx context.Context, c *cli.Command) error {
					args := c.Args().Slice()
					if len(args) < 2 {
						fmt.Fprintln(os.Stderr, "Usage: timetracker amend update <selector> [field=value ...]")
						fmt.Fprintln(os.Stderr, "Selector: last, first, N (Nth from end), or hex ID prefix")
						os.Exit(1)
					}

					selector := args[0]
					fieldArgs := args[1:]

					entries, err := s.Load()
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error loading entries: %v\n", err)
						os.Exit(1)
					}

					idx, err := resolveSelector(entries, selector)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error: %v\n", err)
						os.Exit(1)
					}

					entry := entries[idx]

					for _, arg := range fieldArgs {
						parts := strings.SplitN(arg, "=", 2)
						if len(parts) != 2 {
							fmt.Fprintf(os.Stderr, "Error: invalid field=value argument: %s\n", arg)
							os.Exit(1)
						}
						field, value := parts[0], parts[1]
						switch field {
						case "date":
							if errStr := entry.SetDate(value); errStr != "" {
								fmt.Fprintf(os.Stderr, "Error setting date: %s\n", errStr)
								os.Exit(1)
							}
						case "project":
							entry.Project = value
						case "work_type":
							if !models.IsValidWorkType(value) {
								fmt.Fprintf(os.Stderr, "Error: invalid work_type %q\n", value)
								os.Exit(1)
							}
							entry.WorkType = value
						case "hours":
							var h float64
							if _, err := fmt.Sscanf(value, "%f", &h); err != nil || h <= 0 {
								fmt.Fprintf(os.Stderr, "Error: hours must be a positive number\n")
								os.Exit(1)
							}
							entry.Hours = h
						case "notes":
							entry.Notes = value
						default:
							fmt.Fprintf(os.Stderr, "Error: unknown field %q (supported: date, project, work_type, hours, notes)\n", field)
							os.Exit(1)
						}
					}

					now := time.Now().Format(time.RFC3339)[:19]
					entry.UpdatedAt = now
					entries[idx] = entry

					if err := s.Save(entries); err != nil {
						fmt.Fprintf(os.Stderr, "Error saving entries: %v\n", err)
						os.Exit(1)
					}

					printJSON(map[string]interface{}{"status": "updated", "entry": entry})
					return nil
				},
			},
		},
	}
}

// BackupCommand creates the CLI command for backing up the data file
func BackupCommand(s *storage.Storage, cfg *config.Config) *cli.Command {
	return &cli.Command{
		Name:  "backup",
		Usage: "Backup time entries to a zip file",
		Description: `Create a zip backup of the data file, or check backup status.
Examples:
  timetracker backup
  timetracker backup status`,
		Commands: []*cli.Command{
			{
				Name:  "restore",
				Usage: "Restore entries from the backup zip",
				Description: `Preview or restore time entries from the daily backup zip file.
Without --confirm, shows a preview of what would be restored without writing anything.
Always saves a timestamped safety copy of the current data file before overwriting.
Examples:
  timetracker backup restore            (preview only)
  timetracker backup restore --confirm  (actually restore)`,
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "confirm", Usage: "Actually overwrite the data file (omit to preview only)"},
				},
				Action: func(ctx context.Context, c *cli.Command) error {
					if cfg.BackupFile == "" {
						fmt.Fprintln(os.Stderr, "Error: backup_file not configured.")
						os.Exit(1)
					}
					if _, err := os.Stat(cfg.BackupFile); err != nil {
						fmt.Fprintf(os.Stderr, "Error: no backup file found at %s\n", cfg.BackupFile)
						os.Exit(1)
					}

					// Read backup entries
					zr, err := zip.OpenReader(cfg.BackupFile)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error opening zip: %v\n", err)
						os.Exit(1)
					}
					defer zr.Close()

					if len(zr.File) == 0 {
						fmt.Fprintln(os.Stderr, "Error: backup zip is empty")
						os.Exit(1)
					}

					f, err := zr.File[0].Open()
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error reading zip: %v\n", err)
						os.Exit(1)
					}
					defer f.Close()

					var backupEntries models.TimeEntries
					if err := json.NewDecoder(f).Decode(&backupEntries); err != nil {
						fmt.Fprintf(os.Stderr, "Error parsing backup: %v\n", err)
						os.Exit(1)
					}

					oldest, newest := backupEntries[0].Date, backupEntries[0].Date
					for _, e := range backupEntries {
						if e.Date < oldest {
							oldest = e.Date
						}
						if e.Date > newest {
							newest = e.Date
						}
					}

					live, _ := s.Load()
					preview := map[string]interface{}{
						"backup_entry_count": len(backupEntries),
						"live_entry_count":   len(live),
						"oldest":             oldest,
						"newest":             newest,
					}
					if len(live) > len(backupEntries) {
						preview["warning"] = fmt.Sprintf("restoring will lose %d entries not in the backup", len(live)-len(backupEntries))
					}

					if !c.Bool("confirm") {
						preview["status"] = "preview"
						printJSON(preview)
						return nil
					}

					dataFile := s.Path()
					var safetyCopy string
					if _, err := os.Stat(dataFile); err == nil {
						ts := time.Now().Format("20060102_150405")
						safetyCopy = strings.TrimSuffix(dataFile, ".json") + "_pre_restore_" + ts + ".json"
						data, _ := os.ReadFile(dataFile)
						os.WriteFile(safetyCopy, data, 0644)
					}

					if err := s.Save(backupEntries); err != nil {
						fmt.Fprintf(os.Stderr, "Error writing restored data: %v\n", err)
						os.Exit(1)
					}

					result := map[string]interface{}{
						"status":      "restored",
						"entry_count": len(backupEntries),
						"data_file":   dataFile,
					}
					if safetyCopy != "" {
						result["safety_copy"] = safetyCopy
					}
					printJSON(result)
					return nil
				},
			},
			{
				Name:        "status",
				Usage:       "Show backup file location, date, size, and entry count",
				Description: "Reads the configured backup zip and reports its location, last backup date, file size, and how many entries it contains.",
				Action: func(ctx context.Context, c *cli.Command) error {
					result := map[string]interface{}{
						"backup_file":      cfg.BackupFile,
						"last_backup_date": cfg.LastBackupDate,
					}

					if cfg.BackupFile == "" {
						result["error"] = "no backup file configured"
						printJSON(result)
						return nil
					}

					info, statErr := os.Stat(cfg.BackupFile)
					if statErr != nil {
						result["error"] = "file not found"
					} else {
						result["file_size_bytes"] = info.Size()
						if entryCount, countErr := countEntriesInZip(cfg.BackupFile); countErr == nil {
							result["entry_count"] = entryCount
						}
					}

					printJSON(result)
					return nil
				},
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			if cfg.BackupFile == "" {
				fmt.Fprintln(os.Stderr, "Error: backup_file not configured. Use: timetracker config set-backup-file <path>")
				os.Exit(1)
			}

			today := time.Now().Format("2006-01-02")
			if cfg.LastBackupDate == today {
				printJSON(map[string]interface{}{"status": "skipped", "reason": "already backed up today", "date": today, "backup_file": cfg.BackupFile})
				return nil
			}

			if err := createZipBackup(s, cfg.BackupFile); err != nil {
				fmt.Fprintf(os.Stderr, "Error creating backup: %v\n", err)
				os.Exit(1)
			}

			cfg.LastBackupDate = today
			if err := config.SaveConfig(cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
				os.Exit(1)
			}

			printJSON(map[string]interface{}{"status": "backed_up", "backup_file": cfg.BackupFile, "date": today})
			return nil
		},
	}
}

// createZipBackup creates a zip archive of the data file at destPath
func createZipBackup(s *storage.Storage, destPath string) error {
	// Get data file path by loading and re-serializing to temp
	entries, err := s.Load()
	if err != nil {
		return fmt.Errorf("loading entries: %w", err)
	}

	jsonData, err := entries.ToJSON()
	if err != nil {
		return fmt.Errorf("serializing entries: %w", err)
	}

	// Ensure destination directory exists
	destDir := destPath[:lastIdx(destPath, "/")]
	if destDir != "" {
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return fmt.Errorf("creating backup dir: %w", err)
		}
	}

	zipFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating zip file: %w", err)
	}
	defer zipFile.Close()

	zw := zip.NewWriter(zipFile)
	defer zw.Close()

	w, err := zw.Create("entries.json")
	if err != nil {
		return fmt.Errorf("creating zip entry: %w", err)
	}

	if _, err := w.Write(jsonData); err != nil {
		return fmt.Errorf("writing zip entry: %w", err)
	}

	return nil
}

// countEntriesInZip reads a zip backup and returns the number of time entries
func countEntriesInZip(zipPath string) (int, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return 0, err
	}
	defer r.Close()

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return 0, err
		}
		defer rc.Close()

		var buf strings.Builder
		buf.Grow(int(f.UncompressedSize64))
		tmp := make([]byte, 4096)
		for {
			n, readErr := rc.Read(tmp)
			if n > 0 {
				buf.Write(tmp[:n])
			}
			if readErr != nil {
				break
			}
		}

		entries, parseErr := models.ParseTimeEntriesJSON(buf.String())
		if parseErr != nil {
			return 0, parseErr
		}
		return len(entries), nil
	}
	return 0, fmt.Errorf("no files found in zip")
}

// lastIdx finds the last index of substr in s
func lastIdx(s, substr string) int {
	for i := len(s) - len(substr); i >= 0; i-- {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// expandHome expands a leading ~ to the user's home directory
func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			return homeDir + path[1:]
		}
	}
	return path
}

// ConfigCommand creates the CLI command for managing configuration
func ConfigCommand(cfg *config.Config) *cli.Command {
	return &cli.Command{
		Name:  "config",
		Usage: "Manage timetracker configuration",
		Description: `Read and write settings stored at ~/.config/timetracker.json.
Run 'timetracker config show' to see current values.
Examples:
  timetracker config show
  timetracker config init --data-file ~/OneDrive/timetracker/entries.json --backup-file ~/OneDrive/timetracker/backups/entries-backup.zip
  timetracker config set-boss-email manager@example.com
  timetracker config detect-mail-app`,
		Commands: []*cli.Command{
			{
				Name:  "init",
				Usage: "Create initial config and empty data file",
				Description: `Write a new ~/.config/timetracker.json pointing at the given data and backup paths.
Creates intermediate directories and an empty entries JSON file if they don't exist.
On macOS, OneDrive path is auto-detected to suggest defaults.
Example:
  timetracker config init \
    --data-file ~/OneDrive/timetracker/entries.json \
    --backup-file ~/OneDrive/timetracker/backups/entries-backup.zip`,
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "data-file", Usage: "Path for the entries JSON file (e.g. ~/OneDrive/timetracker/entries.json)"},
					&cli.StringFlag{Name: "backup-file", Usage: "Path for the daily backup zip (e.g. ~/OneDrive/timetracker/backups/entries-backup.zip)"},
				},
				Action: func(ctx context.Context, c *cli.Command) error {
					dataFile := expandHome(c.String("data-file"))
					backupFile := expandHome(c.String("backup-file"))

					if dataFile == "" || backupFile == "" {
						// Try to detect OneDrive
						home, _ := os.UserHomeDir()
						cloudDir := home + "/Library/CloudStorage"
						entries, _ := os.ReadDir(cloudDir)
						oneDrive := ""
						for _, e := range entries {
							if e.IsDir() && strings.HasPrefix(e.Name(), "OneDrive-") {
								oneDrive = cloudDir + "/" + e.Name() + "/"
								break
							}
						}
						if oneDrive != "" {
							fmt.Printf("Detected OneDrive: %s\n", oneDrive)
						}
						if dataFile == "" {
							suggested := oneDrive + "timetracker/entries.json"
							fmt.Fprintf(os.Stderr, "Error: --data-file required (suggested: %s)\n", suggested)
							os.Exit(1)
						}
						if backupFile == "" {
							suggested := oneDrive + "timetracker/backups/entries-backup.zip"
							fmt.Fprintf(os.Stderr, "Error: --backup-file required (suggested: %s)\n", suggested)
							os.Exit(1)
						}
					}

					// Create directories
					if err := os.MkdirAll(filepath.Dir(dataFile), 0755); err != nil {
						fmt.Fprintf(os.Stderr, "Error creating data dir: %v\n", err)
						os.Exit(1)
					}
					if err := os.MkdirAll(filepath.Dir(backupFile), 0755); err != nil {
						fmt.Fprintf(os.Stderr, "Error creating backup dir: %v\n", err)
						os.Exit(1)
					}

					// Create empty data file if missing
					if _, err := os.Stat(dataFile); os.IsNotExist(err) {
						if err := os.WriteFile(dataFile, []byte("[]"), 0644); err != nil {
							fmt.Fprintf(os.Stderr, "Error creating data file: %v\n", err)
							os.Exit(1)
						}
					}

					// Write config
					cfg.DataFile = dataFile
					cfg.BackupFile = backupFile
					if err := config.SaveConfig(cfg); err != nil {
						fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
						os.Exit(1)
					}

					printJSON(map[string]interface{}{
						"status":      "created",
						"data_file":   dataFile,
						"backup_file": backupFile,
					})
					return nil
				},
			},
			{
				Name:  "set-boss-email",
				Usage: "Save manager email address for weekly report emails",
				Description: `Saves the recipient email for weekly time report emails to ~/.config/timetracker.json.
Example:
  timetracker config set-boss-email manager@example.com`,
				Action: func(ctx context.Context, c *cli.Command) error {
					if !c.Args().Present() {
						fmt.Fprintln(os.Stderr, "Usage: timetracker config set-boss-email <EMAIL>")
						os.Exit(1)
					}
					email := c.Args().First()
					cfg.SetBossEmail(email)
					if err := config.SaveConfig(cfg); err != nil {
						fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
						os.Exit(1)
					}
					printJSON(map[string]interface{}{
						"status": "saved",
						"key":    "boss_email",
						"value":  email,
					})
					return nil
				},
			},
			{
				Name:  "set-spreadsheet",
				Usage: "Save path to the Excel timesheet for weekly exports",
				Description: `Saves the target .xlsx path used by the export script to ~/.config/timetracker.json.
Supports ~ expansion. Create intermediate directories before exporting.
Example:
  timetracker config set-spreadsheet "~/OneDrive/Timesheets/Timesheet.xlsx"`,
				Action: func(ctx context.Context, c *cli.Command) error {
					if !c.Args().Present() {
						fmt.Fprintln(os.Stderr, "Usage: timetracker config set-spreadsheet <PATH>")
						os.Exit(1)
					}
					path := expandHome(c.Args().First())
					cfg.SetSpreadsheetFile(path)
					if err := config.SaveConfig(cfg); err != nil {
						fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
						os.Exit(1)
					}
					printJSON(map[string]interface{}{
						"status": "saved",
						"key":    "spreadsheet_file",
						"value":  path,
					})
					return nil
				},
			},
			{
				Name:  "set-backup-file",
				Usage: "Save path for the daily backup zip",
				Description: `Saves the destination path for the daily zip backup to ~/.config/timetracker.json.
Should be inside OneDrive or another synced location. Supports ~ expansion.
Example:
  timetracker config set-backup-file "~/OneDrive/timetracker/backups/entries-backup.zip"`,
				Action: func(ctx context.Context, c *cli.Command) error {
					if !c.Args().Present() {
						fmt.Fprintln(os.Stderr, "Usage: timetracker config set-backup-file <PATH>")
						os.Exit(1)
					}
					path := expandHome(c.Args().First())
					cfg.SetBackupFile(path)
					if err := config.SaveConfig(cfg); err != nil {
						fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
						os.Exit(1)
					}
					printJSON(map[string]interface{}{
						"status": "saved",
						"key":    "backup_file",
						"value":  path,
					})
					return nil
				},
			},
			{
				Name:  "set-mail-app",
				Usage: "Set which mail app to use for weekly report drafts",
				Description: `Saves the preferred mail client to ~/.config/timetracker.json.
Valid values: "Microsoft Outlook" or "Mail".
Run 'config detect-mail-app' to set this automatically.
Example:
  timetracker config set-mail-app "Microsoft Outlook"`,
				Action: func(ctx context.Context, c *cli.Command) error {
					if !c.Args().Present() {
						fmt.Fprintln(os.Stderr, "Usage: timetracker config set-mail-app <NAME>")
						os.Exit(1)
					}
					name := c.Args().First()
					if name != "Microsoft Outlook" && name != "Mail" {
						fmt.Fprintf(os.Stderr, "Error: mail_app must be \"Microsoft Outlook\" or \"Mail\"\n")
						os.Exit(1)
					}
					cfg.SetMailApp(name)
					if err := config.SaveConfig(cfg); err != nil {
						fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
						os.Exit(1)
					}
					printJSON(map[string]interface{}{
						"status": "saved",
						"key":    "mail_app",
						"value":  name,
					})
					return nil
				},
			},
			{
				Name:        "detect-mail-app",
				Usage:       "Auto-detect the default mail app and save it to config",
				Description: "Checks for Microsoft Outlook and Apple Mail on disk and saves whichever is found to ~/.config/timetracker.json. Use 'set-mail-app' to override manually.",
				Action: func(ctx context.Context, c *cli.Command) error {
					var detected string
					if _, err := os.Stat("/Applications/Microsoft Outlook.app"); err == nil {
						detected = "Microsoft Outlook"
					} else if _, err := os.Stat("/System/Applications/Mail.app"); err == nil {
						detected = "Mail"
					}

					if detected == "" {
						printJSON(map[string]interface{}{
							"status":  "not_found",
							"message": "could not detect mail app; use: timetracker config set-mail-app <name>",
						})
						return nil
					}

					cfg.SetMailApp(detected)
					if err := config.SaveConfig(cfg); err != nil {
						fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
						os.Exit(1)
					}
					printJSON(map[string]interface{}{
						"status":   "saved",
						"key":      "mail_app",
						"value":    detected,
						"detected": true,
					})
					return nil
				},
			},
			{
				Name:        "show",
				Usage:       "Show all current configuration values",
				Description: "Prints every key from ~/.config/timetracker.json — data file path, editor, accounts list, backup path, spreadsheet path, boss email, and mail app.",
				Action: func(ctx context.Context, c *cli.Command) error {
					accounts := cfg.Accounts
					if accounts == nil {
						accounts = []string{}
					}
					printJSON(map[string]interface{}{
						"data_file":        cfg.DataFile,
						"editor":           cfg.Editor,
						"accounts":         accounts,
						"backup_file":      cfg.BackupFile,
						"last_backup_date": cfg.LastBackupDate,
						"spreadsheet_file": cfg.SpreadsheetFile,
						"boss_email":       cfg.BossEmail,
						"mail_app":         cfg.MailApp,
					})
					return nil
				},
			},
		},
	}
}
