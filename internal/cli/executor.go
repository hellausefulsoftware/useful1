package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hellausefulsoftware/useful1/internal/config"
	"github.com/hellausefulsoftware/useful1/internal/logging"
)

// GithubClient defines the interface for GitHub operations used by Executor
// Only includes the minimal methods needed by executor.go
type GithubClient interface {
	RespondToIssue(owner, repo string, issueNumber int, comment string) error
}

// Executor handles execution of CLI commands and interaction with prompts
type Executor struct {
	config *config.Config
	github GithubClient
}

// NewExecutor creates a new command executor
func NewExecutor(cfg *config.Config) *Executor {
	return &Executor{
		config: cfg,
		github: nil, // Will be set by SetGithubClient
	}
}

// SetGithubClient sets the GitHub client
func (e *Executor) SetGithubClient(client GithubClient) {
	e.github = client
}

// GetGitHubClient returns the GitHub client
func (e *Executor) GetGitHubClient() GithubClient {
	return e.github
}

// formatErrorResponse formats an error into a JSON response
func (e *Executor) formatErrorResponse(err error, context map[string]interface{}) {
	// Create error response
	response := map[string]interface{}{
		"status":    "error",
		"message":   err.Error(),
		"timestamp": time.Now().Format(time.RFC3339),
	}

	// Add any additional context
	for k, v := range context {
		response[k] = v
	}

	// Marshal and print
	jsonResponse, jsonErr := json.Marshal(response)
	if jsonErr == nil {
		fmt.Println(string(jsonResponse))
	} else {
		// Fallback if JSON marshaling fails
		logging.Error("JSON marshaling failed", "error", jsonErr)
		fmt.Printf("{\"status\":\"error\",\"message\":\"%s\"}", err.Error())
	}
}

// RespondToIssue executes the CLI tool to respond to a GitHub issue
func (e *Executor) RespondToIssue(issueNumber string, templateName string) error {
	// Parse issue number
	issueNum, err := strconv.Atoi(issueNumber)
	if err != nil {
		return fmt.Errorf("invalid issue number: %s", issueNumber)
	}

	// Prepare command arguments
	args := append(
		e.config.CLI.Args,
		"respond",
		"--issue", issueNumber,
		"--template", templateName,
	)

	// Execute the CLI tool
	output, err := e.executeWithPrompts(e.config.CLI.Command, args)
	if err != nil {
		return err
	}

	/*
		// Parse the issue URL to get owner and repo
		owner := "default-owner" // This would typically be parsed from a URL or config
		repo := "default-repo"   // This would typically be parsed from a URL or config

		// Post a comment to the issue with the result
		err = e.github.RespondToIssue(
			owner,
			repo,
			issueNum,
			fmt.Sprintf("Automated response:\n\n```\n%s\n```", output),
		)

		if err != nil {
			return err
		}
	*/

	// Output JSON response
	response := map[string]interface{}{
		"status":          "success",
		"issue_number":    issueNum,
		"template":        templateName,
		"response_length": len(output),
	}

	jsonResponse, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("error formatting JSON response: %w", err)
	}

	fmt.Println(string(jsonResponse))
	return nil
}

// RespondToIssueText processes issue text and responds using the CLI tool
func (e *Executor) RespondToIssueText(owner, repo string, issueNumber int, issueText string) error {
	logging.Debug("===== STARTING RespondToIssueText =====")
	logging.Debug("GitHub Token present:", "token_exists", e.config.GitHub.Token != "", "token_length", len(e.config.GitHub.Token))
	logging.Info("Generating response for issue", "issue", issueNumber, "owner", owner, "repo", repo)

	logging.Debug("Creating temporary file for issue text")
	// Create a temporary file with the issue text
	tmpFile, err := os.CreateTemp("", "issue-*.txt")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	logging.Debug("Created temporary file", "path", tmpFile.Name())
	defer func() {
		if removeErr := os.Remove(tmpFile.Name()); removeErr != nil {
			logging.Warn("Failed to remove temporary file", "file", tmpFile.Name(), "error", removeErr)
		}
	}()

	// Write issue text to the file
	if _, writeErr := tmpFile.WriteString(issueText); writeErr != nil {
		return fmt.Errorf("failed to write to temporary file: %w", writeErr)
	}
	if closeErr := tmpFile.Close(); closeErr != nil {
		return fmt.Errorf("failed to close temporary file: %w", closeErr)
	}

	// Set up command arguments
	args := append(
		e.config.CLI.Args,
		"respond",
		"--issue-file", tmpFile.Name(),
		"--owner", owner,
		"--repo", repo,
		"--number", fmt.Sprintf("%d", issueNumber),
	)

	// Add budget flag
	args = append(args, "--budget", fmt.Sprintf("%.2f", e.config.Budgets.IssueResponse))

	// Create metadata for the CLI tool
	metadata := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"owner":     owner,
		"repo":      repo,
		"issue":     issueNumber,
		"url":       fmt.Sprintf("https://github.com/%s/%s/issues/%d", owner, repo, issueNumber),
	}

	// Create a temporary metadata file
	metadataFile, err := os.CreateTemp("", "metadata-*.json")
	if err != nil {
		return fmt.Errorf("failed to create metadata file: %w", err)
	}
	defer func() {
		if removeErr := os.Remove(metadataFile.Name()); removeErr != nil {
			logging.Warn("Failed to remove metadata file", "file", metadataFile.Name(), "error", removeErr)
		}
	}()

	// Write metadata to the file
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if _, writeErr := metadataFile.Write(metadataBytes); writeErr != nil {
		return fmt.Errorf("failed to write metadata: %w", writeErr)
	}
	if closeErr := metadataFile.Close(); closeErr != nil {
		return fmt.Errorf("failed to close metadata file: %w", closeErr)
	}

	// Add metadata flag
	args = append(args, "--metadata", metadataFile.Name())

	logging.Debug("Executing CLI tool", "command", e.config.CLI.Command, "args", args)

	// Check if the CLI tool exists
	cmdParts := strings.Fields(e.config.CLI.Command)
	if len(cmdParts) > 0 {
		_, err := exec.LookPath(cmdParts[0])
		if err != nil {
			logging.Error("CLI tool not found in PATH", "command", cmdParts[0], "error", err)
			return fmt.Errorf("CLI tool not found in PATH: %s: %w", cmdParts[0], err)
		} else {
			logging.Debug("CLI tool found in PATH", "command", cmdParts[0])
		}
	}

	// Execute the CLI tool with a timeout
	outputChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go func() {
		output, err := e.executeWithPrompts(e.config.CLI.Command, args)
		if err != nil {
			errChan <- err
			return
		}
		outputChan <- output
	}()

	// Wait for execution with a timeout
	var output string
	select {
	case output = <-outputChan:
		logging.Debug("CLI execution completed", "output_length", len(output))
	case err := <-errChan:
		logging.Error("CLI execution error", "error", err)
		return fmt.Errorf("CLI execution error: %w", err)
	case <-time.After(60 * time.Second):
		logging.Error("CLI execution timed out after 60 seconds")
		return fmt.Errorf("CLI execution timed out after 60 seconds")
	}

	logging.Debug("Extracting response from output")
	// Extract response from output
	response, err := e.extractResponse(output)
	if err != nil {
		logging.Error("Failed to extract response", "error", err)
		return fmt.Errorf("failed to extract response: %w", err)
	}
	logging.Debug("Response extracted successfully", "response_length", len(response))

	logging.Debug("Posting response to issue", "owner", owner, "repo", repo, "issue", issueNumber)
	// Post response to the issue
	if postErr := e.github.RespondToIssue(owner, repo, issueNumber, response); postErr != nil {
		logging.Error("Failed to post response", "error", postErr)
		return fmt.Errorf("failed to post response: %w", postErr)
	}
	logging.Debug("Response posted successfully")

	// Output JSON response
	responseObj := map[string]interface{}{
		"status":          "success",
		"issue_number":    issueNumber,
		"owner":           owner,
		"repo":            repo,
		"response_length": len(response),
		"timestamp":       time.Now().Format(time.RFC3339),
		"url":             fmt.Sprintf("https://github.com/%s/%s/issues/%d", owner, repo, issueNumber),
	}

	jsonResponse, err := json.Marshal(responseObj)
	if err != nil {
		return fmt.Errorf("error formatting JSON response: %w", err)
	}

	fmt.Println(string(jsonResponse))
	return nil
}

// CreatePullRequest executes the CLI tool to create a pull request
func (e *Executor) CreatePullRequest(branch, base, title string) error {
	// Prepare command arguments
	args := append(
		e.config.CLI.Args,
		"pr",
		"--branch", branch,
		"--base", base,
	)

	if title != "" {
		args = append(args, "--title", title)
	}

	// Execute the CLI tool
	output, err := e.executeWithPrompts(e.config.CLI.Command, args)
	if err != nil {
		return err
	}

	// Extract PR URL if available in output
	prUrl := ""
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "github.com") && strings.Contains(line, "/pull/") {
			prUrl = strings.TrimSpace(line)
			break
		}
	}

	// Output JSON response
	response := map[string]interface{}{
		"status":    "success",
		"branch":    branch,
		"base":      base,
		"title":     title,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	if prUrl != "" {
		response["pr_url"] = prUrl
	}

	jsonResponse, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("error formatting JSON response: %w", err)
	}

	fmt.Println(string(jsonResponse))
	return nil
}

// RunTests executes the CLI tool to run tests
func (e *Executor) RunTests(testSuite string) error {
	// Prepare command arguments
	args := append(e.config.CLI.Args, "test")

	if testSuite != "" {
		args = append(args, "--suite", testSuite)
	}

	// Execute the CLI tool
	output, err := e.executeWithPrompts(e.config.CLI.Command, args)
	if err != nil {
		// For test failures, we want to display the output but also return structured error
		// Extract test summary if possible
		failed := 0
		passed := 0
		skipped := 0
		for _, line := range strings.Split(output, "\n") {
			if strings.Contains(line, "failed") {
				failed++
			} else if strings.Contains(line, "passed") {
				passed++
			} else if strings.Contains(line, "skipped") {
				skipped++
			}
		}

		// Output JSON response with error details
		response := map[string]interface{}{
			"status":    "error",
			"suite":     testSuite,
			"error":     err.Error(),
			"passed":    passed,
			"failed":    failed,
			"skipped":   skipped,
			"timestamp": time.Now().Format(time.RFC3339),
		}

		jsonResponse, jsonErr := json.Marshal(response)
		if jsonErr == nil {
			fmt.Println(string(jsonResponse))
		}

		return err
	}

	// Extract test summary
	failed := 0
	passed := 0
	skipped := 0
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "failed") {
			failed++
		} else if strings.Contains(line, "passed") {
			passed++
		} else if strings.Contains(line, "skipped") {
			skipped++
		}
	}

	// Output JSON response
	response := map[string]interface{}{
		"status":    "success",
		"suite":     testSuite,
		"passed":    passed,
		"failed":    failed,
		"skipped":   skipped,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	jsonResponse, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("error formatting JSON response: %w", err)
	}

	fmt.Println(string(jsonResponse))
	return nil
}

// executeWithPrompts runs a command and handles interactive prompts
func (e *Executor) executeWithPrompts(cmd string, args []string) (string, error) {
	logging.Debug("executeWithPrompts called", "command", cmd, "args", args)
	// Parse the command string to handle commands with arguments
	// This allows for "claude --dangerously-skip-permissions" to be processed correctly
	cmdParts := strings.Fields(cmd)
	logging.Debug("Command parts", "parts", cmdParts)

	// Verify the command exists
	if len(cmdParts) > 0 {
		path, err := exec.LookPath(cmdParts[0])
		if err != nil {
			logging.Error("Command not found in PATH", "command", cmdParts[0], "error", err)
		} else {
			logging.Debug("Command found at path", "command", cmdParts[0], "path", path)
		}
	}

	var command *exec.Cmd

	if len(cmdParts) > 1 {
		// Command has built-in arguments
		command = exec.Command(cmdParts[0], append(cmdParts[1:], args...)...)
	} else {
		// Command is a single word
		command = exec.Command(cmd, args...)
	}

	// Create pipes for stdin, stdout, stderr
	stdin, err := command.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := command.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := command.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := command.Start(); err != nil {
		return "", fmt.Errorf("failed to start command: %w", err)
	}

	// Collect all output in a buffer
	outputBuffer := &strings.Builder{}

	// Create a merged reader for stdout and stderr
	readers := []io.Reader{stdout, stderr}
	multiReader := io.MultiReader(readers...)
	scanner := bufio.NewScanner(multiReader)

	// Process output and handle prompts
	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println(line) // Echo to our stdout
			outputBuffer.WriteString(line + "\n")

			// Check for prompt patterns
			for _, pattern := range e.config.Prompt.ConfirmationPatterns {
				if strings.Contains(line, pattern.Pattern) {
					// Check if criteria are met
					if e.checkCriteria(outputBuffer.String(), pattern.Criteria) {
						// Send confirmation
						if _, err := fmt.Fprintln(stdin, pattern.Response); err != nil {
							logging.Warn("Failed to send confirmation response", "error", err)
						}
					} else {
						// Send rejection or cancel
						if _, err := fmt.Fprintln(stdin, "n"); err != nil {
							logging.Warn("Failed to send rejection response", "error", err)
						}
					}
				}
			}
		}
	}()

	// Wait for command to complete
	if err := command.Wait(); err != nil {
		return outputBuffer.String(), fmt.Errorf("command failed: %w", err)
	}

	return outputBuffer.String(), nil
}

// extractResponse extracts the response from the CLI tool output
func (e *Executor) extractResponse(output string) (string, error) {
	// First check if the output contains a JSON response marker
	if strings.Contains(output, "RESPONSE_JSON:") {
		// Extract JSON response
		parts := strings.Split(output, "RESPONSE_JSON:")
		if len(parts) < 2 {
			return "", fmt.Errorf("malformed JSON response")
		}

		jsonStr := strings.TrimSpace(parts[1])
		var response struct {
			Content string `json:"content"`
		}

		if err := json.Unmarshal([]byte(jsonStr), &response); err != nil {
			return "", fmt.Errorf("invalid JSON response: %w", err)
		}

		return response.Content, nil
	}

	// If no JSON marker, check for plain text response marker
	if strings.Contains(output, "RESPONSE:") {
		parts := strings.Split(output, "RESPONSE:")
		if len(parts) < 2 {
			return "", fmt.Errorf("malformed response")
		}

		return strings.TrimSpace(parts[1]), nil
	}

	// If no markers found, return the full output with a note
	return fmt.Sprintf("Automated response:\n\n```\n%s\n```", output), nil
}

// checkCriteria checks if all criteria are present in the output
func (e *Executor) checkCriteria(output string, criteria []string) bool {
	for _, criterion := range criteria {
		if !strings.Contains(output, criterion) {
			return false
		}
	}
	return true
}

// Execute runs the configured CLI tool directly with interactive input/output
func (e *Executor) Execute(args []string) error {
	logging.Info("Executing CLI tool", "command", e.config.CLI.Command, "args", args)

	// Parse the command string to handle commands with arguments
	cmdParts := strings.Fields(e.config.CLI.Command)
	var command *exec.Cmd

	if len(cmdParts) > 1 {
		// Command has built-in arguments
		command = exec.Command(cmdParts[0], append(cmdParts[1:], args...)...)
	} else {
		// Command is a single word
		command = exec.Command(e.config.CLI.Command, args...)
	}

	// Connect command's stdin, stdout, and stderr directly to the user's
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	// Set process group to allow proper terminal handling
	command.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Run the command in interactive mode
	logging.Info("Starting interactive command",
		"command", e.config.CLI.Command,
		"args", args)

	err := command.Run()
	if err != nil {
		logging.Error("Command execution failed", "error", err)
		return fmt.Errorf("command execution failed: %w", err)
	}

	logging.Info("Command completed successfully")
	return nil
}

// ExecuteWithOutput runs the CLI tool and returns the output for display in TUI
func (e *Executor) ExecuteWithOutput(args []string) (string, error) {
	logging.Info("Executing CLI tool with output capture", "command", e.config.CLI.Command, "args", args)

	// Parse the command string to handle commands with arguments
	cmdParts := strings.Fields(e.config.CLI.Command)
	var command *exec.Cmd

	if len(cmdParts) > 1 {
		// Command has built-in arguments
		command = exec.Command(cmdParts[0], append(cmdParts[1:], args...)...)
	} else {
		// Command is a single word
		command = exec.Command(e.config.CLI.Command, args...)
	}

	// Capture both stdout and stderr in the output
	stdout, err := command.Output()
	if err != nil {
		// Try to get stderr output
		var errorOutput string
		if exitErr, ok := err.(*exec.ExitError); ok {
			errorOutput = string(exitErr.Stderr)
		}

		// Return both the error and any stderr content
		if errorOutput != "" {
			return fmt.Sprintf("ERROR: %s\n\nCommand stderr output:\n%s", err.Error(), errorOutput), err
		}
		return fmt.Sprintf("ERROR: %s", err.Error()), err
	}

	return string(stdout), nil
}
