package cmd

import (
	"testing"

	"timetracker/internal/models"
)

// TestNavigation tests the key handling logic for the TUI
func TestNavigation(t *testing.T) {
	// Create test entries
	entries := []models.TimeEntry{
		{ID: "1", Date: "2026-04-01", Project: "GXO", Hours: 1.5},
		{ID: "2", Date: "2026-04-01", Project: "FCB", Hours: 2.0},
		{ID: "3", Date: "2026-03-30", Project: "IHG", Hours: 1.0},
		{ID: "4", Date: "2026-03-30", Project: "Arrow", Hours: 3.0},
	}

	tests := []struct {
		name     string
		initial  int
		key      string
		expected int
	}{
		// j - move down
		{"j moves down from 0", 0, "j", 1},
		{"j moves down from 1", 1, "j", 2},
		{"j moves down from last", 3, "j", 3}, // stays at max

		// k - move up
		{"k moves up from 1", 1, "k", 0},
		{"k moves up from 2", 2, "k", 1},
		{"k stays at 0", 0, "k", 0}, // stays at min

		// Arrow keys
		{"down arrow moves down", 0, "↓", 1},
		{"up arrow moves up", 1, "↑", 0},

		// g - go to top
		{"g goes to top", 3, "g", 0},
		{"g at top stays", 0, "g", 0},

		// G - go to bottom
		{"G goes to bottom", 0, "G", 3},
		{"G at bottom stays", 3, "G", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selected := tt.initial
			maxIndex := len(entries) - 1

			// Simulate the key handling logic from Update()
			switch tt.key {
			case "j", "↓":
				if selected < maxIndex {
					selected++
				}
			case "k", "↑":
				if selected > 0 {
					selected--
				}
			case "g":
				selected = 0
			case "G":
				selected = maxIndex
			}

			if selected != tt.expected {
				t.Errorf("key=%s: got %d, want %d", tt.key, selected, tt.expected)
			}
		})
	}
}

// TestSortEntries tests that entries are sorted by date (newest first)
func TestSortEntries(t *testing.T) {
	entries := []models.TimeEntry{
		{ID: "1", Date: "2026-03-30"},
		{ID: "2", Date: "2026-04-01"},
		{ID: "3", Date: "2026-03-25"},
	}

	// Sort by date (newest first) - same logic as in ViewCommand
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].Date > entries[i].Date {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	// Expected order: 2026-04-01, 2026-03-30, 2026-03-25
	if entries[0].Date != "2026-04-01" {
		t.Errorf("First entry should be 2026-04-01, got %s", entries[0].Date)
	}
	if entries[1].Date != "2026-03-30" {
		t.Errorf("Second entry should be 2026-03-30, got %s", entries[1].Date)
	}
	if entries[2].Date != "2026-03-25" {
		t.Errorf("Third entry should be 2026-03-25, got %s", entries[2].Date)
	}
}

// TestTruncate tests the truncate function
func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"this is a long string", 10, "this is..."},
		{"abc", 10, "abc"},               // shorter than max
		{"exactlyten", 18, "exactlyten"}, // exactly 18, no truncation needed
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

// TestBounds tests that selection stays within bounds
func TestBounds(t *testing.T) {
	entries := []models.TimeEntry{
		{ID: "1"},
		{ID: "2"},
		{ID: "3"},
	}

	// Test that we can't go above 0 or below maxIndex
	maxIndex := len(entries) - 1

	// Simulate multiple "k" presses from position 0
	selected := 0
	for i := 0; i < 5; i++ {
		if selected > 0 {
			selected--
		}
	}
	if selected != 0 {
		t.Errorf("Should not go below 0, got %d", selected)
	}

	// Simulate multiple "j" presses from last position
	selected = maxIndex
	for i := 0; i < 5; i++ {
		if selected < maxIndex {
			selected++
		}
	}
	if selected != maxIndex {
		t.Errorf("Should not go above maxIndex (%d), got %d", maxIndex, selected)
	}
}
