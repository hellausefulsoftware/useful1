package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"time"

	expect "github.com/google/goexpect"
	"github.com/hellausefulsoftware/useful1/internal/config"
	"github.com/hellausefulsoftware/useful1/internal/logging"
)

// Executor handles execution of CLI commands and interaction with prompts.
type Executor struct {
	config *config.Config
}

// NewExecutor creates a new command executor.
func NewExecutor(cfg *config.Config) *Executor {
	return &Executor{
		config: cfg,
	}
}

// Execute runs the command in interactive mode, handling the CLI directly.
func (e *Executor) Execute(args []string) error {
	logging.Info("Executing CLI tool", "command", e.config.CLI.Command, "args", args, "timeout", e.config.CLI.Timeout)

	// Create a context with timeout.
	timeout := e.config.CLI.Timeout
	if timeout <= 0 {
		timeout = 600 // Default 10 minutes (600 seconds) if not set or invalid.
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	// Parse command and build exec.Command.
	cmdParts := strings.Fields(e.config.CLI.Command)
	var command *exec.Cmd
	if len(cmdParts) > 1 {
		command = exec.CommandContext(ctx, cmdParts[0], append(cmdParts[1:], args...)...)
	} else {
		command = exec.CommandContext(ctx, e.config.CLI.Command, args...)
	}

	// Connect standard IO.
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	// Set process group for terminal handling.
	command.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	logging.Info("Starting interactive command", "command", e.config.CLI.Command, "args", args, "timeout", timeout)
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			logging.Error("Command execution timed out", "timeout", timeout)
			return fmt.Errorf("command execution timed out after %d seconds", timeout)
		}
		logging.Error("Command execution failed", "error", err)
		return fmt.Errorf("command execution failed: %w", err)
	}
	logging.Info("Command completed successfully")
	return nil
}

// ExecuteWithOutput runs the CLI tool and returns output, handling prompts with expect.
// The function first processes output until both the prompt content and a confirmation (enter)
// have been sent. It then enters a monitoring phase that tracks the pattern "esc to interrupt"
// with ANSI escape codes, and exits when the pattern disappears for a specified time.
func (e *Executor) ExecuteWithOutput(args []string, promptContent string) (string, error) {
	logging.Info("Executing CLI tool with output capture", "command", e.config.CLI.Command, "args", args, "prompt_provided", promptContent != "")

	// Build command arguments.
	cmdParts := strings.Fields(e.config.CLI.Command)
	var cmdArgs []string
	if len(cmdParts) > 1 {
		cmdArgs = append(cmdParts[1:], args...)
	} else {
		cmdArgs = args
	}

	// Set timeout.
	timeout := e.config.CLI.Timeout
	if timeout <= 0 {
		timeout = 600 // Default 10 minutes (600 seconds).
	}
	timeoutDuration := time.Duration(timeout) * time.Second

	logging.Info("Using expect to handle interactive prompts", "command", cmdParts[0], "args", cmdArgs, "timeout", timeoutDuration)

	// Set up environment for better terminal compatibility
	cmd := exec.Command(cmdParts[0], cmdArgs...)
	env := os.Environ()
	customEnv := []string{}

	for _, e := range env {
		if !strings.HasPrefix(e, "TERM=") {
			customEnv = append(customEnv, e)
		}
	}

	// Use xterm-256color for better compatibility with TUI apps
	customEnv = append(customEnv, "TERM=xterm-256color", "LINES=24", "COLUMNS=80")
	cmd.Env = customEnv

	// Spawn command with expect.
	exp, _, err := expect.SpawnWithArgs(cmd.Args,
		timeoutDuration,
		expect.Verbose(false),
		expect.PartialMatch(true),
		expect.CheckDuration(100*time.Millisecond))
	if err != nil {
		logging.Error("Failed to spawn command", "error", err)
		return "", fmt.Errorf("failed to spawn command: %w", err)
	}
	defer func() {
		if err := exp.Close(); err != nil {
			logging.Error("Failed to close expect", "error", err)
		}
	}()

	// Flags to track if we've sent the prompt and the enter key.
	promptSent := false
	enterSent := false

	// Output buffer.
	var output strings.Builder

	// Regex patterns.
	cursorPattern := regexp.MustCompile(`>`)
	humanPattern := regexp.MustCompile(`Human:`)
	promptPattern := regexp.MustCompile(`Prompt:`)
	ynPattern := regexp.MustCompile(`\[y/n\]|\(y/N\)|y/n`)
	welcomePattern := regexp.MustCompile(`Welcome to Claude Code`)
	boxPattern := regexp.MustCompile(`╭|╮|╯|╰`)
	tryPattern := regexp.MustCompile(`Try "how do I`)
	claudePromptPattern := regexp.MustCompile(`>|Human:|Press Ctrl-C`)
	inputBoxPattern := regexp.MustCompile(`\[Pasted text`)

	emptyOutputCount := 0

	// Normal output processing loop.
	for {
		// Exit this loop once both prompt and enter have been sent.
		if promptSent && enterSent {
			break
		}

		result, _, err := exp.Expect(regexp.MustCompile(`.+`), 5*time.Second)
		if result == "" {
			emptyOutputCount++
			logging.Info("Received empty output", "empty_count", emptyOutputCount)
			// If multiple empty outputs and prompt not yet sent, send it.
			if emptyOutputCount > 5 && !promptSent && promptContent != "" {
				logging.Info("Multiple empty outputs detected, sending prompt content")
				promptSent = true
				if sendErr := exp.Send(promptContent + "\n"); sendErr != nil {
					logging.Error("Failed to send prompt content", "error", sendErr)
					return output.String(), sendErr
				}

				// Small delay before sending enter
				time.Sleep(300 * time.Millisecond)
				if sendErr := exp.Send("\r"); sendErr != nil {
					logging.Error("Failed to send enter key", "error", sendErr)
				}
			} else if emptyOutputCount > 10 {
				logging.Info("Assuming process is running despite empty outputs")
				promptSent = true
				if sendErr := exp.Send("\r"); sendErr != nil {
					logging.Error("Failed to send newline", "error", sendErr)
				}
				return "Command appears to be running but not producing detectable output", nil
			}
			continue
		}
		emptyOutputCount = 0

		// Handle EOF.
		if err != nil {
			if err == io.EOF {
				logging.Info("Command completed with EOF")
				output.WriteString(result)
				break
			}
			output.WriteString(result)
			if err.Error() == "expect: timeout" {
				logging.Info("Expect timed out waiting for more output", "timeout_seconds")
				if emptyOutputCount > 3 && !promptSent && promptContent != "" {
					logging.Info("Timeout with multiple empty outputs, sending prompt content")
					promptSent = true
					if sendErr := exp.Send(promptContent + "\n"); sendErr != nil {
						logging.Error("Failed to send prompt content", "error", sendErr)
						return output.String(), sendErr
					}

					// Small delay before sending enter
					time.Sleep(300 * time.Millisecond)
					if sendErr := exp.Send("\r"); sendErr != nil {
						logging.Error("Failed to send enter key", "error", sendErr)
					}
				}
				continue
			}
			logging.Error("Error waiting for command output", "error", err)
			return output.String(), err
		}

		// Process output based on recognized patterns.
		switch {
		// Handle input box prompt.
		case inputBoxPattern.MatchString(result):
			if !promptSent && promptContent != "" {
				logging.Info("Detected pasted text box, sending prompt content")
				if err := exp.Send(promptContent + "\n"); err != nil {
					logging.Error("Failed to send prompt content", "error", err)
					return output.String(), err
				}

				// Small delay before sending enter
				time.Sleep(300 * time.Millisecond)
				if err := exp.Send("\r"); err != nil {
					logging.Error("Failed to send enter key", "error", err)
				}
				promptSent = true
			} else {
				logging.Info("Input box prompt detected again; sending newline")
				if err := exp.Send("\r"); err != nil {
					logging.Error("Failed to send newline", "error", err)
				}
				time.Sleep(2 * time.Second)
				enterSent = true
			}
		// Handle yes/no prompt.
		case ynPattern.MatchString(result):
			logging.Info("Detected yes/no prompt, answering yes")
			if err := exp.Send("y"); err != nil {
				logging.Error("Failed to send yes response", "error", err)
				return output.String(), err
			}

			// Small delay before sending enter
			time.Sleep(100 * time.Millisecond)
			if err := exp.Send("\r"); err != nil {
				logging.Error("Failed to send enter key", "error", err)
				return output.String(), err
			}
		// Handle interactive screen prompts.
		case welcomePattern.MatchString(result) ||
			boxPattern.MatchString(result) ||
			tryPattern.MatchString(result) ||
			claudePromptPattern.MatchString(result):
			if !promptSent && promptContent != "" {
				logging.Info("Detected interactive screen, sending prompt content")
				promptSent = true
				if err := exp.Send(promptContent + "\n"); err != nil {
					logging.Error("Failed to send prompt content", "error", err)
					return output.String(), err
				}
				time.Sleep(500 * time.Millisecond)
				if err := exp.Send("\r"); err != nil {
					logging.Error("Failed to send confirmation newline", "error", err)
				}
				time.Sleep(2 * time.Second)
				enterSent = true
			} else {
				logging.Info("Interactive prompt detected again; sending newline")
				if err := exp.Send("\r"); err != nil {
					logging.Error("Failed to send newline", "error", err)
					return output.String(), err
				}
				time.Sleep(2 * time.Second)
				enterSent = true
			}
		// Handle cursor, human, or prompt patterns.
		case cursorPattern.MatchString(result) ||
			humanPattern.MatchString(result) ||
			promptPattern.MatchString(result):
			time.Sleep(500 * time.Millisecond)
			if err := exp.Send("\r"); err != nil {
				logging.Error("Failed to send confirmation newline", "error", err)
			}
			if !promptSent && promptContent != "" {
				logging.Info("Detected prompt, sending prompt content")
				promptSent = true
				if err := exp.Send(promptContent + "\n"); err != nil {
					logging.Error("Failed to send prompt content", "error", err)
					return output.String(), err
				}

				// Small delay before sending enter
				time.Sleep(300 * time.Millisecond)
				if err := exp.Send("\r"); err != nil {
					logging.Error("Failed to send enter key", "error", err)
				}
			} else {
				logging.Info("Prompt detected again; sending newline")
				if err := exp.Send("\r"); err != nil {
					logging.Error("Failed to send newline", "error", err)
					return output.String(), err
				}
				time.Sleep(2 * time.Second)
				enterSent = true
			}
		}

		output.WriteString(result)
	}

	// Monitoring phase:
	logging.Info("Both prompt and enter sent; entering monitoring phase")

	// These patterns capture "esc to interrupt" with potential ANSI color codes anywhere
	escPatterns := []*regexp.Regexp{
		// Basic pattern with flexible spacing
		regexp.MustCompile(`(?i)esc\s*to\s*interrupt`),

		// Pattern with ANSI control sequences between any characters or words
		// \x1b is the escape character, followed by [ and any number of digits, semicolons, and ending with m
		regexp.MustCompile(`(?i)e\s*(?:\x1b\[[0-9;]*m)*s\s*(?:\x1b\[[0-9;]*m)*c\s*(?:\x1b\[[0-9;]*m)*\s*t\s*(?:\x1b\[[0-9;]*m)*o\s*(?:\x1b\[[0-9;]*m)*\s*i\s*(?:\x1b\[[0-9;]*m)*n\s*(?:\x1b\[[0-9;]*m)*t\s*(?:\x1b\[[0-9;]*m)*e\s*(?:\x1b\[[0-9;]*m)*r\s*(?:\x1b\[[0-9;]*m)*r\s*(?:\x1b\[[0-9;]*m)*u\s*(?:\x1b\[[0-9;]*m)*p\s*(?:\x1b\[[0-9;]*m)*t`),

		// Simpler version that looks for "esc" with possible control codes followed by "to" and "interrupt"
		regexp.MustCompile(`(?i)e(?:\x1b\[[0-9;]*m)*s(?:\x1b\[[0-9;]*m)*c(?:\x1b\[[0-9;]*m)*.*?t(?:\x1b\[[0-9;]*m)*o(?:\x1b\[[0-9;]*m)*.*?i(?:\x1b\[[0-9;]*m)*n(?:\x1b\[[0-9;]*m)*t(?:\x1b\[[0-9;]*m)*e(?:\x1b\[[0-9;]*m)*r(?:\x1b\[[0-9;]*m)*r(?:\x1b\[[0-9;]*m)*u(?:\x1b\[[0-9;]*m)*p(?:\x1b\[[0-9;]*m)*t`),

		// More general pattern looking for "esc" and "interrupt" with anything between
		regexp.MustCompile(`(?i)esc.*?interrupt`),

		// Ultra-flexible pattern that can catch heavily formatted text
		regexp.MustCompile(`(?i)e.*?s.*?c.*?t.*?o.*?i.*?n.*?t.*?e.*?r.*?r.*?u.*?p.*?t`),
	}

	// Variables to track pattern presence
	lastDetection := time.Now()
	patternPresent := false
	missedDetectionCount := 0
	monitorStartTime := time.Now()

	// Buffer to check for patterns
	var monitorBuffer strings.Builder
	monitorBuffer.WriteString(output.String()) // Start with current output

	// Monitoring loop
	for {
		// Check if we're over maximum monitoring time (10 minutes)
		if time.Since(monitorStartTime) > 10*time.Minute {
			logging.Warn("Monitoring phase exceeded maximum duration of 10 minutes")
			break
		}

		// Get output with short timeout
		result, _, err := exp.Expect(regexp.MustCompile(`.+`), 500*time.Millisecond)

		// Add any output to both the main output and monitoring buffer
		if err == nil && result != "" {
			output.WriteString(result)
			monitorBuffer.WriteString(result)

			// Every 3 seconds, check for the pattern in the last chunk
			if time.Since(lastDetection) >= 3*time.Second {
				bufferStr := monitorBuffer.String()
				lastChunkSize := 5000
				if len(bufferStr) < lastChunkSize {
					lastChunkSize = len(bufferStr)
				}

				lastChunk := bufferStr[len(bufferStr)-lastChunkSize:]
				patternFound := false

				// Check all patterns
				for i, pattern := range escPatterns {
					if pattern.MatchString(lastChunk) {
						matched := pattern.FindString(lastChunk)
						logging.Info("Found escape pattern",
							"pattern_index", i,
							"matched_text", matched)
						patternFound = true
						patternPresent = true
						lastDetection = time.Now()
						missedDetectionCount = 0
						break
					}
				}

				// If no pattern found and we've seen it before, increment miss counter
				if !patternFound && patternPresent {
					missedDetectionCount++
					logging.Info("No escape pattern found in last chunk",
						"consecutive_misses", missedDetectionCount)

					// If we've missed the pattern 3 times consecutively, assume done
					if missedDetectionCount >= 3 {
						logging.Info("Pattern disappeared for 3 consecutive checks; assuming command is complete")
						break
					}
				}

				// If we've never seen the pattern and monitoring for over 90 seconds, exit
				if !patternPresent && time.Since(monitorStartTime) > 90*time.Second {
					logging.Info("No pattern detected for 90 seconds; assuming command is complete")
					break
				}
			}
		}

		// Check if there's been no output for 10 seconds
		if time.Since(lastDetection) > 10*time.Second && patternPresent {
			logging.Info("No activity for 10 seconds; exiting monitoring phase")
			break
		}
	}

	// After monitoring ends, send escape key several times to signal termination.
	logging.Info("Sending escape keys to exit")
	for i := 0; i < 3; i++ {
		if err := exp.Send("\x1b"); err != nil {
			logging.Error("Failed to send escape key", "error", err)
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Try Ctrl+C as fallback
	logging.Info("Sending Ctrl+C to exit")
	if err := exp.Send("\x03"); err != nil {
		logging.Error("Failed to send Ctrl+C", "error", err)
	}
	time.Sleep(200 * time.Millisecond)

	// Send 'q' which is common exit key
	logging.Info("Sending 'q' to exit")
	if err := exp.Send("q"); err != nil {
		logging.Error("Failed to send q key", "error", err)
	}

	finalOutput := output.String()
	logging.Info("Command completed successfully", "output_length", len(finalOutput))
	return finalOutput, nil
}
