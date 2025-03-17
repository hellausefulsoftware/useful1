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
		timeout = 120 // Default 120 seconds if not set or invalid.
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
// have been sent. It then enters a monitoring phase that polls every 100ms for the text
// "esc to interrupt". If 5 seconds pass without a new detection, the monitoring phase ends.
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
		timeout = 120 // Default 120 seconds.
	}
	timeoutDuration := time.Duration(timeout) * time.Second

	logging.Info("Using expect to handle interactive prompts", "command", cmdParts[0], "args", cmdArgs, "timeout", timeoutDuration)

	// Spawn command with expect.
	cmd := exec.Command(cmdParts[0], cmdArgs...)
	exp, _, err := expect.SpawnWithArgs(cmd.Args,
		timeoutDuration,
		expect.Verbose(true),
		expect.VerboseWriter(os.Stdout),
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
				logging.Info("Expect timed out waiting for more output", "timeout_seconds", 5)
				if emptyOutputCount > 3 && !promptSent && promptContent != "" {
					logging.Info("Timeout with multiple empty outputs, sending prompt content")
					promptSent = true
					if sendErr := exp.Send(promptContent + "\n"); sendErr != nil {
						logging.Error("Failed to send prompt content", "error", sendErr)
						return output.String(), sendErr
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
			if err := exp.Send("y\n"); err != nil {
				logging.Error("Failed to send yes response", "error", err)
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
	lastDetection := time.Now()
	// Poll for "esc to interrupt" every 100ms.
	for {
		result, _, err := exp.Expect(regexp.MustCompile(`esc to interrupt`), 100*time.Millisecond)
		// If we detect the string, update lastDetection and log it.
		if err == nil && result != "" {
			lastDetection = time.Now()
			output.WriteString(result)
			logging.Info("'esc to interrupt' detected; updating last detection time")
		}
		// If 5 seconds have passed with no new detection, exit monitoring.
		if time.Since(lastDetection) >= 5*time.Second {
			logging.Info("No new 'esc to interrupt' detected in the last 5 seconds; exiting monitoring phase")
			break
		}
	}

	// After monitoring ends, send escape key several times to signal termination.
	for i := 0; i < 5; i++ {
		if err := exp.Send("\x1b"); err != nil {
			logging.Error("Failed to send escape key", "error", err)
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	finalOutput := output.String()
	logging.Info("Command completed successfully", "output_length", len(finalOutput))
	return finalOutput, nil
}
