// Package vcs provides interfaces and implementations for version control system interactions
package vcs

import (
	"time"
)

// IssueComment represents a comment on an issue
type IssueComment struct {
	ID        string
	User      string
	Body      string
	CreatedAt time.Time
}

// Issue represents a generic issue across VCS platforms
type Issue interface {
	GetOwner() string
	GetRepo() string
	GetNumber() int
	GetTitle() string
	GetBody() string
	GetUser() string
	GetState() string
	GetCreatedAt() time.Time
	GetUpdatedAt() time.Time
	GetURL() string
	GetComments() []IssueComment
	GetLabels() []string
	GetAssignees() []string
}

// BaseIssue provides a common implementation of Issue
type BaseIssue struct {
	Owner     string
	Repo      string
	Number    int
	Title     string
	Body      string
	User      string
	State     string
	CreatedAt time.Time
	UpdatedAt time.Time
	URL       string
	Comments  []IssueComment
	Labels    []string
	Assignees []string
}

// GetOwner returns the repository owner
func (i *BaseIssue) GetOwner() string { return i.Owner }

// GetRepo returns the repository name
func (i *BaseIssue) GetRepo() string { return i.Repo }

// GetNumber returns the issue number
func (i *BaseIssue) GetNumber() int { return i.Number }

// GetTitle returns the issue title
func (i *BaseIssue) GetTitle() string { return i.Title }

// GetBody returns the issue body
func (i *BaseIssue) GetBody() string { return i.Body }

// GetUser returns the issue creator username
func (i *BaseIssue) GetUser() string { return i.User }

// GetState returns the issue state (open/closed)
func (i *BaseIssue) GetState() string { return i.State }

// GetCreatedAt returns the issue creation timestamp
func (i *BaseIssue) GetCreatedAt() time.Time { return i.CreatedAt }

// GetUpdatedAt returns when the issue was last updated
func (i *BaseIssue) GetUpdatedAt() time.Time { return i.UpdatedAt }

// GetURL returns the issue URL
func (i *BaseIssue) GetURL() string { return i.URL }

// GetComments returns the issue comments
func (i *BaseIssue) GetComments() []IssueComment { return i.Comments }

// GetLabels returns the issue labels
func (i *BaseIssue) GetLabels() []string { return i.Labels }

// GetAssignees returns usernames of people assigned to the issue
func (i *BaseIssue) GetAssignees() []string { return i.Assignees }
