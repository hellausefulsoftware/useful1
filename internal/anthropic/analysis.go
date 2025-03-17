// Package anthropic provides integration with Anthropic's API for issue analysis
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
	AnalysisModel   = "claude-3-7-sonnet-20250219" // For detailed analysis
	SummaryModel    = "claude-3-7-sonnet-20250219" // For issue summarization
	ClassifierModel = "claude-3-5-haiku-20241022"  // For classification tasks
	CommitModel     = "claude-3-5-haiku-20241022"  // For commit messages
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

// GenerateImplementationPlan generates a detailed implementation plan for solving an issue
func (a *IssueAnalyzer) GenerateImplementationPlan(issue *models.Issue) (string, error) {
	// Create a complete transcript of the issue for analysis
	transcript := formatIssueTranscript(issue)

	// Use Claude 3.7 Sonnet to generate an implementation plan
	return a.generateImplementationPlan(transcript, issue)
}

// GeneratePRDescription generates a comprehensive PR description for the created implementation
func (a *IssueAnalyzer) GeneratePRDescription(issue *models.Issue, implementationPlan string, changedFiles []string) (string, error) {
	// Create a complete transcript of the issue for analysis
	transcript := formatIssueTranscript(issue)

	// Log the implementation plan for debugging purposes
	logging.Info("Generating PR description", "implementationPlan", implementationPlan)

	// Use Claude 3.7 Sonnet to generate a detailed PR description
	return a.generatePRDescription(transcript, implementationPlan, changedFiles)
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

// generateImplementationPlan creates a detailed implementation plan using Claude 3.7 Sonnet
func (a *IssueAnalyzer) generateImplementationPlan(transcript string, issue *models.Issue) (string, error) {
	prompt := `You are implementing a solution for a GitHub issue. 
	
I'll provide the details of the issue, and you need to create the solution.
ISSUE TRANSCRIPT:
${transcript}

First, briefly understand what needs to be changed.

Come with a plan for what specific actions someone should take, write instructions as an order such as "run git commit to make the the changes"
Each step should be concise and written as a direct instruction.

After each Step incorporate into the plan to run "make lint-all" and "make test", then fix any issues that come up. 

IMPORTANT: As part of the plan, at the end write these EXACT commands (replacing placeholders with actual values):
   git add .
   git commit -m "feat: [specific action taken] for issue #${issue_number}"
   git push origin HEAD

The commit message should clearly describe the specific changes made (e.g., "feat: add user authentication flow" instead of just "implement solution").

Given all of the above parameters, what is the step by step plan? Do not omit or abbreviate any steps. Be as detailed as possible.
`

	prompt = strings.Replace(prompt, "${transcript}", transcript, 1)
	prompt = strings.Replace(prompt, "${issue_number}", fmt.Sprintf("%d", issue.Number), 1)

	logging.Debug("Sending implementation plan request to Anthropic API", "model", AnalysisModel)

	logging.Debug("Anthropic API request details for implementation plan",
		"model", AnalysisModel,
		"max_tokens", 2000,
		"prompt_length", len(prompt))

	// Create a message using the SDK
	message, err := a.client.Messages.New(context.Background(), anthropicAPI.MessageNewParams{
		Model:     anthropicAPI.F(AnalysisModel),
		MaxTokens: anthropicAPI.F(int64(2000)),
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
		return "", fmt.Errorf("failed to generate implementation plan: %w", err)
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

	logging.Info("Successfully received implementation plan from Anthropic API",
		"response_length", len(responseText),
		"content_items", len(message.Content))

	return responseText, nil
}

// generatePRDescription creates a detailed PR description using Claude 3.7 Sonnet
func (a *IssueAnalyzer) generatePRDescription(transcript string, implementationPlan string, changedFiles []string) (string, error) {
	// Handle empty implementation plan
	if implementationPlan == "" {
		implementationPlan = "No implementation provided yet."
	}

	// Handle empty changed files
	var changedFilesText string
	if len(changedFiles) == 0 {
		changedFilesText = "No files have been changed yet."
	} else {
		changedFilesText = strings.Join(changedFiles, "\n")
	}

	prompt := `You are a senior software engineer creating a detailed, professional pull request (PR) description for a GitHub issue.
Based on the issue transcript, Claude's implementation output, and the list of changed files, write a comprehensive PR description that clearly explains the changes that were made.

ISSUE TRANSCRIPT:
${transcript}

CLAUDE OUTPUT (implementation that was already done):
${implementation_plan}

CHANGED FILES:
${changed_files}

Create a detailed PR description that includes:

1. Problem Summary:
   - Clear statement of the problem addressed
   - Expected vs. actual behavior before the fix
   - Root cause analysis (if applicable)

2. Solution Implemented:
   - Detailed explanation of the approach that was taken
   - Key changes that were made and their purpose
   - Design decisions and trade-offs that were considered

3. Testing Performed:
   - How the changes were tested
   - Test cases that validate the solution
   - Any edge cases that were considered

4. Additional Information:
   - Impact on other systems
   - Any migration steps required
   - Documentation updates included

IMPORTANT:
- Only include items in your PR description that actually appear in the CLAUDE OUTPUT or CHANGED FILES.
- Do NOT invent or fabricate activities that don't appear in the output.
- If the output doesn't mention tests, don't claim tests were performed.
- Only mention files that were actually changed in the CHANGED FILES section.
- Be accurate and truthful - if very little was done, keep your description short.
- Use past tense to describe only the actual work performed.

Format the PR description in Markdown with clear sections, bullet points, and code snippets where appropriate.
Focus on providing a thorough explanation of what was already implemented, not what will be implemented in the future.
The implementation is complete - use past tense to describe what was done, not future tense for what will be done.`

	// Replace placeholders
	prompt = strings.Replace(prompt, "${transcript}", transcript, 1)
	prompt = strings.Replace(prompt, "${implementation_plan}", implementationPlan, 1)
	prompt = strings.Replace(prompt, "${changed_files}", changedFilesText, 1)

	logging.Debug("Sending PR description request to Anthropic API", "model", AnalysisModel)

	logging.Debug("Anthropic API request details for PR description",
		"model", AnalysisModel,
		"max_tokens", 2000,
		"prompt_length", len(prompt))

	// Create a message using the SDK
	message, err := a.client.Messages.New(context.Background(), anthropicAPI.MessageNewParams{
		Model:     anthropicAPI.F(AnalysisModel),
		MaxTokens: anthropicAPI.F(int64(2000)),
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
		return "", fmt.Errorf("failed to generate PR description: %w", err)
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

	logging.Info("Successfully received PR description from Anthropic API",
		"response_length", len(responseText),
		"content_items", len(message.Content))

	// Add footer to the description
	responseText += "\n\n---\n*This PR description was generated with Claude 3.7 Sonnet*"

	return responseText, nil
}

// GenerateCommitMessage creates a concise, descriptive commit message using Claude 3.5 Haiku
func (a *IssueAnalyzer) GenerateCommitMessage(issue *models.Issue, changedFiles []string, changeSummary string) (string, error) {
	// Create a prompt for the commit message
	prompt := `You are a software developer creating a concise and meaningful git commit message.
Based on the issue description and the changed files, write a clear, specific commit message.

ISSUE: #${issue_number} - ${issue_title}
${issue_description}

CHANGED FILES:
${changed_files}

CHANGES SUMMARY:
${change_summary}

Create a descriptive commit message that follows these guidelines:
1. Start with a verb in present tense (e.g., "Add", "Fix", "Update", "Refactor", "Implement")
2. Be specific about what changed and why
3. Keep it under 80 characters for the first line
4. Include the issue number in the message with the format "Fix #123" or "Implement #123" depending on issue type
5. Follow conventional commit format if appropriate (feat:, fix:, docs:, refactor:, etc.)

Respond with ONLY the commit message, nothing else.`

	// Replace placeholders
	prompt = strings.Replace(prompt, "${issue_number}", fmt.Sprintf("%d", issue.Number), 1)
	prompt = strings.Replace(prompt, "${issue_title}", issue.Title, 1)
	prompt = strings.Replace(prompt, "${issue_description}", issue.Body, 1)
	prompt = strings.Replace(prompt, "${changed_files}", strings.Join(changedFiles, "\n"), 1)
	prompt = strings.Replace(prompt, "${change_summary}", changeSummary, 1)

	logging.Debug("Sending commit message request to Anthropic API", "model", CommitModel)

	logging.Debug("Anthropic API request details for commit message",
		"model", CommitModel,
		"max_tokens", 150,
		"prompt_length", len(prompt))

	// Create a message using the SDK
	message, err := a.client.Messages.New(context.Background(), anthropicAPI.MessageNewParams{
		Model:     anthropicAPI.F(CommitModel),
		MaxTokens: anthropicAPI.F(int64(150)),
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
		return fmt.Sprintf("Add implementation for issue #%d", issue.Number), fmt.Errorf("failed to generate commit message: %w", err)
	}

	// Extract response text from the message
	if len(message.Content) == 0 {
		logging.Warn("Empty response from Anthropic API")
		return fmt.Sprintf("Add implementation for issue #%d", issue.Number), fmt.Errorf("empty response from API")
	}

	var commitMessage string
	for _, content := range message.Content {
		if content.Type == "text" {
			commitMessage += content.Text
		}
	}

	// Trim and clean up the commit message
	commitMessage = strings.TrimSpace(commitMessage)

	// Check if the message is overly long and split it if needed
	lines := strings.Split(commitMessage, "\n")
	if len(lines) > 0 && len(lines[0]) > 80 {
		// Truncate to 80 chars if way too long
		if len(lines[0]) > 120 {
			lines[0] = lines[0][:77] + "..."
		}
	}

	// Reconstruct the message
	commitMessage = strings.Join(lines, "\n")

	logging.Info("Successfully generated commit message",
		"message", commitMessage,
		"length", len(commitMessage))

	return commitMessage, nil
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
