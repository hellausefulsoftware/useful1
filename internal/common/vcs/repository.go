// Package vcs provides interfaces and implementations for version control system interactions
package vcs

// Repository represents a generic repository across VCS platforms
type Repository interface {
	GetOwner() string
	GetName() string
	GetDefaultBranch() string
	GetURL() string
	GetDescription() string
	GetHasIssues() bool
	GetStargazersCount() int
	GetForksCount() int
}

// BaseRepository provides a common implementation of Repository
type BaseRepository struct {
	Owner           string
	Name            string
	DefaultBranch   string
	URL             string
	Description     string
	HasIssues       bool
	StargazersCount int
	ForksCount      int
}

// GetOwner returns the repository owner
func (r *BaseRepository) GetOwner() string { return r.Owner }

// GetName returns the repository name
func (r *BaseRepository) GetName() string { return r.Name }

// GetDefaultBranch returns the default branch name
func (r *BaseRepository) GetDefaultBranch() string { return r.DefaultBranch }

// GetURL returns the repository URL
func (r *BaseRepository) GetURL() string { return r.URL }

// GetDescription returns the repository description
func (r *BaseRepository) GetDescription() string { return r.Description }

// GetHasIssues returns whether the repository has issues enabled
func (r *BaseRepository) GetHasIssues() bool { return r.HasIssues }

// GetStargazersCount returns the number of stars
func (r *BaseRepository) GetStargazersCount() int { return r.StargazersCount }

// GetForksCount returns the number of forks
func (r *BaseRepository) GetForksCount() int { return r.ForksCount }
