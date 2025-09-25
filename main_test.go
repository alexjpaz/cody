package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootCommand(t *testing.T) {
	cmd := rootCmd
	if cmd.Use != "cody" {
		t.Errorf("Expected root command use to be 'cody', got %s", cmd.Use)
	}
}

func TestRunSearch(t *testing.T) {
	// Create a temporary directory to act as home directory
	tempDir, err := os.MkdirTemp("", "test_home")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create the .code.d directory
	codeDirPath := filepath.Join(tempDir, ".code.d")
	if err := os.MkdirAll(codeDirPath, 0755); err != nil {
		t.Fatalf("Failed to create .code.d directory: %v", err)
	}

	// Create test data file
	testFilePath := filepath.Join(codeDirPath, "test.code")
	testData := `line one
line two with pattern
line three
   line four with spaces   
line five with pattern again

line seven (line six was empty)`

	if err := os.WriteFile(testFilePath, []byte(testData), 0644); err != nil {
		t.Fatalf("Failed to write test data file: %v", err)
	}

	// Mock os.UserHomeDir by temporarily changing the HOME environment variable
	originalHome := os.Getenv("HOME")
	if originalHome == "" {
		// On Windows, try USERPROFILE
		originalHome = os.Getenv("USERPROFILE")
		os.Setenv("USERPROFILE", tempDir)
		defer os.Setenv("USERPROFILE", originalHome)
	} else {
		os.Setenv("HOME", tempDir)
		defer os.Setenv("HOME", originalHome)
	}

	tests := []struct {
		name           string
		args           []string
		expectedOutput string
		expectError    bool
	}{
		{
			name: "no pattern - should print all non-empty lines",
			args: []string{},
			expectedOutput: `line one
line two with pattern
line three
line four with spaces
line five with pattern again
line seven (line six was empty)
`,
			expectError: false,
		},
		{
			name: "with pattern - should print matching lines only",
			args: []string{"pattern"},
			expectedOutput: `line two with pattern
line five with pattern again
`,
			expectError: false,
		},
		{
			name:           "pattern not found - should print nothing",
			args:           []string{"nonexistent"},
			expectedOutput: "",
			expectError:    false,
		},
		{
			name: "pattern matching partial word",
			args: []string{"three"},
			expectedOutput: `line three
`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Create a mock cobra command
			cmd := &cobra.Command{}

			// Run the function
			err := runSearch(cmd, tt.args)

			// Restore stdout and get output
			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			io.Copy(&buf, r)
			output := buf.String()

			// Check for errors
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Check output
			if output != tt.expectedOutput {
				t.Errorf("Expected output:\n%q\nGot output:\n%q", tt.expectedOutput, output)
			}
		})
	}
}

func TestRunSearchFileErrors(t *testing.T) {
	// Test case where home directory cannot be determined
	// This is tricky to test directly since os.UserHomeDir() is hard to mock
	// So we'll test the file not found case instead

	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "test_home_no_file")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set home directory but don't create the file
	originalHome := os.Getenv("HOME")
	if originalHome == "" {
		originalHome = os.Getenv("USERPROFILE")
		os.Setenv("USERPROFILE", tempDir)
		defer os.Setenv("USERPROFILE", originalHome)
	} else {
		os.Setenv("HOME", tempDir)
		defer os.Setenv("HOME", originalHome)
	}

	cmd := &cobra.Command{}
	err = runSearch(cmd, []string{})

	if err == nil {
		t.Error("Expected error when file doesn't exist, but got none")
	}

	expectedErrorSubstring := "failed to open data file"
	if !strings.Contains(err.Error(), expectedErrorSubstring) {
		t.Errorf("Expected error to contain '%s', got: %v", expectedErrorSubstring, err)
	}
}

func TestRunSearchEmptyFile(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "test_home_empty")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create the .code.d directory and empty file
	codeDirPath := filepath.Join(tempDir, ".code.d")
	if err := os.MkdirAll(codeDirPath, 0755); err != nil {
		t.Fatalf("Failed to create .code.d directory: %v", err)
	}

	testFilePath := filepath.Join(codeDirPath, "test.code")
	if err := os.WriteFile(testFilePath, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to write empty test file: %v", err)
	}

	// Set up environment
	originalHome := os.Getenv("HOME")
	if originalHome == "" {
		originalHome = os.Getenv("USERPROFILE")
		os.Setenv("USERPROFILE", tempDir)
		defer os.Setenv("USERPROFILE", originalHome)
	} else {
		os.Setenv("HOME", tempDir)
		defer os.Setenv("HOME", originalHome)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := &cobra.Command{}
	err = runSearch(cmd, []string{})

	// Restore stdout and get output
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if err != nil {
		t.Errorf("Unexpected error with empty file: %v", err)
	}

	if output != "" {
		t.Errorf("Expected no output for empty file, got: %q", output)
	}
}

func TestResolveCodyWorkspaceUrl(t *testing.T) {
	homeDir, _ := os.UserHomeDir()

	t.Run("GitHub URL", func(t *testing.T) {
		targetUrl := homeDir + "/code/__DOMAIN__/__ORGANIZATION__/__REPOSITORY__"

		url := resolveCodyWorkspaceUrl("git@__DOMAIN__:__ORGANIZATION__/__REPOSITORY__.git")

		if url != targetUrl {
			t.Errorf("Expected %q, got: %q", targetUrl, url)
		}
	})

	t.Run("Invalid URL", func(t *testing.T) {
		url := resolveCodyWorkspaceUrl("invalid-url")

		if url != "" {
			t.Errorf("Expected empty string for invalid URL, got: %q", url)
		}
	})
}
