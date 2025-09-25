package main

import (
	"bufio"
	"fmt"
	"os"
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
	Short: "Search for partial matches in the data file",
	Long:  "Search for lines that contain the given pattern in the data file",
	Args:  cobra.RangeArgs(0, 1),
	RunE:  runSearch,
}

var addCmd = &cobra.Command{
	Use:   "add [code_path] [git_url]",
	Short: "Add a new code snippet",
	Long:  "Add a new code snippet to the data file",
	Args:  cobra.ExactArgs(2),
	RunE:  runAdd,
}

func init() {
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(addCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runSearch(cmd *cobra.Command, args []string) error {
	var filePath = resolveCodyConfig("test.code")

	var pattern string

	if len(args) > 0 {
		pattern = args[0]
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open data file %s: %w", filePath, err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
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

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	return nil
}

func runAdd(cmd *cobra.Command, args []string) error {
	codePath := args[0]
	gitURL := args[1]

	var filePath = resolveCodyConfig(codePath + ".code")

	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open data file %s: %w", filePath, err)
	}
	defer file.Close()

	entry := fmt.Sprintf("%s\n", gitURL)
	if _, err := file.WriteString(entry); err != nil {
		return fmt.Errorf("failed to write to data file: %w", err)
	}

	fmt.Println("Entry added successfully.")
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
