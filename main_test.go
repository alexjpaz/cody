package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveCodyConfig(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "simple path",
			path:     "test.code",
			expected: filepath.Join(homeDir, ".code.d", "test.code"),
		},
		{
			name:     "nested path",
			path:     "subdir/test.code",
			expected: filepath.Join(homeDir, ".code.d", "subdir", "test.code"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveCodyConfig(tt.path)
			if result != tt.expected {
				t.Errorf("resolveCodyConfig(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestResolveCodyWorkspaceUrl(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	tests := []struct {
		name     string
		entry    codyEntry
		expected string
	}{
		{
			name:     "git SSH URL with .git suffix",
			entry:    codyEntry{url: "git@github.com:user/repo.git", codePath: "personal"},
			expected: filepath.Join(homeDir, "code", "personal", "github.com", "user", "repo"),
		},
		{
			name:     "git SSH URL without .git suffix",
			entry:    codyEntry{url: "git@gitlab.com:group/project", codePath: "work"},
			expected: filepath.Join(homeDir, "code", "work", "gitlab.com", "group", "project"),
		},
		{
			name:     "non-git URL",
			entry:    codyEntry{url: "https://github.com/user/repo", codePath: "test"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveCodyWorkspaceUrl(tt.entry)
			if result != tt.expected {
				t.Errorf("resolveCodyWorkspaceUrl(%v) = %q, want %q", tt.entry, result, tt.expected)
			}
		})
	}
}

func TestCollectAllCodyEntries(t *testing.T) {
	// Create a temporary directory to simulate .code.d
	tmpDir := t.TempDir()

	// Override home directory for testing
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Create test structure
	codeDir := filepath.Join(tmpDir, ".code.d")
	if err := os.MkdirAll(codeDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create test files
	testFiles := map[string]string{
		"test1.code": "git@github.com:user/repo1.git\ngit@github.com:user/repo2.git\n",
		"test2.code": "git@gitlab.com:group/project.git\n\n",
		"ignore.txt": "should be ignored",
	}

	for filename, content := range testFiles {
		path := filepath.Join(codeDir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	// Test collection
	entries, err := collectAllCodyEntries()
	if err != nil {
		t.Fatalf("collectAllCodyEntries() error = %v", err)
	}

	expected := []codyEntry{
		{url: "git@github.com:user/repo1.git", codePath: "test1"},
		{url: "git@github.com:user/repo2.git", codePath: "test1"},
		{url: "git@gitlab.com:group/project.git", codePath: "test2"},
	}

	if len(entries) != len(expected) {
		t.Errorf("collectAllCodyEntries() returned %d entries, want %d", len(entries), len(expected))
	}

	for _, exp := range expected {
		found := false
		for _, entry := range entries {
			if entry.url == exp.url && entry.codePath == exp.codePath {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected entry %v not found in results", exp)
		}
	}
}

func TestRunAdd(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	codeDir := filepath.Join(tmpDir, ".code.d")
	if err := os.MkdirAll(codeDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	tests := []struct {
		name      string
		codePath  string
		gitURL    string
		wantError bool
	}{
		{
			name:      "add new entry",
			codePath:  "test",
			gitURL:    "git@github.com:user/repo.git",
			wantError: false,
		},
		{
			name:      "add duplicate entry",
			codePath:  "test",
			gitURL:    "git@github.com:user/repo.git",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runAdd(nil, []string{tt.gitURL, tt.codePath})
			if (err != nil) != tt.wantError {
				t.Errorf("runAdd() error = %v, wantError %v", err, tt.wantError)
			}

			// Verify the entry was added
			filePath := resolveCodyConfig(tt.codePath + ".code")
			content, err := os.ReadFile(filePath)
			if err != nil {
				t.Fatalf("Failed to read file: %v", err)
			}

			if !strings.Contains(string(content), tt.gitURL) {
				t.Errorf("File does not contain expected URL %q", tt.gitURL)
			}
		})
	}
}

func TestRunSearch(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	codeDir := filepath.Join(tmpDir, ".code.d")
	if err := os.MkdirAll(codeDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create test file
	testFile := filepath.Join(codeDir, "test.code")
	content := "git@github.com:user/repo1.git\ngit@gitlab.com:user/repo2.git\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name    string
		pattern string
		wantErr bool
	}{
		{
			name:    "search with pattern",
			pattern: "github",
			wantErr: false,
		},
		{
			name:    "search without pattern",
			pattern: "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var args []string
			if tt.pattern != "" {
				args = []string{tt.pattern}
			}

			err := runSearch(nil, args)
			if (err != nil) != tt.wantErr {
				t.Errorf("runSearch() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunOpen(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	codeDir := filepath.Join(tmpDir, ".code.d")
	if err := os.MkdirAll(codeDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create test file
	testFile := filepath.Join(codeDir, "test.code")
	content := "git@github.com:user/repo1.git\ngit@gitlab.com:user/repo2.git\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name    string
		filter  string
		wantErr bool
	}{
		{
			name:    "unique match",
			filter:  "repo1",
			wantErr: false,
		},
		{
			name:    "multiple matches",
			filter:  "user",
			wantErr: true,
		},
		{
			name:    "no matches",
			filter:  "nonexistent",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runOpen(nil, []string{tt.filter})
			if (err != nil) != tt.wantErr {
				t.Errorf("runOpen() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
