package models

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"
)

// GenerateID generates a simple UUID-like ID
func GenerateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// TimeEntry represents a single time tracking entry
type TimeEntry struct {
	ID            string  `json:"id"`
	Date          string  `json:"date"`
	WeekStartDate string  `json:"week_start"`
	Project       string  `json:"project"`
	WorkType      string  `json:"work_type"`
	Hours         float64 `json:"hours"`
	Notes         string  `json:"notes"`
	CreatedAt     string  `json:"created_at,omitempty"`
	UpdatedAt     string  `json:"updated_at,omitempty"`
}

// TimeEntries is a collection of TimeEntry
type TimeEntries []TimeEntry

// WeekStart calculates the Monday of the week for the entry's date
func (e *TimeEntry) WeekStart() string {
	if e.WeekStartDate != "" {
		return e.WeekStartDate
	}
	// Calculate from date if WeekStartDate not set
	t, err := ParseDate(e.Date)
	if err != "" {
		return ""
	}
	return t.WeekStart
}

// ParseDate parses a date string and returns a DateInfo
func ParseDate(dateStr string) (DateInfo, string) {
	parsed, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return DateInfo{}, err.Error()
	}

	// Calculate week start (Monday)
	// Go time.Weekday: Sunday=0, Monday=1, ..., Saturday=6
	weekday := int(parsed.Weekday())
	daysToMonday := (weekday + 6) % 7 // Converts to: Mon=0, Tue=1, ..., Sun=6 → then Mon=0, Tue=1...
	weekStart := parsed.AddDate(0, 0, -daysToMonday)

	return DateInfo{
		Date:       dateStr,
		WeekStart:  weekStart.Format("2006-01-02"),
		Month:      int(weekStart.Month()),
		Day:        weekStart.Day(),
		WeekNumber: getWeekNumber(weekStart),
	}, ""
}

// getWeekNumber returns the ISO week number for a given time
func getWeekNumber(t time.Time) int {
	_, week := t.ISOWeek()
	return week
}

// DateInfo contains parsed date information
type DateInfo struct {
	Date       string
	WeekStart  string
	Month      int
	Day        int
	WeekNumber int
}

// SetDate sets the date and calculates week start
func (e *TimeEntry) SetDate(dateStr string) string {
	info, err := ParseDate(dateStr)
	if err != "" {
		return err
	}
	e.Date = dateStr
	e.WeekStartDate = info.WeekStart
	return ""
}

// Validate validates the time entry
func (e *TimeEntry) Validate() error {
	if e.Project == "" {
		return errors.New("project is required")
	}
	if e.WorkType == "" {
		return errors.New("work type is required")
	}
	if !IsValidWorkType(e.WorkType) {
		return errors.New("invalid work type: must be Billable, Non-Billable, or PTO")
	}
	if e.Hours <= 0 {
		return errors.New("hours must be greater than 0")
	}
	if e.Date == "" {
		return errors.New("date is required")
	}
	_, err := ParseDate(e.Date)
	if err != "" {
		return errors.New("invalid date format: use YYYY-MM-DD")
	}
	return nil
}

var validWorkTypes = map[string]bool{
	"Billable":     true,
	"Non-Billable": true,
	"PTO":          true,
}

// IsValidWorkType checks if the work type is valid
func IsValidWorkType(workType string) bool {
	return validWorkTypes[workType]
}

// ValidWorkTypes returns the list of valid work types
func ValidWorkTypes() []string {
	return []string{"Billable", "Non-Billable", "PTO"}
}

// ToJSON converts entries to JSON
func (e TimeEntries) ToJSON() ([]byte, error) {
	return json.MarshalIndent(e, "", "  ")
}

// ParseTimeEntriesJSON parses JSON into TimeEntries
func ParseTimeEntriesJSON(data string) (TimeEntries, error) {
	var entries TimeEntries
	err := json.Unmarshal([]byte(data), &entries)
	return entries, err
}

// ParseTimeEntryJSON parses a single entry from JSON
func ParseTimeEntryJSON(data string) (*TimeEntry, error) {
	var entry TimeEntry
	err := json.Unmarshal([]byte(data), &entry)
	return &entry, err
}

// ToJSON converts a single entry to JSON
func (e *TimeEntry) ToJSON() (string, error) {
	data, err := json.MarshalIndent(e, "", "  ")
	return string(data), err
}

// FilterByWeek filters entries to a specific week
func (e TimeEntries) FilterByWeek(weekStart string) TimeEntries {
	var result TimeEntries
	for _, entry := range e {
		if entry.WeekStart() == weekStart {
			result = append(result, entry)
		}
	}
	return result
}

// FilterByProject filters entries by project
func (e TimeEntries) FilterByProject(project string) TimeEntries {
	var result TimeEntries
	for _, entry := range e {
		if entry.Project == project {
			result = append(result, entry)
		}
	}
	return result
}

// TotalHours calculates total hours
func (e TimeEntries) TotalHours() float64 {
	var total float64
	for _, entry := range e {
		total += entry.Hours
	}
	return total
}

// HoursByProject returns hours grouped by project
func (e TimeEntries) HoursByProject() map[string]float64 {
	result := make(map[string]float64)
	for _, entry := range e {
		result[entry.Project] += entry.Hours
	}
	return result
}

// UniqueProjects returns a sorted list of unique project names
func (e TimeEntries) UniqueProjects() []string {
	projects := make(map[string]bool)
	for _, entry := range e {
		if entry.Project != "" {
			projects[entry.Project] = true
		}
	}

	result := make([]string, 0, len(projects))
	for project := range projects {
		result = append(result, project)
	}
	sort.Strings(result)
	return result
}

// HoursByWorkType returns hours grouped by work type
func (e TimeEntries) HoursByWorkType() map[string]float64 {
	result := make(map[string]float64)
	for _, entry := range e {
		result[entry.WorkType] += entry.Hours
	}
	return result
}

// WeeklySummary contains summary data for a week
type WeeklySummary struct {
	WeekStart        string
	TotalHours       float64
	BillableHours    float64
	NonBillableHours float64
	PTOHours         float64
	HoursByProject   map[string]float64
	EntryCount       int
}

// GenerateWeeklySummary creates a summary for a week
func (e TimeEntries) GenerateWeeklySummary(weekStart string) WeeklySummary {
	weekEntries := e.FilterByWeek(weekStart)

	summary := WeeklySummary{
		WeekStart:      weekStart,
		HoursByProject: make(map[string]float64),
		EntryCount:     len(weekEntries),
	}

	for _, entry := range weekEntries {
		summary.TotalHours += entry.Hours
		summary.HoursByProject[entry.Project] += entry.Hours

		switch entry.WorkType {
		case "Billable":
			summary.BillableHours += entry.Hours
		case "Non-Billable":
			summary.NonBillableHours += entry.Hours
		case "PTO":
			summary.PTOHours += entry.Hours
		}
	}

	return summary
}
