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

	t.Run("unique match", func(t *testing.T) {
		err := runOpen(nil, []string{"repo1"})
		if err != nil {
			t.Errorf("runOpen() error = %v, wantErr false", err)
		}
	})

	// Note: "multiple matches" now triggers interactive select (reads stdin),
	// so we skip that in unit tests. It's tested via the interactive select test.

	t.Run("no matches", func(t *testing.T) {
		err := runOpen(nil, []string{"nonexistent"})
		if err == nil {
			t.Error("runOpen() expected error for no matches, got nil")
		}
	})
}

func TestParseCodyLine(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		wantURL     string
		wantAliases []string
		wantTags    []string
	}{
		{
			name:    "url only",
			line:    "git@github.com:user/repo.git",
			wantURL: "git@github.com:user/repo.git",
		},
		{
			name:        "url with alias",
			line:        "git@github.com:user/repo.git alias=r,repo",
			wantURL:     "git@github.com:user/repo.git",
			wantAliases: []string{"r", "repo"},
		},
		{
			name:     "url with tags",
			line:     "git@github.com:user/repo.git tags=tools,cli",
			wantURL:  "git@github.com:user/repo.git",
			wantTags: []string{"tools", "cli"},
		},
		{
			name:        "url with alias and tags",
			line:        "git@github.com:user/repo.git alias=cody,c tags=tools,cli",
			wantURL:     "git@github.com:user/repo.git",
			wantAliases: []string{"cody", "c"},
			wantTags:    []string{"tools", "cli"},
		},
		{
			name:    "empty line",
			line:    "",
			wantURL: "",
		},
		{
			name:    "unknown keys ignored",
			line:    "git@github.com:user/repo.git foo=bar alias=r",
			wantURL: "git@github.com:user/repo.git",
			wantAliases: []string{"r"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := parseCodyLine(tt.line)
			if entry.url != tt.wantURL {
				t.Errorf("parseCodyLine(%q).url = %q, want %q", tt.line, entry.url, tt.wantURL)
			}
			if !sliceEqual(entry.aliases, tt.wantAliases) {
				t.Errorf("parseCodyLine(%q).aliases = %v, want %v", tt.line, entry.aliases, tt.wantAliases)
			}
			if !sliceEqual(entry.tags, tt.wantTags) {
				t.Errorf("parseCodyLine(%q).tags = %v, want %v", tt.line, entry.tags, tt.wantTags)
			}
		})
	}
}

func TestFormatCodyLine(t *testing.T) {
	tests := []struct {
		name     string
		entry    codyEntry
		expected string
	}{
		{
			name:     "url only",
			entry:    codyEntry{url: "git@github.com:user/repo.git"},
			expected: "git@github.com:user/repo.git",
		},
		{
			name:     "with aliases",
			entry:    codyEntry{url: "git@github.com:user/repo.git", aliases: []string{"r", "repo"}},
			expected: "git@github.com:user/repo.git alias=r,repo",
		},
		{
			name:     "with tags",
			entry:    codyEntry{url: "git@github.com:user/repo.git", tags: []string{"tools"}},
			expected: "git@github.com:user/repo.git tags=tools",
		},
		{
			name:     "with both",
			entry:    codyEntry{url: "git@github.com:user/repo.git", aliases: []string{"c"}, tags: []string{"cli", "tools"}},
			expected: "git@github.com:user/repo.git alias=c tags=cli,tools",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatCodyLine(tt.entry)
			if result != tt.expected {
				t.Errorf("formatCodyLine() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestEntryMatches(t *testing.T) {
	entry := codyEntry{
		url:     "git@github.com:alexjpaz/cody.git",
		aliases: []string{"cody", "c"},
		tags:    []string{"tools", "cli"},
	}

	tests := []struct {
		name    string
		pattern string
		want    bool
	}{
		{"url substring match", "cody", true},
		{"url substring match host", "github.com", true},
		{"alias exact match", "c", true},
		{"alias exact match full", "cody", true},
		{"tag exact match", "tools", true},
		{"tag exact match cli", "cli", true},
		{"no match", "nonexistent", false},
		{"partial alias no match", "cod", true}, // "cod" is substring of url
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := entryMatches(entry, tt.pattern)
			if got != tt.want {
				t.Errorf("entryMatches(%q) = %v, want %v", tt.pattern, got, tt.want)
			}
		})
	}
}

func TestEntryMatchesAliasOnly(t *testing.T) {
	// Test that alias exact match works when not a URL substring
	entry := codyEntry{
		url:     "git@github.com:user/repo.git",
		aliases: []string{"myalias"},
	}

	if !entryMatches(entry, "myalias") {
		t.Error("entryMatches should match alias 'myalias'")
	}
	if entryMatches(entry, "myalia") {
		t.Error("entryMatches should not match partial alias 'myalia'")
	}
}

func TestFormatParseCodyLineRoundTrip(t *testing.T) {
	original := codyEntry{
		url:     "git@github.com:user/repo.git",
		aliases: []string{"r", "repo"},
		tags:    []string{"tools", "cli"},
	}

	line := formatCodyLine(original)
	parsed := parseCodyLine(line)

	if parsed.url != original.url {
		t.Errorf("round-trip url: got %q, want %q", parsed.url, original.url)
	}
	if !sliceEqual(parsed.aliases, original.aliases) {
		t.Errorf("round-trip aliases: got %v, want %v", parsed.aliases, original.aliases)
	}
	if !sliceEqual(parsed.tags, original.tags) {
		t.Errorf("round-trip tags: got %v, want %v", parsed.tags, original.tags)
	}
}

func TestCollectAllCodyEntriesWithMetadata(t *testing.T) {
	tmpDir := t.TempDir()

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	codeDir := filepath.Join(tmpDir, ".code.d")
	if err := os.MkdirAll(codeDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	content := "git@github.com:user/repo1.git alias=r1 tags=cli\ngit@github.com:user/repo2.git\n"
	if err := os.WriteFile(filepath.Join(codeDir, "test.code"), []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	entries, err := collectAllCodyEntries()
	if err != nil {
		t.Fatalf("collectAllCodyEntries() error = %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Find the entry with aliases
	var withAlias codyEntry
	for _, e := range entries {
		if e.url == "git@github.com:user/repo1.git" {
			withAlias = e
			break
		}
	}

	if !sliceEqual(withAlias.aliases, []string{"r1"}) {
		t.Errorf("expected aliases [r1], got %v", withAlias.aliases)
	}
	if !sliceEqual(withAlias.tags, []string{"cli"}) {
		t.Errorf("expected tags [cli], got %v", withAlias.tags)
	}
}

func TestReverseResolveCodePath(t *testing.T) {
	tests := []struct {
		name     string
		repoPath string
		homeDir  string
		tmplStr  string
		expected string
	}{
		{
			name:     "default template with codePath",
			repoPath: "/home/user/code/personal/github.com/user/repo",
			homeDir:  "/home/user",
			tmplStr:  "{{.Home}}/code/{{.CodePath}}/{{.Host}}/{{.Owner}}/{{.Repo}}",
			expected: "personal",
		},
		{
			name:     "legacy template (host as first segment)",
			repoPath: "/home/user/code/github.com/user/repo",
			homeDir:  "/home/user",
			tmplStr:  "{{.Home}}/code/{{.Host}}/{{.Owner}}/{{.Repo}}",
			expected: "uncategorized",
		},
		{
			name:     "outside code dir",
			repoPath: "/opt/repos/something",
			homeDir:  "/home/user",
			tmplStr:  "{{.Home}}/code/{{.CodePath}}/{{.Host}}/{{.Owner}}/{{.Repo}}",
			expected: "uncategorized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reverseResolveCodePath(tt.repoPath, tt.homeDir, tt.tmplStr)
			if got != tt.expected {
				t.Errorf("reverseResolveCodePath() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestRunMigrate(t *testing.T) {
	tmpDir := t.TempDir()

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", originalXDG)

	// Create .code.d with an entry filed under "personal"
	codeDir := filepath.Join(tmpDir, ".code.d")
	os.MkdirAll(codeDir, 0755)
	os.WriteFile(filepath.Join(codeDir, "personal.code"),
		[]byte("git@github.com:user/repo.git\n"), 0644)

	// Create a repo at the legacy path: ~/code/github.com/user/repo
	legacyPath := filepath.Join(tmpDir, "code", "github.com", "user", "repo", ".git")
	os.MkdirAll(legacyPath, 0755)
	os.WriteFile(filepath.Join(legacyPath, "HEAD"), []byte("ref: refs/heads/main\n"), 0644)

	t.Run("dry run shows what would move", func(t *testing.T) {
		migrateDry = true
		defer func() { migrateDry = false }()

		err := runMigrate(nil, nil)
		if err != nil {
			t.Fatalf("runMigrate() error = %v", err)
		}

		// Legacy path should still exist
		if _, err := os.Stat(legacyPath); err != nil {
			t.Error("legacy path should still exist after dry run")
		}

		// New path should NOT exist
		newPath := filepath.Join(tmpDir, "code", "personal", "github.com", "user", "repo")
		if _, err := os.Stat(newPath); err == nil {
			t.Error("new path should not exist after dry run")
		}
	})

	t.Run("actual migrate moves repo", func(t *testing.T) {
		migrateDry = false

		err := runMigrate(nil, nil)
		if err != nil {
			t.Fatalf("runMigrate() error = %v", err)
		}

		// New path should exist
		newGit := filepath.Join(tmpDir, "code", "personal", "github.com", "user", "repo", ".git")
		if _, err := os.Stat(newGit); err != nil {
			t.Errorf("expected repo at new path %s, got error: %v", newGit, err)
		}

		// Legacy path should be gone
		if _, err := os.Stat(legacyPath); err == nil {
			t.Error("legacy path should be removed after migrate")
		}
	})

	t.Run("nothing to migrate when already done", func(t *testing.T) {
		migrateDry = false

		err := runMigrate(nil, nil)
		if err != nil {
			t.Fatalf("runMigrate() error = %v", err)
		}
		// Should print "Nothing to migrate." and not error
	})
}

func TestCleanEmptyDirs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested empty dirs
	nested := filepath.Join(tmpDir, "a", "b", "c")
	os.MkdirAll(nested, 0755)

	// Create a dir with a file (should not be removed)
	withFile := filepath.Join(tmpDir, "keep", "data")
	os.MkdirAll(withFile, 0755)
	os.WriteFile(filepath.Join(withFile, "file.txt"), []byte("hi"), 0644)

	cleanEmptyDirs(tmpDir)

	// Empty nested dirs should be gone
	if _, err := os.Stat(filepath.Join(tmpDir, "a")); err == nil {
		t.Error("empty dir 'a' should have been removed")
	}

	// Dir with file should remain
	if _, err := os.Stat(filepath.Join(withFile, "file.txt")); err != nil {
		t.Error("dir with file should not be removed")
	}
}

func sliceEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
