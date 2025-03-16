package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hellausefulsoftware/useful1/internal/config"
)

// TestExecuteCommandFlagHandling tests that the 'execute' command 
// correctly passes all arguments (including flags) to the underlying tool
func TestExecuteCommandFlagHandling(t *testing.T) {
	// Skip this test when running 'go test ./...' without the 'integration' tag
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// This test needs to be run from the project root directory
	// Get current directory
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Find path to the project root (where the bin directory would be)
	projectRoot := wd
	for {
		if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(projectRoot)
		if parent == projectRoot {
			t.Fatalf("Could not find project root directory with go.mod")
			break
		}
		projectRoot = parent
	}

	// Path to the binary
	binaryPath := filepath.Join(projectRoot, "bin", "useful1")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skip("Skipping integration test: binary not found at", binaryPath)
		return
	}

	// Create a temporary config file for testing
	tmpConfig := filepath.Join(os.TempDir(), "useful1_test_config.json")
	defer os.Remove(tmpConfig)

	// Write test config with echo as the CLI command
	cfg := &config.Config{}
	cfg.GitHub.Token = "dummy-token"
	cfg.Anthropic.Token = "dummy-token"
	cfg.CLI.Command = "/bin/echo"
	cfg.CLI.Args = []string{}

	// Save config to file
	if err := cfg.SaveToFile(tmpConfig); err != nil {
		t.Fatalf("Failed to save test config: %v", err)
	}

	// Setup test cases
	testCases := []struct {
		name      string
		args      []string
		expectContain []string
	}{
		{
			name: "Simple arguments",
			args: []string{"arg1", "arg2", "arg3"},
			expectContain: []string{"arg1", "arg2", "arg3"},
		},
		{
			name: "Flag arguments",
			args: []string{"-p", "paramValue", "--flag", "value"},
			expectContain: []string{"-p", "paramValue", "--flag", "value"},
		},
		{
			name: "Mixed arguments",
			args: []string{"normal", "-p", "flag-value", "--long-flag"},
			expectContain: []string{"normal", "-p", "flag-value", "--long-flag"},
		},
	}

	// Run tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create command to execute
			args := append([]string{"execute"}, tc.args...)
			cmd := exec.Command(binaryPath, args...)

			// Setup environment to use test config
			cmd.Env = append(os.Environ(), 
				fmt.Sprintf("USEFUL1_CONFIG=%s", tmpConfig),
				"GITHUB_TOKEN=dummy-token",
				"ANTHROPIC_API_KEY=dummy-token")

			// Capture output
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			// Add timeout to prevent hanging
			timeout := time.After(5 * time.Second)
			done := make(chan error, 1)
			
			// Run command in a goroutine
			go func() {
				done <- cmd.Run()
			}()
			
			// Wait for command to finish or timeout
			select {
			case <-timeout:
				t.Fatalf("Command execution timed out after 5 seconds")
				return
			case err := <-done:
				if err != nil {
					t.Logf("Command returned error: %v (this may be expected)", err)
				}
			}

			// Check output (combined stdout and stderr)
			output := stdout.String() + stderr.String()
			
			// Look for expected arguments in the output
			for _, expected := range tc.expectContain {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain '%s', but got:\n%s", expected, output)
				}
			}
		})
	}
}