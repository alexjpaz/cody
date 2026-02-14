package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var rootCmd = &cobra.Command{
	Use:   "cody",
	Short: "A versatile CLI tool with built-in functions",
	Long:  "A command-line tool for searching, expanding, and managing data files",
}

var searchCmd = &cobra.Command{
	Use:   "search [pattern]",
	Short: "Search for partial matches in the cody files",
	Long:  "Search for lines that contain the given pattern in the cody files",
	Args:  cobra.RangeArgs(0, 1),
	RunE:  runSearch,
}

var addCmd = &cobra.Command{
	Use:   "add [git_url] [code_path]",
	Short: "Add a new code entry",
	Long:  "Add a new code entry to the cody files. If code_path is omitted, defaults to 'uncategorized'.",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runAdd,
}

var pullCmd = &cobra.Command{
	Use:   "pull [filter]",
	Short: "Run git commands for pulling changes",
	Long:  "Run git commands for pulling changes from a remote repository",
	Args:  cobra.RangeArgs(0, 1),
	RunE:  runPull,
}

var openCmd = &cobra.Command{
	Use:   "open [filter]",
	Short: "Change directory to the repository",
	Long:  "Run git commands for opening changes from a remote repository",
	Args:  cobra.ExactArgs(1),
	RunE:  runOpen,
}

var rmCmd = &cobra.Command{
	Use:   "rm [url]",
	Short: "Remove a code entry",
	Long:  "Remove a code entry matching the given URL from the cody files",
	Args:  cobra.ExactArgs(1),
	RunE:  runRm,
}

var shellIntegrationCmd = &cobra.Command{
	Use:   "shell-integration",
	Short: "Print shell integration snippet",
	Long:  "Print shell function and alias for easy directory navigation. Add 'eval $(cody shell-integration)' to your shell profile.",
	Run:   runShellIntegration,
}

var rmForce bool

func init() {
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(openCmd)
	rootCmd.AddCommand(rmCmd)
	rootCmd.AddCommand(shellIntegrationCmd)

	rmCmd.Flags().BoolVarP(&rmForce, "force", "f", false, "Remove without confirmation")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runSearch(cmd *cobra.Command, args []string) error {
	var pattern string

	if len(args) > 0 {
		pattern = args[0]
	}

	pathTmpl, err := loadPathTemplate()
	if err != nil {
		return fmt.Errorf("failed to load path template: %w", err)
	}

	entries, err := collectAllCodyEntries()
	if err != nil {
		return fmt.Errorf("failed to collect cody entries %w", err)
	}

	for _, entry := range entries {
		if pattern == "" || strings.Contains(entry.url, pattern) {
			dest := resolveCodyWorkspaceUrl(entry, pathTmpl)
			if dest != "" {
				fmt.Println(dest)
			} else {
				fmt.Println(entry.url)
			}
		}
	}

	return nil
}

func runAdd(cmd *cobra.Command, args []string) error {
	gitURL := args[0]
	codePath := "uncategorized"
	if len(args) > 1 {
		codePath = args[1]
	}

	var filePath = resolveCodyConfig(codePath + ".code")

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(filePath), err)
	}

	// Check if entry already exists
	if _, err := os.Stat(filePath); err == nil {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read data file %s: %w", filePath, err)
		}

		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == gitURL {
				fmt.Printf("Entry already exists in %s\n", filePath)
				return nil
			}
		}
	}

	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open data file %s: %w", filePath, err)
	}
	defer file.Close()

	entry := fmt.Sprintf("%s\n", gitURL)
	if _, err := file.WriteString(entry); err != nil {
		return fmt.Errorf("failed to write to data file: %w", err)
	}

	fmt.Printf("Entry added successfully to %s\n", filePath)
	return nil
}

// ANSI color codes
const (
	colorReset = "\033[0m"
	colorGray  = "\033[90m"
	colorBlue  = "\033[34m"
	colorRed   = "\033[31m"
)

func runPull(cmd *cobra.Command, args []string) error {
	pathTmpl, err := loadPathTemplate()
	if err != nil {
		return fmt.Errorf("failed to load path template: %w", err)
	}

	entries, _ := collectAllCodyEntries()

	for _, entry := range entries {
		dest := resolveCodyWorkspaceUrl(entry, pathTmpl)

		if dest == "" {
			fmt.Printf("%sSkipped (unsupported URL format): %s%s\n", colorGray, entry.url, colorReset)
			continue
		}

		// Check if .git directory exists (fast local check)
		gitDir := filepath.Join(dest, ".git")
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			fmt.Printf("%sSkipped (already exists): %s%s\n", colorGray, entry.url, colorReset)
			continue
		}

		fmt.Printf("%sCloning: %s%s\n", colorBlue, entry.url, colorReset)
		if _, _, err := executeShellCommand("git", "clone", entry.url, dest); err != nil {
			fmt.Printf("%sClone failed: %s%s\n", colorRed, entry.url, colorReset)
		} else {
			fmt.Printf("%sClone success: %s to %s%s\n", colorBlue, entry.url, dest, colorReset)
		}
	}

	return nil
}

func runOpen(cmd *cobra.Command, args []string) error {
	filter := args[0]

	pathTmpl, err := loadPathTemplate()
	if err != nil {
		return fmt.Errorf("failed to load path template: %w", err)
	}

	entries, _ := collectAllCodyEntries()

	// find all matching entries
	var matches []codyEntry
	for _, entry := range entries {
		if strings.Contains(entry.url, filter) {
			matches = append(matches, entry)
		}
	}

	if len(matches) > 1 {
		urls := make([]string, len(matches))
		for i, m := range matches {
			urls[i] = m.url
		}
		return fmt.Errorf("multiple matches found: \n%s", strings.Join(urls, "\n"))
	} else if len(matches) == 0 {
		return fmt.Errorf("no matches found for filter '%s'", filter)
	}

	dest := resolveCodyWorkspaceUrl(matches[0], pathTmpl)
	fmt.Printf("cd %s\n", dest)

	return nil
}

func runShellIntegration(cmd *cobra.Command, args []string) {
	fmt.Print(`function cody_cd() {
    eval $(cody open $@)
}

alias c=cody_cd
`)
}

func runRm(cmd *cobra.Command, args []string) error {
	urlToRemove := args[0]

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	codeDir := filepath.Join(homeDir, ".code.d")
	found := false

	if _, statErr := os.Stat(codeDir); os.IsNotExist(statErr) {
		fmt.Printf("Entry not found: %s\n", urlToRemove)
		return nil
	}

	err = filepath.Walk(codeDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && filepath.Ext(path) == ".code" {
			content, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %w", path, err)
			}

			var newLines []string
			var matchedLines []string
			scanner := bufio.NewScanner(strings.NewReader(string(content)))
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line != "" {
					if strings.Contains(line, urlToRemove) {
						matchedLines = append(matchedLines, line)
						found = true
					} else {
						newLines = append(newLines, line)
					}
				}
			}

			for _, matchedLine := range matchedLines {
				if !rmForce {
					fmt.Printf("Remove %s from %s? [y/N] ", matchedLine, path)
					reader := bufio.NewReader(os.Stdin)
					response, _ := reader.ReadString('\n')
					response = strings.TrimSpace(strings.ToLower(response))
					if response != "y" && response != "yes" {
						fmt.Println("Skipped")
						newLines = append(newLines, matchedLine)
						continue
					}
				}
				fmt.Printf("Removed %s from %s\n", matchedLine, path)
			}

			if len(matchedLines) > 0 {
				newContent := strings.Join(newLines, "\n")
				if len(newLines) > 0 {
					newContent += "\n"
				}
				if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
					return fmt.Errorf("failed to write file %s: %w", path, err)
				}
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	if !found {
		fmt.Printf("Entry not found: %s\n", urlToRemove)
	}

	return nil
}

func resolveCodyConfig(path string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return homeDir + "/.code.d/" + path

}

func resolveCodyWorkspaceUrl(entry codyEntry, pathTmpl *template.Template) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	host, owner, repo, ok := parseGitURL(entry.url)
	if !ok {
		return ""
	}

	data := pathTemplateData{
		Home:     homeDir,
		Host:     host,
		Owner:    owner,
		Repo:     repo,
		CodePath: entry.codePath,
	}

	var buf strings.Builder
	if err := pathTmpl.Execute(&buf, data); err != nil {
		return ""
	}

	return filepath.Clean(buf.String())
}

type codyEntry struct {
	url      string
	codePath string // filename without .code extension, e.g. "personal", "uncategorized"
}

type codyConfig struct {
	Path string `yaml:"path"`
}

type pathTemplateData struct {
	Home     string
	Host     string
	Owner    string
	Repo     string
	CodePath string
}

var pathShorthands = map[string]string{
	"default": "{{.Home}}/code/{{.CodePath}}/{{.Host}}/{{.Owner}}/{{.Repo}}",
	"legacy":  "{{.Home}}/code/{{.Host}}/{{.Owner}}/{{.Repo}}",
}

func resolveConfigDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "cody")
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".config", "cody")
}

func loadConfig() (codyConfig, error) {
	configDir := resolveConfigDir()
	if configDir == "" {
		return codyConfig{}, nil
	}
	data, err := os.ReadFile(filepath.Join(configDir, "config.yaml"))
	if err != nil {
		if os.IsNotExist(err) {
			return codyConfig{}, nil
		}
		return codyConfig{}, err
	}
	var cfg codyConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return codyConfig{}, fmt.Errorf("failed to parse config.yaml: %w", err)
	}
	return cfg, nil
}

func resolvePathTemplate(cfg codyConfig) string {
	if cfg.Path == "" || cfg.Path == "default" {
		return pathShorthands["default"]
	}
	if tmpl, ok := pathShorthands[cfg.Path]; ok {
		return tmpl
	}
	return cfg.Path
}

func loadPathTemplate() (*template.Template, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}
	tmplStr := resolvePathTemplate(cfg)
	return template.New("path").Parse(tmplStr)
}

func parseGitURL(url string) (host, owner, repo string, ok bool) {
	if !strings.HasPrefix(url, "git@") {
		return "", "", "", false
	}
	parts := strings.SplitN(url, ":", 2)
	if len(parts) != 2 {
		return "", "", "", false
	}
	host = strings.TrimPrefix(parts[0], "git@")
	path := strings.TrimSuffix(parts[1], ".git")
	pathParts := strings.SplitN(path, "/", 2)
	if len(pathParts) != 2 {
		return "", "", "", false
	}
	owner = pathParts[0]
	repo = pathParts[1]
	return host, owner, repo, true
}

func collectAllCodyEntries() ([]codyEntry, error) {
	var entries []codyEntry

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	codeDir := filepath.Join(homeDir, ".code.d")

	if _, err := os.Stat(codeDir); os.IsNotExist(err) {
		return entries, nil
	}

	err = filepath.Walk(codeDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Only process .code files
		if !info.IsDir() && filepath.Ext(path) == ".code" {
			codePath := strings.TrimSuffix(filepath.Base(path), ".code")

			content, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %w", path, err)
			}

			// Split content into lines and add non-empty lines
			scanner := bufio.NewScanner(strings.NewReader(string(content)))
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line != "" {
					entries = append(entries, codyEntry{url: line, codePath: codePath})
				}
			}

			if err := scanner.Err(); err != nil {
				return fmt.Errorf("error scanning file %s: %w", path, err)
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return entries, nil
}

func executeShellCommand(command string, args ...string) (string, string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.Command(command, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = os.Stdin

	err := cmd.Run()

	return stdout.String(), stderr.String(), err
}
