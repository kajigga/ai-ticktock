package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Config holds the application configuration
type Config struct {
	DataFile        string   `json:"data_file"`
	Editor          string   `json:"editor"`
	Accounts        []string `json:"accounts"`
	BackupFile      string   `json:"backup_file,omitempty"`
	LastBackupDate  string   `json:"last_backup_date,omitempty"`
	SpreadsheetFile string   `json:"spreadsheet_file,omitempty"`
	BossEmail       string   `json:"boss_email,omitempty"`
	MailApp         string   `json:"mail_app,omitempty"`
}

// DefaultConfig returns a default configuration with standard values
func DefaultConfig() Config {
	homeDir, _ := os.UserHomeDir()
	defaultDataFile := filepath.Join(homeDir, "Library", "CloudStorage", "OneDrive-ServiceNow", "timetracker", "entries.json")

	return Config{
		DataFile: defaultDataFile,
		Editor:   "vi",
		Accounts: []string{
			"GXO",
			"FCB",
			"Liberty Mutual",
			"Annual Leave",
			"Internal - Non Billable",
			"Acrisure",
			"Acrisure P1",
			"Arrow",
			"Barracuda",
			"IHG",
		},
	}
}

// Validate checks that the configuration has required fields
func (c *Config) Validate() error {
	if c.DataFile == "" {
		return errors.New("data_file is required")
	}
	if len(c.Accounts) == 0 {
		return errors.New("at least one account is required")
	}
	return nil
}

// ToJSON serializes the config to JSON
func (c *Config) ToJSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

// LoadFromJSON deserializes config from JSON string
func LoadFromJSON(data string) (*Config, error) {
	var cfg Config
	err := json.Unmarshal([]byte(data), &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// AddAccount adds an account to the config if it doesn't exist
func (c *Config) AddAccount(name string) {
	for _, a := range c.Accounts {
		if a == name {
			return
		}
	}
	c.Accounts = append(c.Accounts, name)
}

// RemoveAccount removes an account from the config
func (c *Config) RemoveAccount(name string) {
	newAccounts := []string{}
	for _, a := range c.Accounts {
		if a != name {
			newAccounts = append(newAccounts, a)
		}
	}
	c.Accounts = newAccounts
}

// HasAccount checks if an account exists in the config
func (c *Config) HasAccount(name string) bool {
	for _, a := range c.Accounts {
		if a == name {
			return true
		}
	}
	return false
}

// GetEditor returns the editor to use for editing, preferring config, then $EDITOR env var, then default
func (c *Config) GetEditor() string {
	if c.Editor != "" {
		return c.Editor
	}
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}
	return "vi"
}

// GetConfigPath returns the path to the config file
func (c *Config) GetConfigPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".config", "timetracker.json")
}

// LoadConfig loads the config from disk, or creates default if not found
func LoadConfig() (*Config, error) {
	cfg := DefaultConfig()
	path := cfg.GetConfigPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err := SaveConfig(&cfg); err != nil {
				return nil, err
			}
			return &cfg, nil
		}
		return nil, err
	}

	loaded, err := LoadFromJSON(string(data))
	if err != nil {
		return nil, err
	}

	// Merge with defaults
	if loaded.DataFile == "" {
		loaded.DataFile = cfg.DataFile
	}
	if loaded.Editor == "" {
		loaded.Editor = cfg.Editor
	}
	if len(loaded.Accounts) == 0 {
		loaded.Accounts = cfg.Accounts
	}

	return loaded, nil
}

// SaveConfig writes the config to disk
func SaveConfig(cfg *Config) error {
	path := cfg.GetConfigPath()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := cfg.ToJSON()
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// SetBossEmail sets the boss email in config and saves it
func (c *Config) SetBossEmail(email string) {
	c.BossEmail = email
}

// SetSpreadsheetFile sets the spreadsheet file path in config
func (c *Config) SetSpreadsheetFile(path string) {
	c.SpreadsheetFile = path
}

// SetMailApp sets the mail app in config
func (c *Config) SetMailApp(app string) {
	c.MailApp = app
}

// SetBackupFile sets the backup file path in config
func (c *Config) SetBackupFile(path string) {
	c.BackupFile = path
}
