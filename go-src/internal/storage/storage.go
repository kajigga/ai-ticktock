package storage

import (
	"errors"
	"os"

	"timetracker/internal/models"
)

// Storage handles persistence of time entries to a JSON file
type Storage struct {
	filePath string
}

// New creates a new Storage instance with the given file path
func New(filePath string) *Storage {
	return &Storage{filePath: filePath}
}

// Path returns the file path used by this storage instance
func (s *Storage) Path() string {
	return s.filePath
}

// Load reads all time entries from the storage file
func (s *Storage) Load() (models.TimeEntries, error) {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return models.TimeEntries{}, nil
		}
		return nil, err
	}

	if len(data) == 0 {
		return models.TimeEntries{}, nil
	}

	entries, err := models.ParseTimeEntriesJSON(string(data))
	if err != nil {
		return nil, err
	}

	return entries, nil
}

// Save writes all time entries to the storage file
func (s *Storage) Save(entries models.TimeEntries) error {
	dir := s.filePath[:lastIndex(s.filePath, "/")]
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := entries.ToJSON()
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0644)
}

// AddEntry appends a new time entry to the storage
func (s *Storage) AddEntry(entry models.TimeEntry) error {
	entries, err := s.Load()
	if err != nil {
		return err
	}

	entries = append(entries, entry)
	return s.Save(entries)
}

// UpdateEntry replaces an existing entry with the same ID
func (s *Storage) UpdateEntry(updated models.TimeEntry) error {
	entries, err := s.Load()
	if err != nil {
		return err
	}

	found := false
	for i, e := range entries {
		if e.ID == updated.ID {
			entries[i] = updated
			found = true
			break
		}
	}

	if !found {
		return errors.New("entry not found")
	}

	return s.Save(entries)
}

// DeleteEntry removes an entry by its ID
func (s *Storage) DeleteEntry(id string) error {
	entries, err := s.Load()
	if err != nil {
		return err
	}

	newEntries := models.TimeEntries{}
	found := false
	for _, e := range entries {
		if e.ID == id {
			found = true
			continue
		}
		newEntries = append(newEntries, e)
	}

	if !found {
		return errors.New("entry not found")
	}

	return s.Save(newEntries)
}

// GetEntryByID retrieves a single entry by its ID
func (s *Storage) GetEntryByID(id string) (*models.TimeEntry, error) {
	entries, err := s.Load()
	if err != nil {
		return nil, err
	}

	for _, e := range entries {
		if e.ID == id {
			return &e, nil
		}
	}

	return nil, errors.New("entry not found")
}

// lastIndex finds the last occurrence of substr in s
func lastIndex(s string, substr string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if i >= len(substr)-1 && s[i-len(substr)+1:i+1] == substr {
			return i - len(substr) + 1
		}
	}
	return -1
}
