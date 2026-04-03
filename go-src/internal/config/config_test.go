package config

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.DataFile == "" {
		t.Error("DataFile should not be empty")
	}
	if len(cfg.Accounts) == 0 {
		t.Error("Accounts should have defaults")
	}
	if cfg.Editor == "" {
		t.Error("Editor should have default")
	}
}

func TestConfig_ToJSON(t *testing.T) {
	cfg := Config{
		DataFile: "/path/to/data.json",
		Editor:   "nvim",
		Accounts: []string{"GXO", "FCB"},
	}

	data, err := cfg.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}

	if len(data) == 0 {
		t.Fatal("ToJSON() returned empty data")
	}

	// Should contain our values
	content := string(data)
	if !contains(content, "GXO") {
		t.Error("JSON should contain GXO")
	}
	if !contains(content, "nvim") {
		t.Error("JSON should contain nvim")
	}
}

func TestConfig_LoadFromJSON(t *testing.T) {
	jsonData := `{
		"data_file": "/custom/path.json",
		"editor": "vim",
		"accounts": ["Test1", "Test2"]
	}`

	cfg, err := LoadFromJSON(jsonData)
	if err != nil {
		t.Fatalf("LoadFromJSON() error = %v", err)
	}

	if cfg.DataFile != "/custom/path.json" {
		t.Errorf("DataFile = %s, want /custom/path.json", cfg.DataFile)
	}
	if cfg.Editor != "vim" {
		t.Errorf("Editor = %s, want vim", cfg.Editor)
	}
	if len(cfg.Accounts) != 2 || cfg.Accounts[0] != "Test1" {
		t.Errorf("Accounts = %v, want [Test1 Test2]", cfg.Accounts)
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "valid",
			cfg:     Config{DataFile: "/path.json", Accounts: []string{"A"}},
			wantErr: false,
		},
		{
			name:    "empty data file",
			cfg:     Config{DataFile: "", Accounts: []string{"A"}},
			wantErr: true,
		},
		{
			name:    "empty accounts",
			cfg:     Config{DataFile: "/path.json", Accounts: []string{}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfig_AddAccount(t *testing.T) {
	cfg := Config{
		Accounts: []string{"GXO", "FCB"},
	}

	cfg.AddAccount("IHG")
	if len(cfg.Accounts) != 3 {
		t.Errorf("len(Accounts) = %d, want 3", len(cfg.Accounts))
	}
	if cfg.Accounts[2] != "IHG" {
		t.Errorf("Accounts[2] = %s, want IHG", cfg.Accounts[2])
	}

	// Test duplicate
	cfg.AddAccount("GXO")
	if len(cfg.Accounts) != 3 {
		t.Errorf("Duplicate should not be added, len = %d", len(cfg.Accounts))
	}
}

func TestConfig_RemoveAccount(t *testing.T) {
	cfg := Config{
		Accounts: []string{"GXO", "FCB", "IHG"},
	}

	cfg.RemoveAccount("FCB")
	if len(cfg.Accounts) != 2 {
		t.Errorf("len(Accounts) = %d, want 2", len(cfg.Accounts))
	}
	if cfg.Accounts[0] != "GXO" || cfg.Accounts[1] != "IHG" {
		t.Errorf("Accounts = %v, want [GXO IHG]", cfg.Accounts)
	}

	// Test removing non-existent
	before := len(cfg.Accounts)
	cfg.RemoveAccount("NonExistent")
	if len(cfg.Accounts) != before {
		t.Error("Removing non-existent should not change list")
	}
}

func TestConfig_GetEditor(t *testing.T) {
	tests := []struct {
		name       string
		cfg        Config
		envVar     string
		wantEditor string
	}{
		{
			name:       "config has editor",
			cfg:        Config{Editor: "vim"},
			wantEditor: "vim",
		},
		{
			name:       "fallback to EDITOR env",
			cfg:        Config{Editor: ""},
			envVar:     "nvim",
			wantEditor: "nvim",
		},
		{
			name:       "fallback to default",
			cfg:        Config{Editor: ""},
			envVar:     "",
			wantEditor: "vi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear and set env
			os.Unsetenv("EDITOR")
			if tt.envVar != "" {
				os.Setenv("EDITOR", tt.envVar)
			}
			defer os.Unsetenv("EDITOR")

			got := tt.cfg.GetEditor()
			if got != tt.wantEditor {
				t.Errorf("GetEditor() = %s, want %s", got, tt.wantEditor)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
