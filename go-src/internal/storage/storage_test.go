package storage

import (
	"os"
	"testing"

	"timetracker/internal/models"
)

func TestStorage_Load(t *testing.T) {
	// Create temp file with test data
	tmpFile := t.TempDir() + "/test_entries.json"
	testData := `[
		{
			"id": "test-1",
			"date": "2026-03-30",
			"project": "GXO",
			"work_type": "Billable",
			"hours": 1.5,
			"notes": "Test"
		}
	]`

	if err := os.WriteFile(tmpFile, []byte(testData), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	s := New(tmpFile)
	entries, err := s.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].Project != "GXO" {
		t.Errorf("Project = %s, want GXO", entries[0].Project)
	}
}

func TestStorage_Load_EmptyFile(t *testing.T) {
	tmpFile := t.TempDir() + "/empty.json"

	s := New(tmpFile)
	entries, err := s.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("len(entries) = %d, want 0", len(entries))
	}
}

func TestStorage_Load_NonExistent(t *testing.T) {
	tmpFile := t.TempDir() + "/nonexistent.json"

	s := New(tmpFile)
	entries, err := s.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("len(entries) = %d, want 0", len(entries))
	}
}

func TestStorage_Save(t *testing.T) {
	tmpFile := t.TempDir() + "/test_save.json"

	s := New(tmpFile)
	entries := models.TimeEntries{
		{ID: "1", Date: "2026-03-30", Project: "GXO", Hours: 1.5},
		{ID: "2", Date: "2026-03-30", Project: "IHG", Hours: 2.0},
	}

	err := s.Save(entries)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify by loading
	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded) != 2 {
		t.Errorf("len(loaded) = %d, want 2", len(loaded))
	}
	if loaded[0].Project != "GXO" {
		t.Errorf("loaded[0].Project = %s, want GXO", loaded[0].Project)
	}
}

func TestStorage_AddEntry(t *testing.T) {
	tmpFile := t.TempDir() + "/test_add.json"

	s := New(tmpFile)

	entry := models.TimeEntry{
		ID:       "new-1",
		Date:     "2026-03-30",
		Project:  "GXO",
		WorkType: "Billable",
		Hours:    1.5,
		Notes:    "New entry",
	}

	err := s.AddEntry(entry)
	if err != nil {
		t.Fatalf("AddEntry() error = %v", err)
	}

	entries, _ := s.Load()
	if len(entries) != 1 {
		t.Errorf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].ID != "new-1" {
		t.Errorf("ID = %s, want new-1", entries[0].ID)
	}
}

func TestStorage_UpdateEntry(t *testing.T) {
	tmpFile := t.TempDir() + "/test_update.json"

	// Create initial data
	entries := models.TimeEntries{
		{ID: "existing", Date: "2026-03-30", Project: "GXO", Hours: 1.5},
	}
	s := New(tmpFile)
	s.Save(entries)

	// Update
	updated := models.TimeEntry{
		ID:       "existing",
		Date:     "2026-03-30",
		Project:  "GXO",
		WorkType: "Billable",
		Hours:    2.5, // Changed
		Notes:    "Updated",
	}

	err := s.UpdateEntry(updated)
	if err != nil {
		t.Fatalf("UpdateEntry() error = %v", err)
	}

	loaded, _ := s.Load()
	if len(loaded) != 1 {
		t.Fatalf("len(loaded) = %d, want 1", len(loaded))
	}
	if loaded[0].Hours != 2.5 {
		t.Errorf("Hours = %v, want 2.5", loaded[0].Hours)
	}
	if loaded[0].Notes != "Updated" {
		t.Errorf("Notes = %s, want Updated", loaded[0].Notes)
	}
}

func TestStorage_DeleteEntry(t *testing.T) {
	tmpFile := t.TempDir() + "/test_delete.json"

	entries := models.TimeEntries{
		{ID: "1", Date: "2026-03-30", Project: "GXO"},
		{ID: "2", Date: "2026-03-30", Project: "IHG"},
	}
	s := New(tmpFile)
	s.Save(entries)

	err := s.DeleteEntry("1")
	if err != nil {
		t.Fatalf("DeleteEntry() error = %v", err)
	}

	loaded, _ := s.Load()
	if len(loaded) != 1 {
		t.Errorf("len(loaded) = %d, want 1", len(loaded))
	}
	if loaded[0].ID != "2" {
		t.Errorf("Remaining ID = %s, want 2", loaded[0].ID)
	}
}

func TestStorage_GetEntryByID(t *testing.T) {
	tmpFile := t.TempDir() + "/test_get.json"

	entries := models.TimeEntries{
		{ID: "abc-123", Date: "2026-03-30", Project: "GXO"},
		{ID: "xyz-789", Date: "2026-03-30", Project: "IHG"},
	}
	s := New(tmpFile)
	s.Save(entries)

	entry, err := s.GetEntryByID("abc-123")
	if err != nil {
		t.Fatalf("GetEntryByID() error = %v", err)
	}
	if entry == nil {
		t.Fatal("Entry should not be nil")
	}
	if entry.Project != "GXO" {
		t.Errorf("Project = %s, want GXO", entry.Project)
	}

	// Test non-existent
	_, err = s.GetEntryByID("non-existent")
	if err == nil {
		t.Error("Should return error for non-existent ID")
	}
}
