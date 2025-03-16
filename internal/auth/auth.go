package auth

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/google/go-github/v45/github"
	"golang.org/x/oauth2"
)

// GitHubAuth holds GitHub authentication information
type GitHubAuth struct {
	Token string
	User  string
}

// AnthropicAuth holds Anthropic authentication information
type AnthropicAuth struct {
	Token string
}

// SetupGitHubOAuth guides the user through GitHub OAuth setup
func SetupGitHubOAuth() (*GitHubAuth, error) {
	reader := bufio.NewReader(os.Stdin)

	// Check if user already has a token
	fmt.Println("Do you already have a GitHub Personal Access Token? (y/n)")
	hasToken, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("error reading input: %w", err)
	}
	hasToken = strings.TrimSpace(strings.ToLower(hasToken))

	var token string
	if hasToken == "y" || hasToken == "yes" {
		// Use existing token
		fmt.Println("Enter your GitHub Personal Access Token:")
		token, err = reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("error reading token: %w", err)
		}
		token = strings.TrimSpace(token)
	} else {
		// Guide user to create a new token
		fmt.Println("Please create a new GitHub Personal Access Token:")
		fmt.Println("1. Go to https://github.com/settings/tokens")
		fmt.Println("2. Click 'Generate new token'")
		fmt.Println("3. Give it a name like 'Useful1 CLI'")
		fmt.Println("4. Select the following scopes: repo, workflow, read:org, notifications")
		fmt.Println("5. Click 'Generate token' and copy the token")
		fmt.Println("Enter the new token:")

		token, err = reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("error reading token: %w", err)
		}
		token = strings.TrimSpace(token)
	}

	// Validate the token
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)
	client := github.NewClient(tc)

	// Get the authenticated user to verify token
	user, resp, err := client.Users.Get(context.Background(), "")
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid GitHub token: %v", err)
	}

	fmt.Printf("✅ Successfully authenticated as: %s\n", *user.Login)

	// List accessible repositories to verify permissions
	opts := &github.RepositoryListOptions{
		ListOptions: github.ListOptions{PerPage: 1},
	}

	_, resp, err = client.Repositories.List(context.Background(), "", opts)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unable to list repositories: %v", err)
	}

	fmt.Println("✅ Successfully verified repository access")

	return &GitHubAuth{
		Token: token,
		User:  *user.Login,
	}, nil
}

// SetupAnthropicOAuth guides the user through Anthropic API setup
func SetupAnthropicOAuth() (*AnthropicAuth, error) {
	reader := bufio.NewReader(os.Stdin)

	// Check if user already has a token
	fmt.Println("Do you already have an Anthropic API Key? (y/n)")
	hasToken, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("error reading input: %w", err)
	}
	hasToken = strings.TrimSpace(strings.ToLower(hasToken))

	var token string
	if hasToken == "y" || hasToken == "yes" {
		// Use existing token
		fmt.Println("Enter your Anthropic API Key:")
		token, err = reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("error reading token: %w", err)
		}
		token = strings.TrimSpace(token)
	} else {
		// Guide user to create a new token
		fmt.Println("Please create a new Anthropic API Key:")
		fmt.Println("1. Go to https://console.anthropic.com/account/keys")
		fmt.Println("2. Click 'Create Key'")
		fmt.Println("3. Give it a name like 'Useful1 CLI'")
		fmt.Println("4. Copy the key")
		fmt.Println("Enter the new API key:")

		token, err = reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("error reading token: %w", err)
		}
		token = strings.TrimSpace(token)
	}

	// Validate the token with a simple API call
	if err := validateAnthropicKey(token); err != nil {
		return nil, fmt.Errorf("invalid Anthropic API key: %v", err)
	}

	fmt.Println("✅ Successfully verified Anthropic API key")

	return &AnthropicAuth{
		Token: token,
	}, nil
}

// validateAnthropicKey validates the Anthropic API key
func validateAnthropicKey(apiKey string) error {
	// Anthropic API endpoint
	url := "https://api.anthropic.com/v1/messages"

	// Simple request body
	requestBody, err := json.Marshal(map[string]interface{}{
		"model":      "claude-3-haiku-20240307",
		"max_tokens": 10,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": "Say hello",
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			err = fmt.Errorf("failed to close response body: %w", cerr)
		}
	}()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read error response: %w", err)
		}
		return fmt.Errorf("API request failed: %s - %s", resp.Status, string(body))
	}

	return nil
}
