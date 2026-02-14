package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

var aliasCmd = &cobra.Command{
	Use:   "alias [name] [url-pattern]",
	Short: "Manage aliases for code entries",
	Long:  "Add, list, or remove aliases for code entries. Use --list to list all, --rm to remove.",
	Args:  cobra.RangeArgs(0, 2),
	RunE:  runAlias,
}

var tagCmd = &cobra.Command{
	Use:   "tag [tag] [url-pattern]",
	Short: "Manage tags for code entries",
	Long:  "Add, list, or remove tags for code entries. Use --list to list all, --rm to remove.",
	Args:  cobra.RangeArgs(0, 2),
	RunE:  runTag,
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Bidirectional sync between code.d index and code workspace",
	Long:  "Discover repos in ~/code not tracked in .code.d and clone repos in .code.d not yet in ~/code.",
	RunE:  runSync,
}

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate repos from legacy paths to new namespaced paths",
	Long:  "Move repos from ~/code/{host}/{owner}/{repo} to ~/code/{codePath}/{host}/{owner}/{repo}.",
	RunE:  runMigrate,
}

var (
	aliasList  bool
	aliasRm    string
	tagList    bool
	tagRm      string
	syncDry    bool
	migrateDry bool
)

var (
	rmForce   bool
	addAlias  string
	addTags   string
)

func init() {
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(openCmd)
	rootCmd.AddCommand(rmCmd)
	rootCmd.AddCommand(shellIntegrationCmd)
	rootCmd.AddCommand(aliasCmd)
	rootCmd.AddCommand(tagCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(migrateCmd)

	rmCmd.Flags().BoolVarP(&rmForce, "force", "f", false, "Remove without confirmation")
	addCmd.Flags().StringVar(&addAlias, "alias", "", "Comma-separated aliases for the entry")
	addCmd.Flags().StringVar(&addTags, "tags", "", "Comma-separated tags for the entry")
	aliasCmd.Flags().BoolVar(&aliasList, "list", false, "List all entries with aliases")
	aliasCmd.Flags().StringVar(&aliasRm, "rm", "", "Remove an alias by name")
	tagCmd.Flags().BoolVar(&tagList, "list", false, "List all tags")
	tagCmd.Flags().StringVar(&tagRm, "rm", "", "Remove a tag")
	syncCmd.Flags().BoolVar(&syncDry, "dry-run", false, "Print what would be done without making changes")
	migrateCmd.Flags().BoolVar(&migrateDry, "dry-run", false, "Print what would be moved without making changes")
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
		if pattern == "" || entryMatches(entry, pattern) {
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

	// Check if entry already exists (compare by URL only)
	if _, err := os.Stat(filePath); err == nil {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read data file %s: %w", filePath, err)
		}

		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parsed := parseCodyLine(line)
			if parsed.url == gitURL {
				fmt.Printf("Entry already exists in %s\n", filePath)
				return nil
			}
		}
	}

	newEntry := codyEntry{url: gitURL}
	if addAlias != "" {
		newEntry.aliases = strings.Split(addAlias, ",")
	}
	if addTags != "" {
		newEntry.tags = strings.Split(addTags, ",")
	}

	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open data file %s: %w", filePath, err)
	}
	defer file.Close()

	line := formatCodyLine(newEntry) + "\n"
	if _, err := file.WriteString(line); err != nil {
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
		if entryMatches(entry, filter) {
			matches = append(matches, entry)
		}
	}

	if len(matches) == 0 {
		return fmt.Errorf("no matches found for filter '%s'", filter)
	}

	selected := matches[0]
	if len(matches) > 1 {
		// Interactive select: prompts go to stderr, only cd goes to stdout
		fmt.Fprintf(os.Stderr, "Multiple matches found:\n")
		for i, m := range matches {
			dest := resolveCodyWorkspaceUrl(m, pathTmpl)
			if dest == "" {
				dest = m.url
			}
			fmt.Fprintf(os.Stderr, "  %d) %s\n", i+1, dest)
		}
		fmt.Fprintf(os.Stderr, "Select [1-%d]: ", len(matches))

		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		choice, err := strconv.Atoi(input)
		if err != nil || choice < 1 || choice > len(matches) {
			return fmt.Errorf("invalid selection: %s", input)
		}
		selected = matches[choice-1]
	}

	dest := resolveCodyWorkspaceUrl(selected, pathTmpl)
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

// updateEntryInFile finds an entry by URL pattern, applies a transform, and rewrites the file.
func updateEntryInFile(urlPattern string, transform func(*codyEntry) bool) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	codeDir := filepath.Join(homeDir, ".code.d")
	if _, err := os.Stat(codeDir); os.IsNotExist(err) {
		return fmt.Errorf("no .code.d directory found")
	}

	found := false
	err = filepath.Walk(codeDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || filepath.Ext(path) != ".code" {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		var newLines []string
		changed := false
		scanner := bufio.NewScanner(strings.NewReader(string(content)))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			entry := parseCodyLine(line)
			if strings.Contains(entry.url, urlPattern) {
				if transform(&entry) {
					changed = true
					found = true
				}
				newLines = append(newLines, formatCodyLine(entry))
			} else {
				newLines = append(newLines, line)
			}
		}

		if changed {
			newContent := strings.Join(newLines, "\n")
			if len(newLines) > 0 {
				newContent += "\n"
			}
			return os.WriteFile(path, []byte(newContent), 0644)
		}
		return nil
	})

	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("no entry found matching '%s'", urlPattern)
	}
	return nil
}

func runAlias(cmd *cobra.Command, args []string) error {
	if aliasList {
		entries, err := collectAllCodyEntries()
		if err != nil {
			return err
		}
		for _, e := range entries {
			if len(e.aliases) > 0 {
				fmt.Printf("%s  alias=%s\n", e.url, strings.Join(e.aliases, ","))
			}
		}
		return nil
	}

	if aliasRm != "" {
		// Find entry with this alias and remove it
		entries, err := collectAllCodyEntries()
		if err != nil {
			return err
		}
		for _, e := range entries {
			for _, a := range e.aliases {
				if a == aliasRm {
					return updateEntryInFile(e.url, func(entry *codyEntry) bool {
						var filtered []string
						for _, ea := range entry.aliases {
							if ea != aliasRm {
								filtered = append(filtered, ea)
							}
						}
						entry.aliases = filtered
						return true
					})
				}
			}
		}
		return fmt.Errorf("alias '%s' not found", aliasRm)
	}

	if len(args) < 2 {
		return fmt.Errorf("usage: cody alias <name> <url-pattern>")
	}

	name, pattern := args[0], args[1]
	return updateEntryInFile(pattern, func(entry *codyEntry) bool {
		for _, a := range entry.aliases {
			if a == name {
				return false // already exists
			}
		}
		entry.aliases = append(entry.aliases, name)
		return true
	})
}

func runTag(cmd *cobra.Command, args []string) error {
	if tagList {
		entries, err := collectAllCodyEntries()
		if err != nil {
			return err
		}
		seen := map[string]bool{}
		for _, e := range entries {
			for _, t := range e.tags {
				if !seen[t] {
					fmt.Println(t)
					seen[t] = true
				}
			}
		}
		return nil
	}

	if tagRm != "" {
		// If a pattern is given, remove tag from that entry; otherwise remove from all
		var pattern string
		if len(args) > 0 {
			pattern = args[0]
		}

		entries, err := collectAllCodyEntries()
		if err != nil {
			return err
		}

		found := false
		for _, e := range entries {
			for _, t := range e.tags {
				if t == tagRm {
					if pattern != "" && !strings.Contains(e.url, pattern) {
						continue
					}
					err := updateEntryInFile(e.url, func(entry *codyEntry) bool {
						var filtered []string
						for _, et := range entry.tags {
							if et != tagRm {
								filtered = append(filtered, et)
							}
						}
						entry.tags = filtered
						return true
					})
					if err != nil {
						return err
					}
					found = true
				}
			}
		}
		if !found {
			return fmt.Errorf("tag '%s' not found", tagRm)
		}
		return nil
	}

	if len(args) < 2 {
		return fmt.Errorf("usage: cody tag <tag> <url-pattern>")
	}

	tag, pattern := args[0], args[1]
	return updateEntryInFile(pattern, func(entry *codyEntry) bool {
		for _, t := range entry.tags {
			if t == tag {
				return false
			}
		}
		entry.tags = append(entry.tags, tag)
		return true
	})
}

type discoveredRepo struct {
	path      string
	remoteURL string
}

func discoverRepos(root string) ([]discoveredRepo, error) {
	var repos []discoveredRepo

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if !info.IsDir() {
			return nil
		}
		// Check for .git directory
		gitDir := filepath.Join(path, ".git")
		if gi, err := os.Stat(gitDir); err == nil && gi.IsDir() {
			// Get remote URL
			cmd := exec.Command("git", "-C", path, "remote", "get-url", "origin")
			out, err := cmd.Output()
			if err == nil {
				url := strings.TrimSpace(string(out))
				if url != "" {
					repos = append(repos, discoveredRepo{path: path, remoteURL: url})
				}
			}
			return filepath.SkipDir // don't descend into git repos
		}
		return nil
	})
	return repos, err
}

func reverseResolveCodePath(repoPath, homeDir, tmplStr string) string {
	// For default template, strip ~/code/ prefix and use first segment
	defaultPrefix := filepath.Join(homeDir, "code") + "/"
	if strings.HasPrefix(repoPath, defaultPrefix) {
		rel := strings.TrimPrefix(repoPath, defaultPrefix)
		parts := strings.SplitN(rel, string(filepath.Separator), 2)
		if len(parts) >= 1 {
			// Check if the first segment looks like a host (contains a dot)
			// If so, this is likely a legacy layout without codePath
			if strings.Contains(parts[0], ".") {
				return "uncategorized"
			}
			return parts[0]
		}
	}
	return "uncategorized"
}

func runSync(cmd *cobra.Command, args []string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	tmplStr := resolvePathTemplate(cfg)

	pathTmpl, err := template.New("path").Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("failed to parse path template: %w", err)
	}

	// Step 1: Discover untracked repos
	codeRoot := filepath.Join(homeDir, "code")
	if _, err := os.Stat(codeRoot); os.IsNotExist(err) {
		fmt.Println("No ~/code directory found, skipping discovery")
	} else {
		existingEntries, err := collectAllCodyEntries()
		if err != nil {
			return fmt.Errorf("failed to collect entries: %w", err)
		}

		knownURLs := map[string]bool{}
		for _, e := range existingEntries {
			knownURLs[e.url] = true
		}

		discovered, err := discoverRepos(codeRoot)
		if err != nil {
			return fmt.Errorf("failed to discover repos: %w", err)
		}

		for _, repo := range discovered {
			if knownURLs[repo.remoteURL] {
				continue
			}

			codePath := reverseResolveCodePath(repo.path, homeDir, tmplStr)

			if syncDry {
				fmt.Printf("%s[dry-run] Would add: %s -> %s.code%s\n", colorBlue, repo.remoteURL, codePath, colorReset)
			} else {
				fmt.Printf("%sDiscovered: %s -> %s.code%s\n", colorBlue, repo.remoteURL, codePath, colorReset)
				filePath := resolveCodyConfig(codePath + ".code")
				if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
					return fmt.Errorf("failed to create directory: %w", err)
				}
				file, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
				if err != nil {
					return fmt.Errorf("failed to open %s: %w", filePath, err)
				}
				fmt.Fprintf(file, "%s\n", repo.remoteURL)
				file.Close()
			}
			knownURLs[repo.remoteURL] = true
		}
	}

	// Step 2: Clone missing repos (reuse pull logic)
	entries, err := collectAllCodyEntries()
	if err != nil {
		return fmt.Errorf("failed to collect entries: %w", err)
	}

	for _, entry := range entries {
		dest := resolveCodyWorkspaceUrl(entry, pathTmpl)
		if dest == "" {
			continue
		}

		gitDir := filepath.Join(dest, ".git")
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			continue
		}

		if syncDry {
			fmt.Printf("%s[dry-run] Would clone: %s -> %s%s\n", colorBlue, entry.url, dest, colorReset)
		} else {
			fmt.Printf("%sCloning: %s%s\n", colorBlue, entry.url, colorReset)
			if _, _, err := executeShellCommand("git", "clone", entry.url, dest); err != nil {
				fmt.Printf("%sClone failed: %s%s\n", colorRed, entry.url, colorReset)
			} else {
				fmt.Printf("%sClone success: %s to %s%s\n", colorBlue, entry.url, dest, colorReset)
			}
		}
	}

	return nil
}

func runMigrate(cmd *cobra.Command, args []string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	legacyTmpl, err := template.New("legacy").Parse(pathShorthands["legacy"])
	if err != nil {
		return fmt.Errorf("failed to parse legacy template: %w", err)
	}

	defaultTmpl, err := template.New("default").Parse(pathShorthands["default"])
	if err != nil {
		return fmt.Errorf("failed to parse default template: %w", err)
	}

	entries, err := collectAllCodyEntries()
	if err != nil {
		return fmt.Errorf("failed to collect entries: %w", err)
	}

	moved := 0
	for _, entry := range entries {
		legacyPath := resolveCodyWorkspaceUrl(entry, legacyTmpl)
		newPath := resolveCodyWorkspaceUrl(entry, defaultTmpl)

		if legacyPath == "" || newPath == "" {
			continue
		}

		// Skip if legacy and new resolve to the same path
		if legacyPath == newPath {
			continue
		}

		// Check if repo exists at legacy path
		legacyGit := filepath.Join(legacyPath, ".git")
		if _, err := os.Stat(legacyGit); err != nil {
			continue
		}

		// Check if repo already exists at new path
		newGit := filepath.Join(newPath, ".git")
		if _, err := os.Stat(newGit); err == nil {
			fmt.Printf("%sSkipped (already exists at new path): %s%s\n", colorGray, newPath, colorReset)
			continue
		}

		if migrateDry {
			fmt.Printf("[dry-run] %s -> %s\n", legacyPath, newPath)
		} else {
			if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
				fmt.Printf("%sFailed to create directory for %s: %v%s\n", colorRed, newPath, err, colorReset)
				continue
			}
			if err := os.Rename(legacyPath, newPath); err != nil {
				fmt.Printf("%sFailed to move %s -> %s: %v%s\n", colorRed, legacyPath, newPath, err, colorReset)
				continue
			}
			fmt.Printf("%sMoved: %s -> %s%s\n", colorBlue, legacyPath, newPath, colorReset)
		}
		moved++
	}

	// Clean up empty directories left behind in ~/code
	if !migrateDry && moved > 0 {
		codeRoot := filepath.Join(homeDir, "code")
		cleanEmptyDirs(codeRoot)
	}

	if moved == 0 {
		fmt.Println("Nothing to migrate.")
	} else if migrateDry {
		fmt.Printf("\n%d repo(s) would be moved. Run without --dry-run to apply.\n", moved)
	} else {
		fmt.Printf("\n%d repo(s) migrated.\n", moved)
	}

	return nil
}

// cleanEmptyDirs removes empty directories bottom-up under root.
func cleanEmptyDirs(root string) {
	// Walk in reverse depth order by collecting dirs first
	var dirs []string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && path != root {
			dirs = append(dirs, path)
		}
		return nil
	})
	// Remove deepest first
	for i := len(dirs) - 1; i >= 0; i-- {
		os.Remove(dirs[i]) // only succeeds if empty
	}
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
					entry := parseCodyLine(line)
					if strings.Contains(entry.url, urlToRemove) {
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
	aliases  []string
	tags     []string
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
					entry := parseCodyLine(line)
					entry.codePath = codePath
					entries = append(entries, entry)
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

// parseCodyLine parses an extended .code line: "url [alias=x,y] [tags=a,b]"
func parseCodyLine(line string) codyEntry {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return codyEntry{}
	}

	entry := codyEntry{url: fields[0]}

	for _, field := range fields[1:] {
		parts := strings.SplitN(field, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], parts[1]
		values := strings.Split(val, ",")
		switch key {
		case "alias":
			entry.aliases = values
		case "tags":
			entry.tags = values
		}
	}

	return entry
}

// formatCodyLine serializes a codyEntry back to the .code line format
func formatCodyLine(entry codyEntry) string {
	parts := []string{entry.url}
	if len(entry.aliases) > 0 {
		parts = append(parts, "alias="+strings.Join(entry.aliases, ","))
	}
	if len(entry.tags) > 0 {
		parts = append(parts, "tags="+strings.Join(entry.tags, ","))
	}
	return strings.Join(parts, " ")
}

// entryMatches returns true if the pattern matches the entry's URL (substring),
// aliases (exact), or tags (exact).
func entryMatches(entry codyEntry, pattern string) bool {
	if strings.Contains(entry.url, pattern) {
		return true
	}
	for _, alias := range entry.aliases {
		if alias == pattern {
			return true
		}
	}
	for _, tag := range entry.tags {
		if tag == pattern {
			return true
		}
	}
	return false
}
