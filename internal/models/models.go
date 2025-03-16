package models

import (
	"time"
)

// Issue represents a GitHub issue with all relevant data
type Issue struct {
	Owner        string
	Repo         string
	Number       int
	Title        string
	Body         string
	User         string
	State        string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Comments     []*IssueComment
	MentionsUser bool
	URL          string
	Labels       []string
	Assignees    []string
}

// IssueComment represents a comment on a GitHub issue
type IssueComment struct {
	ID        int64
	User      string
	Body      string
	CreatedAt time.Time
}

// IssueResponder defines the interface for responding to GitHub issues
type IssueResponder interface {
	RespondToIssueText(owner, repo string, issueNumber int, issueText string) error
}
