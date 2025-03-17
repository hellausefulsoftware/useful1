// Package vcs provides interfaces and implementations for version control system interactions
package vcs

import (
	"time"
)

// Service defines the common interface for all VCS providers (GitHub, GitLab, etc.)
type Service interface {
	// Issue operations
	GetIssue(owner, repo string, number int) (Issue, error)
	GetIssueWithComments(owner, repo string, number int) (Issue, error)
	GetAssignedIssues(username string, since time.Time, limit int) ([]Issue, error)
	RespondToIssue(owner, repo string, issueNumber int, comment string) error
	
	// Repository operations
	GetRepository(owner, repo string) (Repository, error)
	GetDefaultBranch(owner, repo string) (string, error)
	CloneRepository(owner, repo, branch string, number int) (string, error)
	GetRepositories() ([]Repository, error) // Get all accessible repositories for the authenticated user
	
	// Branch operations
	CreateBranch(owner, repo, branchName, baseBranch string) error
	
	// PR operations
	CreateDraftPullRequest(owner, repo, title, body, head, base string) (PullRequest, error)
	GetPullRequestsForIssue(owner, repo string, issueNumber int) ([]PullRequest, error)
	
	// Authentication
	GetAuthenticatedUser() (string, error)
}

// ServiceProvider creates VCS service instances
type ServiceProvider interface {
	GetService() Service
}
