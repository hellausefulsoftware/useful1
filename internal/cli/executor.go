package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/hellausefulsoftware/useful1/internal/config"
	"github.com/hellausefulsoftware/useful1/internal/github"
)

// Executor handles execution of CLI commands and interaction with prompts
type Executor struct {
	config *config.Config
	github *github.Client
}

// NewExecutor creates a new command executor
func NewExecutor(cfg *config.Config) *Executor {
	return &Executor{
		config: cfg,
		github: github.NewClient(cfg.GitHub.Token),
	}
}

// formatErrorResponse formats an error into a JSON response for programmatic mode
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
	
	// Output JSON for programmatic mode
	response := map[string]interface{}{
		"status": "success",
		"issue_number": issueNum,
		"owner": owner,
		"repo": repo,
		"template": templateName,
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
	fmt.Printf("Generating response for issue #%d in %s/%s\n", issueNumber, owner, repo)

	// Create a temporary file with the issue text
	tmpFile, err := ioutil.TempFile("", "issue-*.txt")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write issue text to the file
	if _, err := tmpFile.WriteString(issueText); err != nil {
		return fmt.Errorf("failed to write to temporary file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
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
	metadataFile, err := ioutil.TempFile("", "metadata-*.json")
	if err != nil {
		return fmt.Errorf("failed to create metadata file: %w", err)
	}
	defer os.Remove(metadataFile.Name())

	// Write metadata to the file
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if _, err := metadataFile.Write(metadataBytes); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}
	if err := metadataFile.Close(); err != nil {
		return fmt.Errorf("failed to close metadata file: %w", err)
	}

	// Add metadata flag
	args = append(args, "--metadata", metadataFile.Name())

	// Execute the CLI tool
	output, err := e.executeWithPrompts(e.config.CLI.Command, args)
	if err != nil {
		return fmt.Errorf("CLI execution error: %w", err)
	}

	// Extract response from output
	response, err := e.extractResponse(output)
	if err != nil {
		return fmt.Errorf("failed to extract response: %w", err)
	}

	// Post response to the issue
	if err := e.github.RespondToIssue(owner, repo, issueNumber, response); err != nil {
		return fmt.Errorf("failed to post response: %w", err)
	}

	// Output JSON for programmatic mode
	responseObj := map[string]interface{}{
		"status": "success",
		"issue_number": issueNumber,
		"owner": owner,
		"repo": repo,
		"response_length": len(response),
		"timestamp": time.Now().Format(time.RFC3339),
		"url": fmt.Sprintf("https://github.com/%s/%s/issues/%d", owner, repo, issueNumber),
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

	// Output JSON for programmatic mode
	response := map[string]interface{}{
		"status": "success",
		"branch": branch,
		"base": base,
		"title": title,
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
		
		// Output JSON for programmatic mode with error details
		response := map[string]interface{}{
			"status": "error",
			"suite": testSuite,
			"error": err.Error(),
			"passed": passed,
			"failed": failed,
			"skipped": skipped,
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

	// Output JSON for programmatic mode
	response := map[string]interface{}{
		"status": "success",
		"suite": testSuite,
		"passed": passed,
		"failed": failed,
		"skipped": skipped,
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
	// Create the command
	command := exec.Command(cmd, args...)

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
						fmt.Fprintln(stdin, pattern.Response)
					} else {
						// Send rejection or cancel
						fmt.Fprintln(stdin, "n")
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
