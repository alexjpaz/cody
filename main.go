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
	Use:   "add [code_path] [git_url]",
	Short: "Add a new code entry",
	Long:  "Add a new code entry to the cody files",
	Args:  cobra.ExactArgs(2),
	RunE:  runAdd,
}

var pullCmd = &cobra.Command{
	Use:   "pull [filter]",
	Short: "Run git commands for pulling changes",
	Long:  "Run git commands for pulling changes from a remote repository",
	Args:  cobra.RangeArgs(0, 1),
	RunE:  runPull,
}

func init() {
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(pullCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
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
	codePath := args[0]
	gitURL := args[1]

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

func runPull(cmd *cobra.Command, args []string) error {
	// filter := args[0]
	urls, _ := collectAllCodyEntries()

	for _, url := range urls {
		dest := resolveCodyWorkspaceUrl(url)

		fmt.Println("Cloning ", url, " to ", dest)

		if _, _, err := executeShellCommand("git", "clone", url, dest); err != nil {
			fmt.Printf("failed to clone repository %s\n", url)
		}
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
