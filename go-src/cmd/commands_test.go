package cmd

import (
	"testing"
	"time"

	"timetracker/internal/models"
)

// ── parseRelativeDate ────────────────────────────────────────────────────────

func TestParseRelativeDate(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	sevenAgo := time.Now().AddDate(0, 0, -7).Format("2006-01-02")

	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"", today, false},
		{"today", today, false},
		{"yesterday", yesterday, false},
		{"-1", yesterday, false},
		{"-7", sevenAgo, false},
		{"2026-03-30", "2026-03-30", false},
		{"invalid", "", true},
		{"-abc", "", true},
		{"-0", "", true}, // n=0 fails the n>0 guard
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseRelativeDate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseRelativeDate(%q) error=%v, wantErr=%v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseRelativeDate(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ── currentWeekMonday ────────────────────────────────────────────────────────

func TestCurrentWeekMonday(t *testing.T) {
	parse := func(s string) time.Time {
		t, _ := time.Parse("2006-01-02", s)
		return t
	}

	tests := []struct {
		ref  string
		want string
	}{
		{"2026-03-30", "2026-03-30"}, // Monday → self
		{"2026-03-31", "2026-03-30"}, // Tuesday
		{"2026-04-01", "2026-03-30"}, // Wednesday
		{"2026-04-04", "2026-03-30"}, // Saturday
		{"2026-04-05", "2026-03-30"}, // Sunday
		{"2026-04-06", "2026-04-06"}, // next Monday
	}

	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			got := currentWeekMonday(parse(tt.ref))
			if got != tt.want {
				t.Errorf("currentWeekMonday(%s) = %s, want %s", tt.ref, got, tt.want)
			}
		})
	}
}

// ── entrySortKey ─────────────────────────────────────────────────────────────

func TestEntrySortKey(t *testing.T) {
	// When CreatedAt is set it takes precedence over Date.
	e := models.TimeEntry{Date: "2026-03-30", CreatedAt: "2026-03-30T09:00:00"}
	if got := entrySortKey(e); got != "2026-03-30T09:00:00" {
		t.Errorf("entrySortKey with CreatedAt = %q, want CreatedAt value", got)
	}

	// When CreatedAt is absent, falls back to Date.
	e2 := models.TimeEntry{Date: "2026-03-30"}
	if got := entrySortKey(e2); got != "2026-03-30" {
		t.Errorf("entrySortKey without CreatedAt = %q, want Date value", got)
	}
}

// ── sortEntries ──────────────────────────────────────────────────────────────

func TestSortEntriesDate(t *testing.T) {
	entries := models.TimeEntries{
		{ID: "c", Date: "2026-04-03"},
		{ID: "a", Date: "2026-03-30"},
		{ID: "b", Date: "2026-04-01"},
	}
	sorted := sortEntries(entries, "date", false)
	want := []string{"2026-03-30", "2026-04-01", "2026-04-03"}
	for i, e := range sorted {
		if e.Date != want[i] {
			t.Errorf("sorted[%d].Date = %s, want %s", i, e.Date, want[i])
		}
	}
}

func TestSortEntriesDateReverse(t *testing.T) {
	entries := models.TimeEntries{
		{ID: "a", Date: "2026-03-30"},
		{ID: "b", Date: "2026-04-01"},
		{ID: "c", Date: "2026-04-03"},
	}
	sorted := sortEntries(entries, "date", true)
	want := []string{"2026-04-03", "2026-04-01", "2026-03-30"}
	for i, e := range sorted {
		if e.Date != want[i] {
			t.Errorf("sorted[%d].Date = %s, want %s", i, e.Date, want[i])
		}
	}
}

func TestSortEntriesHours(t *testing.T) {
	// Default for hours is descending.
	entries := models.TimeEntries{
		{ID: "a", Hours: 1.0},
		{ID: "b", Hours: 8.0},
		{ID: "c", Hours: 4.5},
	}
	sorted := sortEntries(entries, "hours", false)
	want := []float64{8.0, 4.5, 1.0}
	for i, e := range sorted {
		if e.Hours != want[i] {
			t.Errorf("sorted[%d].Hours = %v, want %v", i, e.Hours, want[i])
		}
	}
}

func TestSortEntriesProject(t *testing.T) {
	entries := models.TimeEntries{
		{ID: "a", Project: "IHG"},
		{ID: "b", Project: "Arrow"},
		{ID: "c", Project: "GXO"},
	}
	sorted := sortEntries(entries, "project", false)
	want := []string{"Arrow", "GXO", "IHG"}
	for i, e := range sorted {
		if e.Project != want[i] {
			t.Errorf("sorted[%d].Project = %s, want %s", i, e.Project, want[i])
		}
	}
}

func TestSortEntriesDoesNotMutateOriginal(t *testing.T) {
	entries := models.TimeEntries{
		{ID: "z", Date: "2026-04-03"},
		{ID: "a", Date: "2026-03-30"},
	}
	original := make(models.TimeEntries, len(entries))
	copy(original, entries)

	sortEntries(entries, "date", false)

	for i, e := range entries {
		if e.ID != original[i].ID {
			t.Errorf("original slice mutated at index %d: got %s, want %s", i, e.ID, original[i].ID)
		}
	}
}

// ── filterByDateRange ────────────────────────────────────────────────────────

func TestFilterByDateRange(t *testing.T) {
	entries := models.TimeEntries{
		{ID: "a", Date: "2026-03-28"},
		{ID: "b", Date: "2026-03-30"},
		{ID: "c", Date: "2026-04-01"},
		{ID: "d", Date: "2026-04-03"},
	}

	tests := []struct {
		from    string
		to      string
		wantIDs []string
	}{
		{"", "", []string{"a", "b", "c", "d"}},
		{"2026-03-30", "", []string{"b", "c", "d"}},
		{"", "2026-04-01", []string{"a", "b", "c"}},
		{"2026-03-30", "2026-04-01", []string{"b", "c"}},
		{"2026-04-10", "", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.from+"/"+tt.to, func(t *testing.T) {
			got := filterByDateRange(entries, tt.from, tt.to)
			if len(got) != len(tt.wantIDs) {
				t.Fatalf("len=%d, want %d", len(got), len(tt.wantIDs))
			}
			for i, e := range got {
				if e.ID != tt.wantIDs[i] {
					t.Errorf("[%d] ID=%s, want %s", i, e.ID, tt.wantIDs[i])
				}
			}
		})
	}
}

// ── resolveSelector ──────────────────────────────────────────────────────────

func TestResolveSelector(t *testing.T) {
	entries := models.TimeEntries{
		{ID: "aaa111", Date: "2026-03-28", CreatedAt: "2026-03-28T08:00:00"},
		{ID: "bbb222", Date: "2026-03-30", CreatedAt: "2026-03-30T09:00:00"},
		{ID: "ccc333", Date: "2026-04-01", CreatedAt: "2026-04-01T10:00:00"},
	}

	t.Run("last returns highest createdAt index", func(t *testing.T) {
		idx, err := resolveSelector(entries, "last")
		if err != nil || idx != 2 {
			t.Errorf("last: idx=%d err=%v, want idx=2", idx, err)
		}
	})

	t.Run("first returns lowest createdAt index", func(t *testing.T) {
		idx, err := resolveSelector(entries, "first")
		if err != nil || idx != 0 {
			t.Errorf("first: idx=%d err=%v, want idx=0", idx, err)
		}
	})

	t.Run("N=1 returns last element (Nth from end)", func(t *testing.T) {
		idx, err := resolveSelector(entries, "1")
		if err != nil || idx != 2 {
			t.Errorf("1: idx=%d err=%v, want idx=2", idx, err)
		}
	})

	t.Run("N=2 returns second-to-last", func(t *testing.T) {
		idx, err := resolveSelector(entries, "2")
		if err != nil || idx != 1 {
			t.Errorf("2: idx=%d err=%v, want idx=1", idx, err)
		}
	})

	t.Run("N=3 returns first element", func(t *testing.T) {
		idx, err := resolveSelector(entries, "3")
		if err != nil || idx != 0 {
			t.Errorf("3: idx=%d err=%v, want idx=0", idx, err)
		}
	})

	t.Run("hex ID prefix match", func(t *testing.T) {
		idx, err := resolveSelector(entries, "bbb")
		if err != nil || idx != 1 {
			t.Errorf("bbb: idx=%d err=%v, want idx=1", idx, err)
		}
	})

	t.Run("N out of range returns error", func(t *testing.T) {
		_, err := resolveSelector(entries, "99")
		if err == nil {
			t.Error("expected error for out-of-range index")
		}
	})

	t.Run("unknown selector returns error", func(t *testing.T) {
		_, err := resolveSelector(entries, "zzz999")
		if err == nil {
			t.Error("expected error for unknown selector")
		}
	})

	t.Run("empty entries returns error", func(t *testing.T) {
		_, err := resolveSelector(models.TimeEntries{}, "last")
		if err == nil {
			t.Error("expected error for empty entries")
		}
	})
}
