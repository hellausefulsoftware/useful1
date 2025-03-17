package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/hellausefulsoftware/useful1/internal/config"
	"github.com/hellausefulsoftware/useful1/internal/logging"
)

// Executor handles execution of CLI commands and interaction with prompts
type Executor struct {
	config *config.Config
}

// NewExecutor creates a new command executor
func NewExecutor(cfg *config.Config) *Executor {
	return &Executor{
		config: cfg,
	}
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
