package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"

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

// ExecuteWithPromptsInternal executes a command and captures its output, handling prompts
func (e *Executor) ExecuteWithPromptsInternal(command string, args []string) (string, error) {
	logging.Debug("executeWithPrompts called", "command", command, "args", args)
	// Parse the command string to handle commands with arguments
	// This allows for "claude --dangerously-skip-permissions" to be processed correctly
	cmdParts := strings.Fields(command)
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

	var cmd *exec.Cmd

	if len(cmdParts) > 1 {
		// Command has built-in arguments
		cmd = exec.Command(cmdParts[0], append(cmdParts[1:], args...)...)
	} else {
		// Command is a single word
		cmd = exec.Command(command, args...)
	}

	// Create pipes for stdin, stdout, stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
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
					// Check if criteria are met
					shouldConfirm := true // Default to confirming if no criteria
					if len(pattern.Criteria) > 0 {
						shouldConfirm = false
						for _, criterion := range pattern.Criteria {
							if strings.Contains(outputBuffer.String(), criterion) {
								shouldConfirm = true
								break
							}
						}
					}

					if shouldConfirm {
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
	if err := cmd.Wait(); err != nil {
		return outputBuffer.String(), fmt.Errorf("command failed: %w", err)
	}

	return outputBuffer.String(), nil
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
	return e.ExecuteWithPromptsInternal(e.config.CLI.Command, args)
}

// ExecuteBasic runs the CLI tool and returns the output for display in TUI
func (e *Executor) ExecuteBasic(args []string) (string, error) {
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
