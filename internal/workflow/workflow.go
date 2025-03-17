// Package workflow provides implementation orchestration for issue resolution
package workflow

import (
	"github.com/google/go-github/v45/github"
	"github.com/hellausefulsoftware/useful1/internal/config"
	"github.com/hellausefulsoftware/useful1/internal/workflow/services"
)

// ImplementationWorkflow represents the complete implementation workflow
type ImplementationWorkflow struct {
	config                *config.Config
	implementationService *services.GitHubImplementationService
}

// NewImplementationWorkflow creates a new implementation workflow
func NewImplementationWorkflow(cfg *config.Config) *ImplementationWorkflow {
	return &ImplementationWorkflow{
		config:                cfg,
		implementationService: services.NewGitHubImplementationService(cfg),
	}
}

// NewImplementationWorkflowWithService creates a new implementation workflow with a provided service
func NewImplementationWorkflowWithService(cfg *config.Config, service *services.GitHubImplementationService) *ImplementationWorkflow {
	return &ImplementationWorkflow{
		config:                cfg,
		implementationService: service,
	}
}

// GenerateBranchAndTitle generates a branch name and PR title for an issue
func (w *ImplementationWorkflow) GenerateBranchAndTitle(owner, repo, title, body string) (string, string, error) {
	return w.implementationService.GenerateBranchAndTitle(owner, repo, title, body)
}

// CreateImplementationPromptAndExecute creates and executes an implementation plan
// Returns the Claude CLI output for use in PR description
func (w *ImplementationWorkflow) CreateImplementationPromptAndExecute(owner, repo, branchName string, issueNumber int) (string, error) {
	return w.implementationService.CreateImplementationPromptAndExecute(owner, repo, branchName, issueNumber)
}

// CreatePullRequestForIssue creates a PR specifically linked to an issue
// claudeOutput parameter contains the implementation output from Claude CLI
// repoDir is the directory where the repository is cloned
func (w *ImplementationWorkflow) CreatePullRequestForIssue(owner, repo, branch, base string, issueNumber int, claudeOutput string, repoDir string) (*github.PullRequest, error) {
	return w.implementationService.CreatePullRequestForIssue(owner, repo, branch, base, issueNumber, claudeOutput, repoDir)
}

// RespondToIssue posts a comment to a GitHub issue
func (w *ImplementationWorkflow) RespondToIssue(owner, repo string, issueNumber int, comment string) error {
	_, err := w.implementationService.RespondToIssue(owner, repo, issueNumber, comment)
	return err
}

// CreateAndImplementIssue creates a branch, implementation plan, and executes it
// Returns the Claude CLI output for use in PR description
func CreateAndImplementIssue(cfg *config.Config, owner, repo string, issueNumber int, title, body string) (string, error) {
	// Create workflow
	workflow := NewImplementationWorkflow(cfg)

	// Generate branch name and PR title
	branchName, _, err := workflow.GenerateBranchAndTitle(owner, repo, title, body)
	if err != nil {
		return "", err
	}

	// Create and execute implementation plan
	return workflow.CreateImplementationPromptAndExecute(owner, repo, branchName, issueNumber)
}
