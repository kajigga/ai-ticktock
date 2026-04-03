package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"timetracker/internal/config"
	"timetracker/internal/models"
	"timetracker/internal/storage"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/urfave/cli/v3"
)

func ViewCommand(s *storage.Storage, cfg *config.Config) *cli.Command {
	return &cli.Command{
		Name:  "view",
		Usage: "Interactive time entry viewer",
		Description: `Open an interactive terminal UI to browse and manage time entries.
Navigate with j/k (vim-style) or arrow keys. Use e to edit, d to delete, q to quit.
Use 'timetracker view --week 2026-03-30' to filter by week.`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "week",
				Usage: "Filter by week start date (YYYY-MM-DD)",
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			weekStart := c.String("week")

			entries, err := s.Load()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading entries: %v\n", err)
				os.Exit(1)
			}

			if weekStart != "" {
				entries = entries.FilterByWeek(weekStart)
			}

			if len(entries) == 0 {
				fmt.Println("No entries to display")
				return nil
			}

			sortedEntries := make([]models.TimeEntry, len(entries))
			copy(sortedEntries, entries)
			for i := 0; i < len(sortedEntries)-1; i++ {
				for j := i + 1; j < len(sortedEntries); j++ {
					if sortedEntries[j].Date > sortedEntries[i].Date {
						sortedEntries[i], sortedEntries[j] = sortedEntries[j], sortedEntries[i]
					}
				}
			}

			p := tea.NewProgram(
				newViewModel(sortedEntries, weekStart, s, cfg),
				tea.WithAltScreen(),
			)
			if _, err := p.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
				os.Exit(1)
			}

			return nil
		},
	}
}

var (
	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Background(lipgloss.Color("236"))

	cellSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("212")).
				Bold(true).
				Underline(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250"))
)

type ViewModel struct {
	entries        models.TimeEntries
	weekStart      string
	selected       int
	selectedColumn int
	storage        *storage.Storage
	config         *config.Config
	quitting       bool
	message        string
	sortField      string
	sortAsc        bool
	filterProject  string
	filterWorkType string
	mode           string
	exportFormat   string
	editField      int
	editBuffer     string
	isNewEntry     bool
	projects       []string
	workTypes      []string
	dropdownIndex  int // For dropdown navigation
	showDropdown   bool
}

var sortFields = []string{"date", "project", "hours", "type"}

func newViewModel(entries []models.TimeEntry, weekStart string, s *storage.Storage, cfg *config.Config) *ViewModel {
	vm := &ViewModel{
		entries:        entries,
		weekStart:      weekStart,
		selected:       0,
		selectedColumn: 0,
		storage:        s,
		config:         cfg,
		sortField:      "date",
		sortAsc:        false,
		filterProject:  "",
		filterWorkType: "",
		mode:           "normal",
		exportFormat:   "csv",
		isNewEntry:     false,
		workTypes:      []string{"Billable", "Non-Billable", "PTO"},
	}
	vm.projects = vm.getUniqueProjects()
	vm.sortEntries()
	return vm
}

func (m *ViewModel) getUniqueProjects() []string {
	entries, err := m.storage.Load()
	if err != nil {
		return []string{}
	}
	return entries.UniqueProjects()
}

func (m *ViewModel) sortEntries() {
	filtered := make([]models.TimeEntry, 0)
	for _, e := range m.entries {
		if m.filterProject != "" && e.Project != m.filterProject {
			continue
		}
		if m.filterWorkType != "" && e.WorkType != m.filterWorkType {
			continue
		}
		filtered = append(filtered, e)
	}

	for i := 0; i < len(filtered)-1; i++ {
		for j := i + 1; j < len(filtered); j++ {
			shouldSwap := false
			switch m.sortField {
			case "date":
				shouldSwap = m.sortAsc && filtered[j].Date < filtered[i].Date ||
					!m.sortAsc && filtered[j].Date > filtered[i].Date
			case "project":
				shouldSwap = m.sortAsc && filtered[j].Project < filtered[i].Project ||
					!m.sortAsc && filtered[j].Project > filtered[i].Project
			case "hours":
				shouldSwap = m.sortAsc && filtered[j].Hours < filtered[i].Hours ||
					!m.sortAsc && filtered[j].Hours > filtered[i].Hours
			case "type":
				shouldSwap = m.sortAsc && filtered[j].WorkType < filtered[i].WorkType ||
					!m.sortAsc && filtered[j].WorkType > filtered[i].WorkType
			}
			if shouldSwap {
				filtered[i], filtered[j] = filtered[j], filtered[i]
			}
		}
	}

	m.entries = filtered
}

func (m *ViewModel) getOriginalEntries() (models.TimeEntries, error) {
	return m.storage.Load()
}

func (m *ViewModel) Init() tea.Cmd {
	return nil
}

func (m *ViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()

		if m.mode == "filter" && len(key) == 1 {
			if key >= "a" && key <= "z" || key >= "A" && key <= "Z" || key == " " || key == "-" {
				m.filterProject += key
			}
			return m, nil
		}

		if m.mode == "filter" && key == "backspace" {
			if len(m.filterProject) > 0 {
				m.filterProject = m.filterProject[:len(m.filterProject)-1]
			}
			return m, nil
		}

		switch key {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "esc":
			if m.mode != "normal" {
				if m.mode == "edit" {
					m.mode = "normal"
					m.editBuffer = ""
					m.sortEntries()
				} else {
					m.mode = "normal"
					m.message = ""
					m.filterProject = ""
				}
			} else {
				m.quitting = true
				return m, tea.Quit
			}

		case "s":
			if m.mode == "normal" {
				m.mode = "sort"
				m.message = "Sort: d=date, p=project, y=hours, t=type | S=toggle direction"
			}

		case "f":
			if m.mode == "normal" {
				m.mode = "filter"
				m.message = "Filter by project (type name, Enter to apply, c=clear, esc=cancel)"
				m.filterProject = ""
			}

		case "e":
			if m.mode == "normal" {
				m.mode = "export"
				m.message = "Export: 1=csv, 2=json, 3=markdown | Enter=save to file"
			}

		case "o":
			if m.mode == "normal" {
				m.addNewEntry()
			}

		case "1":
			if m.mode == "export" {
				m.exportFormat = "csv"
				m.doExport()
				m.mode = "normal"
			} else if m.mode == "normal" {
				m.deleteSelected()
			}

		case "2":
			if m.mode == "export" {
				m.exportFormat = "json"
				m.doExport()
				m.mode = "normal"
			}

		case "3":
			if m.mode == "export" {
				m.exportFormat = "markdown"
				m.doExport()
				m.mode = "normal"
			}

		case "d", "D":
			if m.mode == "sort" {
				m.sortField = "date"
				m.sortAsc = false
				m.sortEntries()
				m.mode = "normal"
				m.message = "Sorted by date (newest first)"
			} else if m.mode == "normal" {
				m.deleteSelected()
			}

		case "p":
			if m.mode == "sort" {
				m.sortField = "project"
				m.sortAsc = false
				m.sortEntries()
				m.mode = "normal"
				m.message = "Sorted by project"
			}

		case "y":
			if m.mode == "sort" {
				m.sortField = "hours"
				m.sortAsc = false
				m.sortEntries()
				m.mode = "normal"
				m.message = "Sorted by hours"
			}

		case "t":
			if m.mode == "sort" {
				m.sortField = "type"
				m.sortAsc = false
				m.sortEntries()
				m.mode = "normal"
				m.message = "Sorted by type"
			}

		case "S":
			if m.mode == "normal" {
				m.sortAsc = !m.sortAsc
				m.sortEntries()
				direction := "ascending"
				if !m.sortAsc {
					direction = "descending"
				}
				m.message = "Sort direction: " + direction
			}

		case "j", "↓", "down", "ctrl+n":
			if m.mode == "edit" && m.showDropdown {
				if m.editField == 1 {
					options := m.getFilteredProjects()
					if m.dropdownIndex < len(options)-1 {
						m.dropdownIndex++
					}
				} else if m.editField == 2 {
					if m.dropdownIndex < len(m.workTypes)-1 {
						m.dropdownIndex++
					}
				}
			} else if m.mode == "normal" && m.selected < len(m.entries)-1 {
				m.selected++
			}

		case "k", "↑", "up", "ctrl+p":
			if m.mode == "edit" && m.showDropdown {
				if m.dropdownIndex > 0 {
					m.dropdownIndex--
				}
			} else if m.mode == "normal" && m.selected > 0 {
				m.selected--
			}

		case "l", "tab", "right":
			if m.mode == "normal" {
				if m.selectedColumn < 4 {
					m.selectedColumn++
				}
			} else if m.mode == "edit" {
				m.moveToNextCell()
			}

		case "h", "shift+tab", "left":
			if m.mode == "normal" {
				if m.selectedColumn > 0 {
					m.selectedColumn--
				}
			} else if m.mode == "edit" {
				m.moveToPrevCell()
			}

		case "g":
			if m.mode == "normal" {
				m.selected = 0
			}

		case "G":
			if m.mode == "normal" {
				m.selected = len(m.entries) - 1
			}

		case "enter":
			if m.mode == "filter" {
				m.mode = "normal"
				m.sortEntries()
				if m.filterProject != "" {
					m.message = "Filter: " + m.filterProject
				} else {
					m.message = "Filter cleared"
				}
			} else if m.mode == "edit" && m.showDropdown {
				if m.editField == 1 {
					options := m.getFilteredProjects()
					if m.dropdownIndex < len(options) {
						m.editBuffer = strings.Replace(options[m.dropdownIndex], " (new)", "", 1)
						m.autoSaveEdit()
						m.showDropdown = false
					}
				} else if m.editField == 2 {
					if m.dropdownIndex < len(m.workTypes) {
						m.editBuffer = m.workTypes[m.dropdownIndex]
						m.autoSaveEdit()
						m.showDropdown = false
					}
				}
			} else if m.mode == "normal" {
				m.startEdit()
			} else if m.mode == "edit" {
				m.saveEdit()
				m.sortEntries()
			}

		case "i":
			if m.mode == "normal" {
				m.startEdit()
			}

		case "E":
			if m.mode == "normal" {
				m.editSelected()
			}

		case "r":
			if m.mode == "normal" {
				m.reloadEntries()
			}

		case "c":
			if m.mode == "filter" {
				m.filterProject = ""
				m.filterWorkType = ""
				m.sortEntries()
				m.mode = "normal"
				m.message = "Filters cleared"
			}

		case "backspace":
			if m.mode == "edit" && len(m.editBuffer) > 0 {
				m.editBuffer = m.editBuffer[:len(m.editBuffer)-1]
				m.autoSaveEdit()
			}

		case "w", "b", "W", "B", "x", "a", "A", "I", "O":
			if m.mode == "edit" {
				// Do nothing - ignore vim operators in edit mode
			}

		default:
			if m.mode == "edit" && len(key) == 1 {
				m.editBuffer += key
				m.autoSaveEdit()
			}
		}
	}
	return m, nil
}

func (m *ViewModel) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}

	var s strings.Builder

	weekLabel := "All Entries"
	if m.weekStart != "" {
		weekLabel = "Week: " + m.weekStart
	}
	s.WriteString(headerStyle.Render("Time Tracker - "+weekLabel) + "\n")
	s.WriteString("\n")

	if m.mode == "sort" {
		s.WriteString(headerStyle.Render("[SORT MODE] ") + infoStyle.Render("d=date, p=project, y=hours, t=type, S=toggle") + "\n")
	} else if m.mode == "filter" {
		s.WriteString(headerStyle.Render("[FILTER MODE] ") + infoStyle.Render(fmt.Sprintf("Project: %s | Enter=apply, c=clear, esc=cancel", m.filterProject)) + "\n")
	} else if m.mode == "edit" {
		s.WriteString(headerStyle.Render("[EDIT MODE] ") + infoStyle.Render("h/l/tab=field, Enter=save, E=external, esc=cancel") + "\n")
	}

	sortIndicator := "↓"
	if m.sortAsc {
		sortIndicator = "↑"
	}
	sortInfo := fmt.Sprintf("Sort: %s %s", m.sortField, sortIndicator)
	if m.filterProject != "" {
		sortInfo += fmt.Sprintf(" | Filter: %s", m.filterProject)
	}
	s.WriteString(infoStyle.Render(sortInfo) + "\n")
	s.WriteString("\n")

	totalHours := 0.0
	billableHours := 0.0
	ptoHours := 0.0
	for _, e := range m.entries {
		totalHours += e.Hours
		if e.WorkType == "Billable" {
			billableHours += e.Hours
		} else if e.WorkType == "PTO" {
			ptoHours += e.Hours
		}
	}
	s.WriteString(infoStyle.Render(fmt.Sprintf("Total: %.1f hrs | Billable: %.1f | PTO: %.1f", totalHours, billableHours, ptoHours)) + "\n")
	s.WriteString("\n")

	s.WriteString(fmt.Sprintf("  %-4s %-10s %-18s %-12s %-6s %s\n", "#", "Date", "Project", "Type", "Hours", "Notes"))
	s.WriteString(fmt.Sprintf("  %-4s %-10s %-18s %-12s %-6s %s\n", "────", "──────────", "──────────────────", "────────────", "──────", "────────────────────────────"))

	for i, e := range m.entries {
		notes := e.Notes
		if len(notes) > 30 {
			notes = notes[:27] + "..."
		}
		if notes == "" {
			notes = "-"
		}

		prefix := "  "
		if i == m.selected {
			prefix = "▶ "
		}

		row := fmt.Sprintf("%-2s%-4d %-10s %-18s %-12s %6.1f %s",
			prefix, i+1, e.Date, truncate(e.Project, 18), e.WorkType, e.Hours, notes)

		if m.mode == "edit" && i == m.selected {
			s.WriteString(m.renderEditRow(e) + "\n")
		} else if i == m.selected {
			s.WriteString(m.renderSelectedRow(e, notes) + "\n")
		} else {
			s.WriteString(normalStyle.Render(row) + "\n")
		}
	}

	s.WriteString("\n")

	if m.mode == "sort" {
		s.WriteString(infoStyle.Render("[d/p/y/t: sort by field] [S: toggle direction] [esc: cancel]") + "\n")
	} else if m.mode == "filter" {
		s.WriteString(infoStyle.Render("[type: enter project name] [enter: apply] [c: clear] [esc: cancel]") + "\n")
	} else if m.mode == "export" {
		s.WriteString(infoStyle.Render("[1: csv] [2: json] [3: markdown] [esc: cancel]") + "\n")
	} else if m.mode == "edit" {
		s.WriteString(infoStyle.Render("[h/l or tab: prev/next field] [Enter: save] [E: external] [esc: cancel]") + "\n")
	} else {
		s.WriteString(infoStyle.Width(80).Render("[j/k: up/down] [h/l: prev/next cell] [o: add] [s: sort] [f: filter] [e: export]") + "\n")
		s.WriteString(infoStyle.Width(80).Render("[Enter/i: edit cell] [d: delete] [g: top] [G: bottom] [r: reload] [q: quit]") + "\n")
	}

	if m.message != "" {
		s.WriteString("\n" + headerStyle.Render(m.message) + "\n")
		m.message = ""
	}

	return s.String()
}

func (m *ViewModel) editSelected() {
	if m.selected < 0 || m.selected >= len(m.entries) {
		return
	}

	entry := m.entries[m.selected]

	jsonData, _ := entry.ToJSON()
	tmpFile, _ := os.CreateTemp("", "timetracker-edit-*.json")
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	tmpFile.Write([]byte(jsonData))
	tmpFile.Close()

	editor := m.config.GetEditor()
	cmd := exec.Command(editor, tmpPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	m.message = "Opening editor... (save and exit to update)"
	if err := cmd.Run(); err != nil {
		m.message = fmt.Sprintf("Editor error: %v", err)
		return
	}

	updatedData, err := os.ReadFile(tmpPath)
	if err != nil {
		m.message = fmt.Sprintf("Error reading file: %v", err)
		return
	}

	updatedEntry, err := models.ParseTimeEntryJSON(string(updatedData))
	if err != nil {
		m.message = fmt.Sprintf("Error parsing: %v", err)
		return
	}

	if err := m.storage.UpdateEntry(*updatedEntry); err != nil {
		m.message = fmt.Sprintf("Error saving: %v", err)
		return
	}

	m.message = "Entry updated!"
}

func (m *ViewModel) startEdit() {
	if m.selected < 0 || m.selected >= len(m.entries) {
		return
	}
	m.mode = "edit"
	m.editField = m.selectedColumn
	m.editBuffer = m.getCurrentFieldValue()
	m.showDropdown = false
	m.dropdownIndex = 0

	// For project (field 1) and work type (field 2), show dropdown immediately
	if m.editField == 1 || m.editField == 2 {
		m.showDropdown = true
		m.dropdownIndex = 0
	}
}

func (m *ViewModel) saveEdit() {
	if m.selected < 0 || m.selected >= len(m.entries) {
		return
	}

	entry := &m.entries[m.selected]

	switch m.editField {
	case 0:
		entry.Date = m.editBuffer
		entry.WeekStartDate = ""
		if info, err := models.ParseDate(m.editBuffer); err == "" {
			entry.WeekStartDate = info.WeekStart
		}
	case 1:
		entry.Project = m.editBuffer
	case 2:
		entry.WorkType = m.editBuffer
	case 3:
		fmt.Sscanf(m.editBuffer, "%f", &entry.Hours)
	case 4:
		entry.Notes = m.editBuffer
	}

	if err := m.storage.UpdateEntry(*entry); err != nil {
		m.message = fmt.Sprintf("Error saving: %v", err)
		return
	}

	m.mode = "normal"
	m.editBuffer = ""
	m.sortEntries()
	m.message = "Entry updated!"
}

func (m *ViewModel) autoSaveEdit() {
	if m.selected < 0 || m.selected >= len(m.entries) {
		return
	}

	entry := &m.entries[m.selected]

	switch m.editField {
	case 0:
		entry.Date = m.editBuffer
		entry.WeekStartDate = ""
		if info, err := models.ParseDate(m.editBuffer); err == "" {
			entry.WeekStartDate = info.WeekStart
		}
	case 1:
		entry.Project = m.editBuffer
	case 2:
		entry.WorkType = m.editBuffer
	case 3:
		fmt.Sscanf(m.editBuffer, "%f", &entry.Hours)
	case 4:
		entry.Notes = m.editBuffer
	}

	if entry.ID == "" || m.isNewEntry {
		if err := m.storage.AddEntry(*entry); err != nil {
			m.message = fmt.Sprintf("Error saving: %v", err)
			return
		}
		m.isNewEntry = false
		m.reloadEntries()
		m.sortEntries()
	} else {
		if err := m.storage.UpdateEntry(*entry); err != nil {
			m.message = fmt.Sprintf("Error saving: %v", err)
			return
		}
		m.sortEntries()
	}
}

func (m *ViewModel) moveToNextCell() {
	if m.editField < 4 {
		m.editField++
		m.editBuffer = m.getCurrentFieldValue()
	} else if m.selected < len(m.entries)-1 {
		m.editField = 0
		m.selected++
		m.editBuffer = m.getCurrentFieldValue()
	}
}

func (m *ViewModel) moveToPrevCell() {
	if m.editField > 0 {
		m.editField--
		m.editBuffer = m.getCurrentFieldValue()
	} else if m.selected > 0 {
		m.editField = 4
		m.selected--
		m.editBuffer = m.getCurrentFieldValue()
	}
}

func (m *ViewModel) getCurrentFieldValue() string {
	if m.selected < 0 || m.selected >= len(m.entries) {
		return ""
	}

	entry := m.entries[m.selected]

	switch m.editField {
	case 0:
		return entry.Date
	case 1:
		return entry.Project
	case 2:
		return entry.WorkType
	case 3:
		return fmt.Sprintf("%.1f", entry.Hours)
	case 4:
		return entry.Notes
	}
	return ""
}

func (m *ViewModel) getFilteredProjects() []string {
	var result []string
	m.editBuffer = strings.ToLower(m.editBuffer)
	for _, p := range m.projects {
		if m.editBuffer == "" || strings.Contains(strings.ToLower(p), m.editBuffer) {
			result = append(result, p)
		}
	}
	if m.editBuffer != "" && !contains(result, m.editBuffer) {
		result = append([]string{m.editBuffer + " (new)"}, result...)
	}
	return result
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}

func (m *ViewModel) deleteSelected() {
	if m.selected < 0 || m.selected >= len(m.entries) {
		return
	}

	entry := m.entries[m.selected]

	if err := m.storage.DeleteEntry(entry.ID); err != nil {
		m.message = fmt.Sprintf("Error: %v", err)
		return
	}

	m.message = "Entry deleted!"
	m.reloadEntries()

	if m.selected >= len(m.entries) && m.selected > 0 {
		m.selected = len(m.entries) - 1
	}
}

func (m *ViewModel) reloadEntries() {
	entries, err := m.storage.Load()
	if err != nil {
		m.message = fmt.Sprintf("Error loading: %v", err)
		return
	}

	if m.weekStart != "" {
		entries = entries.FilterByWeek(m.weekStart)
	}

	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].Date > entries[i].Date {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	m.entries = entries
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func (m *ViewModel) renderEditRow(e models.TimeEntry) string {
	notes := e.Notes
	if len(notes) > 30 {
		notes = notes[:27] + "..."
	}
	if notes == "" {
		notes = "-"
	}

	values := [5]string{
		e.Date,
		truncate(e.Project, 18),
		e.WorkType,
		fmt.Sprintf("%6.1f", e.Hours),
		notes,
	}

	if m.editBuffer != "" {
		switch m.editField {
		case 0:
			values[0] = m.editBuffer
		case 1:
			values[1] = truncate(m.editBuffer, 18)
		case 2:
			values[2] = m.editBuffer
		case 3:
			var h float64
			fmt.Sscanf(m.editBuffer, "%f", &h)
			values[3] = fmt.Sprintf("%6.1f", h)
		case 4:
			values[4] = m.editBuffer
		}
	}

	styles := [5]lipgloss.Style{
		normalStyle,
		normalStyle,
		normalStyle,
		normalStyle,
		normalStyle,
	}
	if m.editField < 5 {
		styles[m.editField] = selectedStyle
	}

	row := "▶ " + fmt.Sprintf("%-4d ", m.selected+1)
	row += styles[0].Render(fmt.Sprintf("%-10s", values[0])) + " "
	row += styles[1].Render(fmt.Sprintf("%-18s", values[1])) + " "
	row += styles[2].Render(fmt.Sprintf("%-12s", values[2])) + " "
	row += styles[3].Render(fmt.Sprintf("%6s", values[3])) + " "
	row += styles[4].Render(values[4])

	var dropdownLines []string
	if m.showDropdown {
		if m.editField == 1 {
			options := m.getFilteredProjects()
			for i, opt := range options {
				prefix := "  "
				if i == m.dropdownIndex {
					prefix = "▶ "
				}
				dropdownLines = append(dropdownLines, prefix+opt)
			}
		} else if m.editField == 2 {
			for i, opt := range m.workTypes {
				prefix := "  "
				if i == m.dropdownIndex {
					prefix = "▶ "
				}
				dropdownLines = append(dropdownLines, prefix+opt)
			}
		}
	}

	var result strings.Builder
	result.WriteString(row)
	for _, line := range dropdownLines {
		result.WriteString("\n")
		result.WriteString(infoStyle.Render(line))
	}

	return result.String()
}

func (m *ViewModel) renderSelectedRow(e models.TimeEntry, notes string) string {
	values := [5]string{
		e.Date,
		truncate(e.Project, 18),
		e.WorkType,
		fmt.Sprintf("%6.1f", e.Hours),
		notes,
	}

	styles := [5]lipgloss.Style{
		normalStyle,
		normalStyle,
		normalStyle,
		normalStyle,
		normalStyle,
	}
	if m.selectedColumn < 5 {
		styles[m.selectedColumn] = cellSelectedStyle
	}

	row := "▶ " + fmt.Sprintf("%-4d ", m.selected+1)
	row += styles[0].Render(fmt.Sprintf("%-10s", values[0])) + " "
	row += styles[1].Render(fmt.Sprintf("%-18s", values[1])) + " "
	row += styles[2].Render(fmt.Sprintf("%-12s", values[2])) + " "
	row += styles[3].Render(fmt.Sprintf("%6s", values[3])) + " "
	row += styles[4].Render(values[4])

	return row
}

func (m *ViewModel) doExport() {
	entries, err := m.getOriginalEntries()
	if err != nil {
		m.message = fmt.Sprintf("Error loading entries: %v", err)
		return
	}

	if m.weekStart != "" {
		entries = entries.FilterByWeek(m.weekStart)
	}

	var filename string
	var content []byte

	switch m.exportFormat {
	case "csv":
		filename = "timetracker_export.csv"
		content, err = exportToCSV(entries)
	case "json":
		filename = "timetracker_export.json"
		content, err = exportToJSON(entries)
	case "markdown":
		filename = "timetracker_export.md"
		content, err = exportToMarkdown(entries)
	default:
		m.message = "Unknown format: " + m.exportFormat
		return
	}

	if err != nil {
		m.message = fmt.Sprintf("Error exporting: %v", err)
		return
	}

	if err := os.WriteFile(filename, content, 0644); err != nil {
		m.message = fmt.Sprintf("Error writing file: %v", err)
		return
	}

	m.message = fmt.Sprintf("Exported to %s", filename)
}

func (m *ViewModel) addNewEntry() {
	today := time.Now().Format("2006-01-02")

	info, _ := models.ParseDate(today)
	weekStart := info.WeekStart

	newEntry := models.TimeEntry{
		ID:            "",
		Date:          today,
		WeekStartDate: weekStart,
		Project:       "",
		WorkType:      "Billable",
		Hours:         8.0,
		Notes:         "",
	}

	m.entries = append([]models.TimeEntry{newEntry}, m.entries...)
	m.selected = 0
	m.selectedColumn = 0
	m.mode = "edit"
	m.editField = 0
	m.editBuffer = ""
	m.isNewEntry = true
}
