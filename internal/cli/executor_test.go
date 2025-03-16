package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/hellausefulsoftware/useful1/internal/config"
)

// MockGitHubClient is a mock implementation of the GitHub client for testing
type MockGitHubClient struct {
	RespondToIssueFunc    func(owner, repo string, issueNumber int, comment string) error
	CreatePullRequestFunc func(owner, repo, title, body, head, base string) error
	GetIssuesFunc         func(owner, repo string) error
	GetIssueCommentsFunc  func(owner, repo string, issueNumber int) error
}

// Test helper to create a basic test config
func createTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.GitHub.Token = "github-token"
	cfg.Anthropic.Token = "anthropic-token"
	cfg.CLI.Command = "echo"
	cfg.CLI.Args = []string{"--test"}
	cfg.Budgets.IssueResponse = 0.5
	cfg.Budgets.PRCreation = 1.0
	cfg.Budgets.TestRun = 0.3
	cfg.Budgets.Default = 0.2
	return cfg
}

func TestNewExecutor(t *testing.T) {
	// Create a test config
	cfg := createTestConfig()

	// Create a new executor
	executor := NewExecutor(cfg)
	if executor == nil {
		t.Fatal("NewExecutor returned nil")
	}
	if executor.config != cfg {
		t.Error("Executor has incorrect config reference")
	}
	if executor.github == nil {
		t.Error("Executor has nil GitHub client")
	}
}

// TestExtractResponse tests the extractResponse method
func TestExtractResponse(t *testing.T) {
	// Create a test executor
	executor := &Executor{
		config: createTestConfig(),
	}

	tests := []struct {
		name     string
		output   string
		expected string
		wantErr  bool
	}{
		{
			name:     "JSON response",
			output:   "Some output\nRESPONSE_JSON:{\"content\":\"Test response\"}",
			expected: "Test response",
			wantErr:  false,
		},
		{
			name:     "Plain text response",
			output:   "Some output\nRESPONSE:Test response",
			expected: "Test response",
			wantErr:  false,
		},
		{
			name:     "No response marker - return full output",
			output:   "Some output\nNo specific response marker",
			expected: "Automated response:\n\n```\nSome output\nNo specific response marker\n```",
			wantErr:  false,
		},
		{
			name:     "Malformed JSON response",
			output:   "Some output\nRESPONSE_JSON:{invalid json",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := executor.extractResponse(tt.output)

			// Check error condition
			if (err != nil) != tt.wantErr {
				t.Errorf("extractResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Skip further checks if we expected an error
			if tt.wantErr {
				return
			}

			// Check response content
			if response != tt.expected {
				t.Errorf("extractResponse() got = %v, want %v", response, tt.expected)
			}
		})
	}
}

// TestCheckCriteria tests the checkCriteria method
func TestCheckCriteria(t *testing.T) {
	// Create a test executor
	executor := &Executor{
		config: createTestConfig(),
	}

	tests := []struct {
		name     string
		output   string
		criteria []string
		expected bool
	}{
		{
			name:     "All criteria met",
			output:   "This is a test output\nwith multiple lines\ncontaining criteria one\nand criteria two",
			criteria: []string{"criteria one", "criteria two"},
			expected: true,
		},
		{
			name:     "Some criteria missing",
			output:   "This is a test output\nwith multiple lines\ncontaining criteria one",
			criteria: []string{"criteria one", "missing criteria"},
			expected: false,
		},
		{
			name:     "No criteria met",
			output:   "This is a test output\nwith multiple lines",
			criteria: []string{"missing criteria", "also missing"},
			expected: false,
		},
		{
			name:     "Empty criteria",
			output:   "This is a test output",
			criteria: []string{},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.checkCriteria(tt.output, tt.criteria)
			if result != tt.expected {
				t.Errorf("checkCriteria() got = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestRunTests tests the RunTests method with mocked command execution
func TestRunTests(t *testing.T) {
	// Skip this test if you're not on a system with echo command
	if _, err := os.Stat("/bin/echo"); os.IsNotExist(err) {
		t.Skip("Skipping test as /bin/echo does not exist on this system")
	}

	// Create a test config that uses echo for the CLI command
	cfg := createTestConfig()
	cfg.CLI.Command = "/bin/echo"
	cfg.CLI.Args = []string{}

	// Create a test executor
	executor := &Executor{
		config: cfg,
	}

	// Test RunTests
	err := executor.RunTests("test-suite")
	if err != nil {
		t.Fatalf("RunTests returned error: %v", err)
	}
}

// Integration test for error handling
func TestFormatErrorResponse(t *testing.T) {
	// Create a test executor
	executor := &Executor{
		config: createTestConfig(),
	}

	// Test error formatting
	testErr := fmt.Errorf("test error")
	context := map[string]interface{}{
		"test_key": "test_value",
	}

	// Redirect stdout to capture the output
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	os.Stdout = w

	// Call the method
	executor.formatErrorResponse(testErr, context)

	// Restore stdout
	if closeErr := w.Close(); closeErr != nil {
		t.Logf("Warning: Failed to close pipe writer: %v", closeErr)
	}
	os.Stdout = oldStdout

	// Read captured output
	var buf [1024]byte
	n, readErr := r.Read(buf[:])
	if readErr != nil && readErr != io.EOF {
		t.Fatalf("Failed to read from pipe: %v", readErr)
	}
	output := string(buf[:n])

	// Verify the output contains expected fields
	if !strings.Contains(output, "\"status\":\"error\"") {
		t.Errorf("formatErrorResponse() output doesn't contain status field")
	}
	if !strings.Contains(output, "\"message\":\"test error\"") {
		t.Errorf("formatErrorResponse() output doesn't contain correct error message")
	}
	if !strings.Contains(output, "\"test_key\":\"test_value\"") {
		t.Errorf("formatErrorResponse() output doesn't contain context fields")
	}
}

// TestExecuteWithArguments tests that the Execute method correctly passes all arguments
// including flags to the underlying command, but uses executeWithPrompts to avoid terminal interaction
func TestExecuteWithArguments(t *testing.T) {
	// Skip this test if you're not on a system with echo command
	if _, err := os.Stat("/bin/echo"); os.IsNotExist(err) {
		t.Skip("Skipping test as /bin/echo does not exist on this system")
	}

	// Create a test config that uses echo for the CLI command
	cfg := createTestConfig()
	cfg.CLI.Command = "/bin/echo"
	cfg.CLI.Args = []string{}

	// Create a test executor
	executor := &Executor{
		config: cfg,
	}

	// Prepare test cases with different types of arguments
	testCases := []struct {
		name         string
		args         []string
		shouldContain []string
	}{
		{
			name: "Simple arguments",
			args: []string{"arg1", "arg2", "arg3"},
			shouldContain: []string{"arg1", "arg2", "arg3"},
		},
		{
			name: "Flag arguments",
			args: []string{"-p", "param", "--flag", "value"},
			shouldContain: []string{"-p", "param", "--flag", "value"},
		},
		{
			name: "Mixed arguments",
			args: []string{"normal", "-f", "flag-value", "--long-flag"},
			shouldContain: []string{"normal", "-f", "flag-value", "--long-flag"},
		},
		{
			name: "Quoted arguments",
			args: []string{"-p", "\"quoted param\"", "--name", "value with spaces"},
			shouldContain: []string{"-p", "\"quoted param\"", "--name", "value with spaces"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Use executeWithPrompts instead of Execute to avoid terminal interaction
			output, err := executor.executeWithPrompts(executor.config.CLI.Command, tc.args)
			
			// Check for errors
			if err != nil {
				t.Fatalf("executeWithPrompts returned error: %v", err)
			}

			// Verify all expected arguments appear in the output
			for _, arg := range tc.shouldContain {
				if !strings.Contains(output, arg) {
					t.Errorf("Expected output to contain '%s', but got: %s", arg, output)
				}
			}
		})
	}
}