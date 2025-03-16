package anthropic

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	anthropicAPI "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/hellausefulsoftware/useful1/internal/config"
	"github.com/hellausefulsoftware/useful1/internal/logging"
	"github.com/hellausefulsoftware/useful1/internal/models"
)

// Constants for API
const (
	AnalysisModel   = "claude-3-7-sonnet-20250219"
	SummaryModel    = "claude-3-7-sonnet-20250219" // Latest Sonnet model
	ClassifierModel = "claude-3-5-haiku-20241022"  // Latest Haiku model
)

// Issue type classification
const (
	TypeBug     = "bug"
	TypeFeature = "feature"
	TypeChore   = "chore"
)

// IssueAnalyzer provides methods to analyze GitHub issues using Anthropic API
type IssueAnalyzer struct {
	config *config.Config
	client *anthropicAPI.Client
}

// NewAnalyzer creates a new issue analyzer
func NewAnalyzer(cfg *config.Config) *IssueAnalyzer {
	// Log basic info without revealing full token
	var tokenStatus string
	if cfg.Anthropic.Token == "" {
		tokenStatus = "empty"
	} else {
		tokenLen := len(cfg.Anthropic.Token)
		last4 := ""
		if tokenLen >= 4 {
			last4 = cfg.Anthropic.Token[tokenLen-4:]
		}
		tokenStatus = fmt.Sprintf("provided (length: %d, ends with: %s)", tokenLen, last4)
	}
	logging.Info("Creating Anthropic analyzer", "token_status", tokenStatus)

	// Attempt to decode the token if it looks base64 encoded
	token := cfg.Anthropic.Token
	if !strings.HasPrefix(token, "sk-ant-") {
		// Try to decode it as base64
		decoded, err := base64.StdEncoding.DecodeString(token)
		if err == nil {
			decodedStr := string(decoded)
			if strings.HasPrefix(decodedStr, "sk-ant-") {
				token = decodedStr
				logging.Info("Successfully decoded base64 Anthropic token")
			}
		}
	}

	// Create anthropic client with API key from config
	client := anthropicAPI.NewClient(
		option.WithAPIKey(token),
	)

	// Validate token format (basic check)
	if !strings.HasPrefix(token, "sk-ant-") {
		logging.Warn("Anthropic token appears to be in incorrect format",
			"format_valid", strings.HasPrefix(token, "sk-ant-"))
	}

	return &IssueAnalyzer{
		config: cfg,
		client: client,
	}
}

// SummarizeIssue takes an issue transcript and returns a concise summary
func (a *IssueAnalyzer) SummarizeIssue(transcript string) (string, error) {
	return a.summarizeIssue(transcript)
}

// AnalyzeIssue analyzes an issue using the Anthropic API and returns a branch name suggestion
func (a *IssueAnalyzer) AnalyzeIssue(issue *models.Issue) (string, error) {
	// 1. Compile issue transcript
	transcript := formatIssueTranscript(issue)
	logging.Debug("Created initial issue transcript",
		"length", len(transcript),
		"issue_number", issue.Number,
		"issue_title", issue.Title,
		"issue_body_length", len(issue.Body),
		"comment_count", len(issue.Comments))

	// 2. Summarize the issue using Claude 3.5 Sonnet
	logging.Info("Requesting issue summary from Anthropic API")
	summary, err := a.summarizeIssue(transcript)
	if err != nil {
		logging.Error("Failed to summarize issue", "error", err)
		return defaultBranchName(issue), err
	}
	logging.Info("Received issue summary", "length", len(summary))

	// 3. Classify the issue type using Claude 3 Haiku
	logging.Info("Requesting issue classification from Anthropic API")
	issueType, err := a.classifyIssueType(summary)
	if err != nil {
		logging.Error("Failed to classify issue type", "error", err)
		return defaultBranchName(issue), err
	}
	logging.Info("Received issue classification", "issue_type", issueType)

	// 4. Generate a descriptive branch name
	logging.Info("Requesting branch name generation from Anthropic API")
	branchName, err := a.generateBranchName(summary, issueType, issue)
	if err != nil {
		logging.Error("Failed to generate branch name", "error", err)
		return defaultBranchName(issue), err
	}
	logging.Info("Received branch name", "branch_name", branchName)

	return branchName, nil
}

// formatIssueTranscript creates a formatted transcript of the issue and its comments
func formatIssueTranscript(issue *models.Issue) string {
	var transcript strings.Builder

	// Issue metadata
	transcript.WriteString(fmt.Sprintf("ISSUE #%d: %s\n\n", issue.Number, issue.Title))
	transcript.WriteString(fmt.Sprintf("Created by: %s\n", issue.User))
	transcript.WriteString(fmt.Sprintf("State: %s\n", issue.State))
	transcript.WriteString(fmt.Sprintf("Created: %s\n", issue.CreatedAt.Format("2006-01-02")))
	transcript.WriteString(fmt.Sprintf("Updated: %s\n", issue.UpdatedAt.Format("2006-01-02")))

	if len(issue.Labels) > 0 {
		transcript.WriteString(fmt.Sprintf("Labels: %s\n", strings.Join(issue.Labels, ", ")))
	}

	if len(issue.Assignees) > 0 {
		transcript.WriteString(fmt.Sprintf("Assignees: %s\n", strings.Join(issue.Assignees, ", ")))
	}

	// Issue description
	transcript.WriteString("\nISSUE DESCRIPTION:\n")
	transcript.WriteString(issue.Body)
	transcript.WriteString("\n\n")

	// Comments
	if len(issue.Comments) > 0 {
		transcript.WriteString("COMMENTS:\n\n")
		for i, comment := range issue.Comments {
			transcript.WriteString(fmt.Sprintf("--- Comment #%d by %s (%s) ---\n",
				i+1,
				comment.User,
				comment.CreatedAt.Format("2006-01-02")))
			transcript.WriteString(comment.Body)
			transcript.WriteString("\n\n")
		}
	}

	return transcript.String()
}

// summarizeIssue uses Claude 3.5 Sonnet to summarize the issue
func (a *IssueAnalyzer) summarizeIssue(transcript string) (string, error) {
	prompt := `You are a technical project manager reviewing GitHub issues. Analyze this issue transcript and provide a concise summary. 
Focus only on the technical details and remove any off-topic comments or non-technical discussions.
Be brief but detailed enough to understand the core problem or request.

ISSUE TRANSCRIPT:
${transcript}

Provide a concise summary of the issue in 1-3 short paragraphs, focusing only on the relevant technical points.`

	prompt = strings.Replace(prompt, "${transcript}", transcript, 1)

	logging.Debug("Sending issue summary request to Anthropic API", "model", SummaryModel)

	// Debug the request
	logging.Debug("Anthropic API request details for summarization",
		"model", SummaryModel,
		"max_tokens", 500,
		"prompt_length", len(prompt))

	// Create a message using the SDK
	message, err := a.client.Messages.New(context.Background(), anthropicAPI.MessageNewParams{
		Model:     anthropicAPI.F(SummaryModel),
		MaxTokens: anthropicAPI.F(int64(500)),
		Messages: anthropicAPI.F([]anthropicAPI.MessageParam{
			anthropicAPI.NewUserMessage(
				anthropicAPI.NewTextBlock(prompt),
			),
		}),
	})

	if err != nil {
		logging.Error("Anthropic API error",
			"error", err.Error(),
			"error_type", fmt.Sprintf("%T", err))
		return "", fmt.Errorf("failed to summarize issue: %w", err)
	}

	// Extract response text from the message
	if len(message.Content) == 0 {
		logging.Warn("Empty response from Anthropic API")
		return "", fmt.Errorf("empty response from API")
	}

	var responseText string
	for _, content := range message.Content {
		if content.Type == "text" {
			responseText += content.Text
		}
	}

	logging.Info("Successfully received response from Anthropic API",
		"response_length", len(responseText),
		"content_items", len(message.Content))

	logging.Debug("Received issue summary from Anthropic API", "length", len(responseText))

	return responseText, nil
}

// classifyIssueType uses Claude 3 Haiku to determine the issue type
func (a *IssueAnalyzer) classifyIssueType(summary string) (string, error) {
	prompt := `You are a software development issue classifier. Based on the following issue summary, classify this issue as one of these types:
- bug: A problem with existing functionality
- feature: A request for new functionality
- chore: Regular maintenance, refactoring, or administrative tasks

Issue Summary:
${summary}

Respond with only one word: "bug", "feature", or "chore".`

	prompt = strings.Replace(prompt, "${summary}", summary, 1)

	logging.Debug("Sending issue classification request to Anthropic API", "model", ClassifierModel)

	// Debug the request
	logging.Debug("Anthropic API request details for classification",
		"model", ClassifierModel,
		"max_tokens", 10,
		"prompt_length", len(prompt),
		"summary_length", len(summary))

	// Create a message using the SDK
	message, err := a.client.Messages.New(context.Background(), anthropicAPI.MessageNewParams{
		Model:     anthropicAPI.F(ClassifierModel),
		MaxTokens: anthropicAPI.F(int64(10)),
		Messages: anthropicAPI.F([]anthropicAPI.MessageParam{
			anthropicAPI.NewUserMessage(
				anthropicAPI.NewTextBlock(prompt),
			),
		}),
	})

	if err != nil {
		logging.Error("Failed to classify issue",
			"error", err.Error(),
			"error_type", fmt.Sprintf("%T", err))
		return TypeBug, fmt.Errorf("failed to classify issue: %w", err)
	}

	logging.Info("Successfully received classification from Anthropic API",
		"content_items", len(message.Content))

	// Extract response text
	var issueType string
	if len(message.Content) > 0 {
		for _, content := range message.Content {
			if content.Type == "text" {
				issueType += content.Text
			}
		}
	}

	// Clean up response and normalize
	issueType = strings.ToLower(strings.TrimSpace(issueType))

	// Validate and normalize the response
	switch {
	case strings.Contains(issueType, "bug"):
		return TypeBug, nil
	case strings.Contains(issueType, "feature"):
		return TypeFeature, nil
	case strings.Contains(issueType, "chore"):
		return TypeChore, nil
	default:
		logging.Warn("Unknown issue type from classifier, defaulting to bug", "raw_type", issueType)
		return TypeBug, nil
	}
}

// generateBranchName creates a formatted branch name based on the issue analysis
func (a *IssueAnalyzer) generateBranchName(summary string, issueType string, issue *models.Issue) (string, error) {
	// Generate a short, descriptive name for the branch
	prompt := `Based on this issue summary, generate a short, descriptive name for a git branch.
The name should be 3-5 words maximum, use lowercase with hyphens instead of spaces, and clearly describe the purpose.
Don't include issue numbers or prefixes.

Issue Summary:
${summary}

Give only the branch name, e.g., "fix-header-overflow" or "add-user-permissions".`

	prompt = strings.Replace(prompt, "${summary}", summary, 1)

	logging.Debug("Sending branch name generation request to Anthropic API", "model", ClassifierModel)

	// Debug the request
	logging.Debug("Anthropic API request details for branch name generation",
		"model", ClassifierModel,
		"max_tokens", 20,
		"prompt_length", len(prompt),
		"summary_length", len(summary))

	// Create a message using the SDK
	message, err := a.client.Messages.New(context.Background(), anthropicAPI.MessageNewParams{
		Model:     anthropicAPI.F(ClassifierModel),
		MaxTokens: anthropicAPI.F(int64(20)),
		Messages: anthropicAPI.F([]anthropicAPI.MessageParam{
			anthropicAPI.NewUserMessage(
				anthropicAPI.NewTextBlock(prompt),
			),
		}),
	})

	if err != nil {
		logging.Error("Failed to generate branch name",
			"error", err.Error(),
			"error_type", fmt.Sprintf("%T", err))
		return defaultBranchName(issue), fmt.Errorf("failed to generate branch name: %w", err)
	}

	logging.Info("Successfully received branch name from Anthropic API",
		"content_items", len(message.Content))

	// Extract response text
	var branchName string
	if len(message.Content) > 0 {
		for _, content := range message.Content {
			if content.Type == "text" {
				branchName += content.Text
			}
		}
	}

	// Clean up response
	branchName = strings.ToLower(strings.TrimSpace(branchName))

	// Remove any quotes, dots, etc.
	branchName = strings.Trim(branchName, "`\"'.,")

	// Replace spaces with hyphens if present
	branchName = strings.ReplaceAll(branchName, " ", "-")

	// Determine the branch prefix
	var prefix string
	switch issueType {
	case TypeBug:
		prefix = "bugfix"
	case TypeFeature:
		prefix = "feature"
	case TypeChore:
		prefix = "chore"
	default:
		prefix = "bugfix"
	}

	// Get username from config
	username := a.config.GitHub.User

	// Format the final branch name
	finalBranchName := fmt.Sprintf("%s/%s-issue-%d-%s",
		prefix,
		username,
		issue.Number,
		branchName)

	// Ensure it doesn't have double hyphens
	finalBranchName = strings.ReplaceAll(finalBranchName, "--", "-")

	logging.Info("Generated branch name", "name", finalBranchName)

	return finalBranchName, nil
}

// defaultBranchName generates a simple branch name as a fallback
func defaultBranchName(issue *models.Issue) string {
	// Sanitize the title for use in a branch name
	sanitizedTitle := strings.ToLower(issue.Title)
	sanitizedTitle = strings.ReplaceAll(sanitizedTitle, " ", "-")
	sanitizedTitle = strings.ReplaceAll(sanitizedTitle, "/", "-")
	sanitizedTitle = strings.ReplaceAll(sanitizedTitle, ":", "")
	sanitizedTitle = strings.ReplaceAll(sanitizedTitle, ".", "")
	sanitizedTitle = strings.ReplaceAll(sanitizedTitle, ",", "")

	// Limit branch name length
	if len(sanitizedTitle) > 50 {
		sanitizedTitle = sanitizedTitle[:50]
	}

	return fmt.Sprintf("bugfix/issue-%d-%s", issue.Number, sanitizedTitle)
}
