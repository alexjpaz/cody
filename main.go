package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
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

var rmForce bool

func init() {
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(openCmd)
	rootCmd.AddCommand(rmCmd)

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

	lines, err := collectAllCodyEntries()
	if err != nil {
		return fmt.Errorf("failed to collect cody entries %w", err)
	}

	if pattern != "" {
		for _, line := range lines {
			if strings.Contains(line, pattern) {
				fmt.Println(line)
			}
		}
	} else {
		for _, line := range lines {
			fmt.Println(line)
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
	// filter := args[0]
	urls, _ := collectAllCodyEntries()

	for _, url := range urls {
		dest := resolveCodyWorkspaceUrl(url)

		if dest == "" {
			fmt.Printf("%sSkipped (unsupported URL format): %s%s\n", colorGray, url, colorReset)
			continue
		}

		// Check if .git directory exists (fast local check)
		gitDir := filepath.Join(dest, ".git")
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			fmt.Printf("%sSkipped (already exists): %s%s\n", colorGray, url, colorReset)
			continue
		}

		fmt.Printf("%sCloning: %s%s\n", colorBlue, url, colorReset)
		if _, _, err := executeShellCommand("git", "clone", url, dest); err != nil {
			fmt.Printf("%sClone failed: %s%s\n", colorRed, url, colorReset)
		} else {
			fmt.Printf("%sClone success: %s to %s%s\n", colorBlue, url, dest, colorReset)
		}
	}

	return nil
}

func runOpen(cmd *cobra.Command, args []string) error {
	filter := args[0]
	urls, _ := collectAllCodyEntries()

	// find all matching urls
	var matches []string
	for _, url := range urls {
		if strings.Contains(url, filter) {
			matches = append(matches, url)
		}
	}

	if len(matches) > 1 {
		return fmt.Errorf("multiple matches found: \n%s", strings.Join(matches, "\n"))
	} else if len(matches) == 0 {
		return fmt.Errorf("no matches found for filter '%s'", filter)
	}

	dest := resolveCodyWorkspaceUrl(matches[0])
	fmt.Printf("cd %s\n", dest)

	return nil
}

func runRm(cmd *cobra.Command, args []string) error {
	urlToRemove := args[0]

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	codeDir := filepath.Join(homeDir, ".code.d")
	found := false

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

func resolveCodyWorkspaceUrl(url string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	if strings.HasPrefix(url, "git@") {
		parts := strings.SplitN(url, ":", 2)
		if len(parts) == 2 {
			domain := strings.TrimPrefix(parts[0], "git@")
			path := parts[1]
			target := homeDir + "/code/" + domain + "/" + strings.TrimSuffix(path, ".git")
			return target
		}
	}

	return ""
}

func collectAllCodyEntries() ([]string, error) {
	var entries []string

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	codeDir := filepath.Join(homeDir, ".code.d")

	err = filepath.Walk(codeDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Only process .code files
		if !info.IsDir() && filepath.Ext(path) == ".code" {
			content, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %w", path, err)
			}

			// Split content into lines and add non-empty lines
			scanner := bufio.NewScanner(strings.NewReader(string(content)))
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line != "" {
					entries = append(entries, line)
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
