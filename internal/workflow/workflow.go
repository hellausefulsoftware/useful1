// Package workflow provides implementation orchestration for issue resolution
package workflow

import (
	"github.com/google/go-github/v45/github"
	"github.com/hellausefulsoftware/useful1/internal/config"
	"github.com/hellausefulsoftware/useful1/internal/workflow/services"
)

// ImplementationWorkflow represents the complete implementation workflow
type ImplementationWorkflow struct {
	config *config.Config
	implementationService *services.GitHubImplementationService
}

// NewImplementationWorkflow creates a new implementation workflow
func NewImplementationWorkflow(cfg *config.Config) *ImplementationWorkflow {
	return &ImplementationWorkflow{
		config: cfg,
		implementationService: services.NewGitHubImplementationService(cfg),
	}
}

// NewImplementationWorkflowWithService creates a new implementation workflow with a provided service
func NewImplementationWorkflowWithService(cfg *config.Config, service *services.GitHubImplementationService) *ImplementationWorkflow {
	return &ImplementationWorkflow{
		config: cfg,
		implementationService: service,
	}
}

// GenerateBranchAndTitle generates a branch name and PR title for an issue
func (w *ImplementationWorkflow) GenerateBranchAndTitle(owner, repo, title, body string) (string, string, error) {
	return w.implementationService.GenerateBranchAndTitle(owner, repo, title, body)
}

// CreateImplementationPlan creates and executes an implementation plan
func (w *ImplementationWorkflow) CreateImplementationPlan(owner, repo, branchName string, issueNumber int) error {
	return w.implementationService.CreateImplementationPromptAndExecute(owner, repo, branchName, issueNumber)
}

// CreatePullRequest creates a new pull request with AI-generated description
func (w *ImplementationWorkflow) CreatePullRequest(owner, repo, branch, base, title string) (*github.PullRequest, error) {
	return w.implementationService.CreatePullRequest(owner, repo, branch, base, title)
}

// CreatePullRequestForIssue creates a PR specifically linked to an issue
func (w *ImplementationWorkflow) CreatePullRequestForIssue(owner, repo, branch, base string, issueNumber int) (*github.PullRequest, error) {
	return w.implementationService.CreatePullRequestForIssue(owner, repo, branch, base, issueNumber)
}

// RespondToIssue posts a comment to a GitHub issue
func (w *ImplementationWorkflow) RespondToIssue(owner, repo string, issueNumber int, comment string) error {
	return w.implementationService.RespondToIssue(owner, repo, issueNumber, comment)
}

// CreateAndImplementIssue creates a branch, implementation plan, and executes it
func CreateAndImplementIssue(cfg *config.Config, owner, repo string, issueNumber int, title, body string) error {
	// Create workflow
	workflow := NewImplementationWorkflow(cfg)
	
	// Generate branch name and PR title
	branchName, _, err := workflow.GenerateBranchAndTitle(owner, repo, title, body)
	if err != nil {
		return err
	}
	
	// Create and execute implementation plan
	return workflow.CreateImplementationPlan(owner, repo, branchName, issueNumber)
}