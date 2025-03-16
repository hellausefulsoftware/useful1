# Branch Naming in Draft PRs

This file documents how branch names are intelligently created for Draft PRs using keyword analysis.

## Implementation Details

The `generateBranchAndTitle` function in `client.go` now:

1. Extracts issue number from title (if present)
2. Analyzes the title and body text to determine issue type:
   - Classifies as "bugfix" if keywords like "bug", "fix", "issue", "problem" are present
   - Classifies as "chore" if keywords like "refactor", "clean", "doc" are present
   - Defaults to "feature" otherwise

3. Extracts meaningful keywords from the title by:
   - Filtering out common words (articles, prepositions, etc.)
   - Selecting 3-5 most meaningful terms
   - Sanitizing the terms for branch name compatibility

4. Constructs the branch name with:
   - Proper prefix based on issue type (bugfix/, feature/, chore/)
   - Issue number for easier tracking (if available)
   - Descriptive keywords for readability and clarity

5. Adds appropriate prefix to PR title:
   - "Fix:" for bugs
   - "Feature:" for features
   - "Chore:" for maintenance

## Examples

For an issue titled "Bug #123: Fix parsing error in JSON processor":
- Type: "bugfix" (detected from "bug" and "fix" keywords)
- Branch: "bugfix/issue-123-parsing-error-json-processor"
- Title: "Fix: Bug #123: Fix parsing error in JSON processor"

For an issue titled "Add support for new API endpoints":
- Type: "feature" (default)
- Branch: "feature/issue-456-support-api-endpoints"
- Title: "Feature: Add support for new API endpoints"

For an issue titled "Refactor authentication module":
- Type: "chore" (detected from "refactor" keyword)
- Branch: "chore/issue-789-authentication-module"
- Title: "Chore: Refactor authentication module"

## Benefits

This approach provides:
1. Consistent branch naming
2. Easy identification of issue type
3. Easy tracking via issue numbers
4. Clear description of changes via keywords
5. Improved merge request organization