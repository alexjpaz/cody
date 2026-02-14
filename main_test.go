package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
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

func defaultPathTemplate(t *testing.T) *template.Template {
	t.Helper()
	tmpl, err := template.New("path").Parse(pathShorthands["default"])
	if err != nil {
		t.Fatalf("Failed to parse default template: %v", err)
	}
	return tmpl
}

func TestResolveCodyWorkspaceUrl(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	tmpl := defaultPathTemplate(t)

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
			result := resolveCodyWorkspaceUrl(tt.entry, tmpl)
			if result != tt.expected {
				t.Errorf("resolveCodyWorkspaceUrl(%v) = %q, want %q", tt.entry, result, tt.expected)
			}
		})
	}
}

func TestResolveCodyWorkspaceUrlLegacy(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	tmpl, err := template.New("path").Parse(pathShorthands["legacy"])
	if err != nil {
		t.Fatalf("Failed to parse legacy template: %v", err)
	}

	entry := codyEntry{url: "git@github.com:user/repo.git", codePath: "personal"}
	expected := filepath.Join(homeDir, "code", "github.com", "user", "repo")

	result := resolveCodyWorkspaceUrl(entry, tmpl)
	if result != expected {
		t.Errorf("resolveCodyWorkspaceUrl (legacy) = %q, want %q", result, expected)
	}
}

func TestResolveCodyWorkspaceUrlCustomTemplate(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	tmpl, err := template.New("path").Parse("{{.Home}}/src/{{.Host}}/{{.Repo}}")
	if err != nil {
		t.Fatalf("Failed to parse custom template: %v", err)
	}

	entry := codyEntry{url: "git@github.com:user/repo.git", codePath: "personal"}
	expected := filepath.Join(homeDir, "src", "github.com", "repo")

	result := resolveCodyWorkspaceUrl(entry, tmpl)
	if result != expected {
		t.Errorf("resolveCodyWorkspaceUrl (custom) = %q, want %q", result, expected)
	}
}

func TestParseGitURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantHost  string
		wantOwner string
		wantRepo  string
		wantOk    bool
	}{
		{
			name:      "standard SSH URL with .git",
			url:       "git@github.com:user/repo.git",
			wantHost:  "github.com",
			wantOwner: "user",
			wantRepo:  "repo",
			wantOk:    true,
		},
		{
			name:      "SSH URL without .git",
			url:       "git@gitlab.com:group/project",
			wantHost:  "gitlab.com",
			wantOwner: "group",
			wantRepo:  "project",
			wantOk:    true,
		},
		{
			name:   "HTTPS URL",
			url:    "https://github.com/user/repo",
			wantOk: false,
		},
		{
			name:   "empty string",
			url:    "",
			wantOk: false,
		},
		{
			name:   "git@ but no colon",
			url:    "git@github.com/user/repo",
			wantOk: false,
		},
		{
			name:   "git@ with colon but no slash",
			url:    "git@github.com:repo",
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, owner, repo, ok := parseGitURL(tt.url)
			if ok != tt.wantOk {
				t.Errorf("parseGitURL(%q) ok = %v, want %v", tt.url, ok, tt.wantOk)
				return
			}
			if !ok {
				return
			}
			if host != tt.wantHost {
				t.Errorf("parseGitURL(%q) host = %q, want %q", tt.url, host, tt.wantHost)
			}
			if owner != tt.wantOwner {
				t.Errorf("parseGitURL(%q) owner = %q, want %q", tt.url, owner, tt.wantOwner)
			}
			if repo != tt.wantRepo {
				t.Errorf("parseGitURL(%q) repo = %q, want %q", tt.url, repo, tt.wantRepo)
			}
		})
	}
}

func TestResolveConfigDir(t *testing.T) {
	t.Run("uses XDG_CONFIG_HOME when set", func(t *testing.T) {
		original := os.Getenv("XDG_CONFIG_HOME")
		os.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-test")
		defer os.Setenv("XDG_CONFIG_HOME", original)

		result := resolveConfigDir()
		expected := "/tmp/xdg-test/cody"
		if result != expected {
			t.Errorf("resolveConfigDir() = %q, want %q", result, expected)
		}
	})

	t.Run("falls back to ~/.config/cody", func(t *testing.T) {
		original := os.Getenv("XDG_CONFIG_HOME")
		os.Unsetenv("XDG_CONFIG_HOME")
		defer os.Setenv("XDG_CONFIG_HOME", original)

		homeDir, _ := os.UserHomeDir()
		expected := filepath.Join(homeDir, ".config", "cody")

		result := resolveConfigDir()
		if result != expected {
			t.Errorf("resolveConfigDir() = %q, want %q", result, expected)
		}
	})
}

func TestLoadConfig(t *testing.T) {
	t.Run("no config file returns zero value", func(t *testing.T) {
		tmpDir := t.TempDir()
		original := os.Getenv("XDG_CONFIG_HOME")
		os.Setenv("XDG_CONFIG_HOME", tmpDir)
		defer os.Setenv("XDG_CONFIG_HOME", original)

		cfg, err := loadConfig()
		if err != nil {
			t.Fatalf("loadConfig() error = %v", err)
		}
		if cfg.Path != "" {
			t.Errorf("loadConfig() Path = %q, want empty", cfg.Path)
		}
	})

	t.Run("reads path from config file", func(t *testing.T) {
		tmpDir := t.TempDir()
		configDir := filepath.Join(tmpDir, "cody")
		os.MkdirAll(configDir, 0755)
		os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("path: legacy\n"), 0644)

		original := os.Getenv("XDG_CONFIG_HOME")
		os.Setenv("XDG_CONFIG_HOME", tmpDir)
		defer os.Setenv("XDG_CONFIG_HOME", original)

		cfg, err := loadConfig()
		if err != nil {
			t.Fatalf("loadConfig() error = %v", err)
		}
		if cfg.Path != "legacy" {
			t.Errorf("loadConfig() Path = %q, want %q", cfg.Path, "legacy")
		}
	})

	t.Run("returns error for invalid YAML", func(t *testing.T) {
		tmpDir := t.TempDir()
		configDir := filepath.Join(tmpDir, "cody")
		os.MkdirAll(configDir, 0755)
		os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(":\n  :\n    - ]["), 0644)

		original := os.Getenv("XDG_CONFIG_HOME")
		os.Setenv("XDG_CONFIG_HOME", tmpDir)
		defer os.Setenv("XDG_CONFIG_HOME", original)

		_, err := loadConfig()
		if err == nil {
			t.Error("loadConfig() expected error for invalid YAML, got nil")
		}
	})
}

func TestResolvePathTemplate(t *testing.T) {
	tests := []struct {
		name     string
		cfg      codyConfig
		expected string
	}{
		{
			name:     "empty path returns default",
			cfg:      codyConfig{},
			expected: pathShorthands["default"],
		},
		{
			name:     "explicit default",
			cfg:      codyConfig{Path: "default"},
			expected: pathShorthands["default"],
		},
		{
			name:     "legacy shorthand",
			cfg:      codyConfig{Path: "legacy"},
			expected: pathShorthands["legacy"],
		},
		{
			name:     "custom template passthrough",
			cfg:      codyConfig{Path: "{{.Home}}/src/{{.Host}}/{{.Repo}}"},
			expected: "{{.Home}}/src/{{.Host}}/{{.Repo}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolvePathTemplate(tt.cfg)
			if result != tt.expected {
				t.Errorf("resolvePathTemplate() = %q, want %q", result, tt.expected)
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

	// Point XDG_CONFIG_HOME to tmpDir so no config file is found
	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", originalXDG)

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

	// Point XDG_CONFIG_HOME to tmpDir so no config file is found
	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", originalXDG)

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
