package monitoring

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// MonitorSettings holds configuration for issue monitoring
type MonitorSettings struct {
	PollInterval  int      // in minutes
	CheckMentions bool     // whether to check for mentions
	RepoFilter    []string // optional list of repositories to filter (empty for all)
}

// SetupMonitoring guides the user through monitoring configuration
func SetupMonitoring() (*MonitorSettings, error) {
	reader := bufio.NewReader(os.Stdin)
	settings := &MonitorSettings{
		PollInterval:  5, // default 5 minutes
		CheckMentions: true,
		RepoFilter:    []string{},
	}

	// Configure poll interval
	fmt.Println("How often should we check for new notifications/mentions? (minutes, default: 5)")
	intervalStr, _ := reader.ReadString('\n')
	intervalStr = strings.TrimSpace(intervalStr)

	if intervalStr != "" {
		interval, err := strconv.Atoi(intervalStr)
		if err != nil {
			return nil, fmt.Errorf("invalid poll interval: %v", err)
		}
		if interval < 1 {
			return nil, fmt.Errorf("poll interval must be at least 1 minute")
		}
		settings.PollInterval = interval
	}

	// Configure mentions monitoring
	fmt.Println("Do you want to monitor for mentions in issues? (y/n, default: y)")
	mentionsStr, _ := reader.ReadString('\n')
	mentionsStr = strings.TrimSpace(strings.ToLower(mentionsStr))

	if mentionsStr == "n" || mentionsStr == "no" {
		settings.CheckMentions = false
	}

	// Configure repository filtering
	fmt.Println("Do you want to filter specific repositories? (y/n, default: n)")
	fmt.Println("(If no, all accessible repositories will be monitored)")
	filterStr, _ := reader.ReadString('\n')
	filterStr = strings.TrimSpace(strings.ToLower(filterStr))

	if filterStr == "y" || filterStr == "yes" {
		fmt.Println("Enter repository names in the format 'owner/repo', one per line.")
		fmt.Println("Press Enter twice when done.")

		for {
			repo, _ := reader.ReadString('\n')
			repo = strings.TrimSpace(repo)

			if repo == "" {
				break
			}

			// Validate repository format
			if !strings.Contains(repo, "/") {
				fmt.Println("Invalid format. Please use 'owner/repo' format.")
				continue
			}

			settings.RepoFilter = append(settings.RepoFilter, repo)
		}
	}

	// Display settings summary
	fmt.Println("\nMonitoring Configuration Summary:")
	fmt.Printf("- Poll Interval: %d minutes\n", settings.PollInterval)
	fmt.Printf("- Check Mentions: %v\n", settings.CheckMentions)

	if len(settings.RepoFilter) > 0 {
		fmt.Println("- Monitoring only these repositories:")
		for _, repo := range settings.RepoFilter {
			fmt.Printf("  * %s\n", repo)
		}
	} else {
		fmt.Println("- Monitoring all accessible repositories")
	}

	// Confirm settings
	fmt.Println("\nAre these settings correct? (y/n)")
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(strings.ToLower(confirm))

	if confirm != "y" && confirm != "yes" {
		return nil, fmt.Errorf("monitoring configuration canceled")
	}

	return settings, nil
}
