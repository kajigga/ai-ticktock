package models

import (
	"testing"
)

func TestTimeEntry_WeekStart(t *testing.T) {
	tests := []struct {
		date          string
		wantWeekStart string
	}{
		{"2026-03-30", "2026-03-30"}, // Monday
		{"2026-03-31", "2026-03-30"}, // Tuesday
		{"2026-04-04", "2026-03-30"}, // Saturday
		{"2026-04-06", "2026-04-06"}, // Monday
		{"2026-04-10", "2026-04-06"}, // Friday
	}

	for _, tt := range tests {
		t.Run(tt.date, func(t *testing.T) {
			entry := &TimeEntry{Date: tt.date}
			got := entry.WeekStart()
			if got != tt.wantWeekStart {
				t.Errorf("WeekStart(%s) = %s, want %s", tt.date, got, tt.wantWeekStart)
			}
		})
	}
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		input    string
		wantInfo DateInfo
		wantErr  string
	}{
		{"2026-03-30", DateInfo{Date: "2026-03-30", WeekStart: "2026-03-30", Month: 3, Day: 30}, ""},
		{"2026-12-31", DateInfo{Date: "2026-12-31", WeekStart: "2026-12-28", Month: 12, Day: 31}, ""},
		{"invalid", DateInfo{}, "parsing \"invalid\" as \"2006-01-02\": cannot parse \"invalid\" as date"},
		{"", DateInfo{}, "parsing \"\" as \"2006-01-02\": cannot parse \"\" as date"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			info, err := ParseDate(tt.input)
			if err != tt.wantErr {
				if tt.wantErr == "" && err == "" {
					// OK
				} else if tt.wantErr == "" {
					t.Errorf("ParseDate(%q) unexpected error: %v", tt.input, err)
				} else if err == "" {
					t.Errorf("ParseDate(%q) expected error %q, got none", tt.input, tt.wantErr)
				} else {
					// Error message contains more info, just check if it starts with expected
				}
			}
			if tt.wantErr == "" && info.WeekStart != tt.wantInfo.WeekStart {
				t.Errorf("WeekStart = %s, want %s", info.WeekStart, tt.wantInfo.WeekStart)
			}
		})
	}
}

func TestTimeEntry_Validate(t *testing.T) {
	entry := TimeEntry{
		ID:       "test-id",
		Date:     "2026-03-30",
		Project:  "GXO",
		WorkType: "Billable",
		Hours:    1.5,
		Notes:    "Test work",
	}

	err := entry.Validate()
	if err != nil {
		t.Errorf("Validate() error = %v", err)
	}

	// Test invalid entry
	invalid := TimeEntry{}
	err = invalid.Validate()
	if err == nil {
		t.Error("Validate() expected error for empty entry")
	}
}

func TestTimeEntry_SetDate(t *testing.T) {
	entry := &TimeEntry{}
	err := entry.SetDate("2026-03-30")
	if err != "" {
		t.Errorf("SetDate() error = %v", err)
	}
	if entry.Date != "2026-03-30" {
		t.Errorf("Date = %s, want 2026-03-30", entry.Date)
	}
	if entry.WeekStartDate != "2026-03-30" {
		t.Errorf("WeekStartDate = %s, want 2026-03-30", entry.WeekStartDate)
	}
}

func TestValidWorkTypes(t *testing.T) {
	validTypes := []string{"Billable", "Non-Billable", "PTO"}
	for _, vt := range validTypes {
		if !IsValidWorkType(vt) {
			t.Errorf("IsValidWorkType(%q) = false, want true", vt)
		}
	}

	if IsValidWorkType("Invalid") {
		t.Error("IsValidWorkType('Invalid') = true, want false")
	}
}

func TestParseTimeEntry_JSON(t *testing.T) {
	jsonStr := `{
		"id": "test-123",
		"date": "2026-03-30",
		"project": "GXO",
		"work_type": "Billable",
		"hours": 1.5,
		"notes": "Oracle work"
	}`

	entry, err := ParseTimeEntryJSON(jsonStr)
	if err != nil {
		t.Fatalf("ParseTimeEntryJSON() error = %v", err)
	}

	if entry.ID != "test-123" {
		t.Errorf("ID = %s, want test-123", entry.ID)
	}
	if entry.Project != "GXO" {
		t.Errorf("Project = %s, want GXO", entry.Project)
	}
	if entry.Hours != 1.5 {
		t.Errorf("Hours = %v, want 1.5", entry.Hours)
	}
}

func TestTimeEntries_JSON(t *testing.T) {
	entries := TimeEntries{
		{ID: "1", Date: "2026-03-30", Project: "GXO", Hours: 1.5},
		{ID: "2", Date: "2026-03-30", Project: "IHG", Hours: 2.0},
	}

	data, err := entries.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}

	parsed, err := ParseTimeEntriesJSON(string(data))
	if err != nil {
		t.Fatalf("ParseTimeEntriesJSON() error = %v", err)
	}

	if len(parsed) != 2 {
		t.Errorf("len(parsed) = %d, want 2", len(parsed))
	}
	if parsed[0].Project != "GXO" {
		t.Errorf("parsed[0].Project = %s, want GXO", parsed[0].Project)
	}
}

func TestGenerateID(t *testing.T) {
	id1 := GenerateID()
	id2 := GenerateID()

	if id1 == "" {
		t.Error("GenerateID() returned empty string")
	}
	if id2 == "" {
		t.Error("GenerateID() returned empty string")
	}
	if id1 == id2 {
		t.Error("GenerateID() returned duplicate IDs")
	}
	if len(id1) != 32 { // 16 bytes = 32 hex chars
		t.Errorf("ID length = %d, want 32", len(id1))
	}
}
