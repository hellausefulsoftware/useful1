package tui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hellausefulsoftware/useful1/internal/cli"
	"github.com/hellausefulsoftware/useful1/internal/github"
	"github.com/hellausefulsoftware/useful1/internal/logging"
)

// Repository represents a GitHub repository for display
type Repository struct {
	FullName      string
	Description   string
	Selected      bool
	HasIssues     bool
	Stars         int
	ForksCount    int
	DefaultBranch string
}

// MonitorScreen is the screen for monitoring GitHub issues
type MonitorScreen struct {
	BaseScreen
	spinner          spinner.Model
	executor         *cli.Executor
	monitor          *github.Monitor
	running          bool
	runOnce          bool
	pollTime         time.Time
	nextPoll         time.Time
	logs             []string
	err              error
	repositories     []Repository
	cursor           int
	repoListVisible  bool
	showCreatePRForm bool
	fetchingRepos    bool
	selectAllRepos   bool
}

// Issue represents a GitHub issue for display
type Issue struct {
	ID     int
	Title  string
	URL    string
	User   string
	Status string
}

// NewMonitorScreen creates a new monitor screen
func NewMonitorScreen(app *App) *MonitorScreen {
	s := spinner.New()
	s.Spinner = spinner.Dot

	var executor *cli.Executor
	if app.GetConfig() != nil {
		executor = cli.NewExecutor(app.GetConfig())
	}

	var monitor *github.Monitor
	if app.GetConfig() != nil && executor != nil {
		monitor = github.NewMonitor(app.GetConfig(), executor)
	}

	screen := &MonitorScreen{
		BaseScreen:       NewBaseScreen(app, "Monitor GitHub Issues"),
		spinner:          s,
		executor:         executor,
		monitor:          monitor,
		running:          false,
		logs:             []string{},
		repositories:     []Repository{},
		cursor:           0,
		repoListVisible:  true,
		showCreatePRForm: false,
		fetchingRepos:    false,
		selectAllRepos:   false,
	}

	// Queue loading repositories if the client is available
	if app.GetConfig() != nil && executor != nil {
		screen.fetchingRepos = true
	}

	return screen
}

// Init initializes the monitor screen
func (m *MonitorScreen) Init() tea.Cmd {
	m.running = false
	m.err = nil

	var cmds []tea.Cmd
	cmds = append(cmds, m.spinner.Tick)

	// Fetch repositories if needed
	if m.fetchingRepos {
		cmds = append(cmds, m.fetchRepositories)
	}

	return tea.Batch(cmds...)
}

// fetchRepositories fetches repositories the user has access to
func (m *MonitorScreen) fetchRepositories() tea.Msg {
	if m.executor == nil || m.executor.GetGitHubClient() == nil {
		return fetchRepositoriesMsg{
			repos: nil,
			err:   fmt.Errorf("GitHub client not configured"),
		}
	}

	// Fetch repositories
	ghRepos, err := m.executor.GetGitHubClient().GetRepositories()
	if err != nil {
		return fetchRepositoriesMsg{
			repos: nil,
			err:   err,
		}
	}

	// Convert to our Repository type
	repos := make([]Repository, 0, len(ghRepos))
	for _, repo := range ghRepos {
		if repo.GetName() == "" || repo.GetOwner() == nil {
			continue
		}

		repos = append(repos, Repository{
			FullName:      repo.GetFullName(),
			Description:   repo.GetDescription(),
			Selected:      false,
			HasIssues:     repo.GetHasIssues(),
			Stars:         repo.GetStargazersCount(),
			ForksCount:    repo.GetForksCount(),
			DefaultBranch: repo.GetDefaultBranch(),
		})
	}

	return fetchRepositoriesMsg{
		repos: repos,
		err:   nil,
	}
}

// Update handles UI updates for the monitor screen
func (m *MonitorScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case fetchRepositoriesMsg:
		m.fetchingRepos = false
		if msg.err != nil {
			m.logs = append(m.logs, "Error fetching repositories: "+msg.err.Error())
		} else {
			m.repositories = msg.repos
			if len(m.repositories) > 0 {
				m.logs = append(m.logs, fmt.Sprintf("Loaded %d repositories", len(m.repositories)))
			} else {
				m.logs = append(m.logs, "No repositories found")
			}
		}
		return m, nil

	case tea.KeyMsg:
		// If in repository selection mode
		if m.repoListVisible && !m.running {
			switch {
			case key.Matches(msg, m.app.keyMap.Back):
				// Go back to main menu
				return m, m.app.ChangeScreen(ScreenMainMenu)

			case key.Matches(msg, m.app.keyMap.Up):
				// Move cursor up in repo list
				if m.cursor > 0 {
					m.cursor--
				} else if len(m.repositories) > 0 {
					// Wrap to bottom
					m.cursor = len(m.repositories)
				}
				return m, nil

			case key.Matches(msg, m.app.keyMap.Down):
				// Move cursor down in repo list
				if m.cursor < len(m.repositories) {
					m.cursor++
				} else {
					// Wrap to top
					m.cursor = 0
				}
				return m, nil

			case key.Matches(msg, m.app.keyMap.Select):
				// Select/deselect a repository or handle special options
				if m.cursor == len(m.repositories) {
					// "Select All" option
					m.selectAllRepos = !m.selectAllRepos
					// Apply to all repositories
					for i := range m.repositories {
						m.repositories[i].Selected = m.selectAllRepos
					}
				} else if m.cursor < len(m.repositories) {
					// Toggle selection for the current repository
					m.repositories[m.cursor].Selected = !m.repositories[m.cursor].Selected

					// Update selectAllRepos based on selection state
					allSelected := true
					for _, repo := range m.repositories {
						if !repo.Selected {
							allSelected = false
							break
						}
					}
					m.selectAllRepos = allSelected
				}
				return m, nil

			case key.Matches(msg, m.app.keyMap.Execute):
				// Launch monitoring for selected repositories
				selectedCount := 0
				var selectedRepos []string
				for _, repo := range m.repositories {
					if repo.Selected {
						selectedCount++
						selectedRepos = append(selectedRepos, repo.FullName)
					}
				}

				if selectedCount > 0 {
					// Update the config with selected repositories
					m.app.GetConfig().Monitor.RepoFilter = selectedRepos

					m.logs = append(m.logs, fmt.Sprintf("Set repository filter to %d repositories", selectedCount))
					m.logs = append(m.logs, "Monitoring assigned issues only")

					// Update the monitor with the new config
					if m.monitor != nil {
						m.monitor = github.NewMonitor(m.app.GetConfig(), m.executor)
						m.logs = append(m.logs, "Updated monitor with new settings")
					}

					m.running = true
					m.runOnce = false
					m.logs = append(m.logs, fmt.Sprintf("Starting continuous monitoring for %d repositories...", selectedCount))
					m.pollTime = time.Now()
					m.nextPoll = m.pollTime.Add(time.Duration(m.app.GetConfig().Monitor.PollInterval) * time.Minute)
					return m, m.checkIssues()
				} else {
					m.logs = append(m.logs, "Please select at least one repository first")
					return m, nil
				}
			}
		} else {
			// Normal monitoring mode
			switch {
			case key.Matches(msg, m.app.keyMap.Back):
				// Stop monitoring and go back
				m.running = false
				return m, nil

			case key.Matches(msg, m.app.keyMap.Execute):
				if !m.running {
					// Start monitoring
					// If no repos are explicitly selected but we have a repo filter, use that
					if len(m.app.GetConfig().Monitor.RepoFilter) == 0 {
						m.logs = append(m.logs, "Warning: No repository filter set, will monitor all repositories")
					} else {
						m.logs = append(m.logs, fmt.Sprintf("Using filter with %d repositories",
							len(m.app.GetConfig().Monitor.RepoFilter)))
					}

					// Always monitor assigned issues only
					m.app.GetConfig().Monitor.AssignedIssuesOnly = true
					m.logs = append(m.logs, "Monitoring assigned issues only")

					// Log the poll interval in seconds
					pollIntervalSecs := m.app.GetConfig().Monitor.PollInterval * 60
					m.logs = append(m.logs, fmt.Sprintf("Poll interval: %d seconds", pollIntervalSecs))

					// Update the monitor with the new settings
					if m.monitor != nil {
						m.monitor = github.NewMonitor(m.app.GetConfig(), m.executor)
					}

					m.running = true
					m.runOnce = false
					m.logs = append(m.logs, "Starting continuous monitoring...")
					m.pollTime = time.Now()
					m.nextPoll = m.pollTime.Add(time.Duration(m.app.GetConfig().Monitor.PollInterval) * time.Minute)
					return m, m.checkIssues()
				}

			case key.Matches(msg, m.app.keyMap.Select):
				if !m.running {
					// Run once
					// Report repository filter settings
					if len(m.app.GetConfig().Monitor.RepoFilter) == 0 {
						m.logs = append(m.logs, "Warning: No repository filter set, will check all repositories")
					} else {
						m.logs = append(m.logs, fmt.Sprintf("Using filter with %d repositories",
							len(m.app.GetConfig().Monitor.RepoFilter)))
					}

					// Always check assigned issues only
					m.app.GetConfig().Monitor.AssignedIssuesOnly = true
					m.logs = append(m.logs, "Checking assigned issues only")

					// Log the poll interval in seconds
					pollIntervalSecs := m.app.GetConfig().Monitor.PollInterval * 60
					m.logs = append(m.logs, fmt.Sprintf("Poll interval: %d seconds", pollIntervalSecs))

					// Update the monitor with the new setting
					if m.monitor != nil {
						m.monitor = github.NewMonitor(m.app.GetConfig(), m.executor)
					}

					m.running = true
					m.runOnce = true
					m.logs = append(m.logs, "Checking issues once...")
					return m, m.checkIssues()
				}
			}
		}

	case spinner.TickMsg:
		if m.running {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case monitorResultMsg:
		m.logs = append(m.logs, msg.log...)

		if m.runOnce {
			m.running = false
			m.logs = append(m.logs, "Check completed")
		} else {
			// Schedule next poll
			m.pollTime = time.Now()
			m.nextPoll = m.pollTime.Add(time.Duration(m.app.GetConfig().Monitor.PollInterval) * time.Minute)
			cmds = append(cmds, m.scheduleNextPoll())
		}

		if msg.err != nil {
			m.err = msg.err
			m.logs = append(m.logs, "Error: "+msg.err.Error())
			m.running = false
		}

	case tickMsg:
		if m.running && !m.runOnce {
			now := time.Now()
			if now.After(m.nextPoll) {
				return m, m.checkIssues()
			}
			return m, m.scheduleNextPoll()
		}
	}

	if m.running {
		spinnerCmd := m.spinner.Tick
		cmds = append(cmds, spinnerCmd)
	}

	return m, tea.Batch(cmds...)
}

// View renders the monitor screen
func (m *MonitorScreen) View() string {
	theme := m.app.GetTheme()
	content := m.RenderTitle() + "\n\n"

	// Repository list section (visible when not monitoring)
	if m.repoListVisible && !m.running {
		if m.fetchingRepos {
			content += m.spinner.View() + " Loading repositories...\n\n"
		} else if len(m.repositories) > 0 {
			content += theme.Subtitle.Render("Select Repositories:") + "\n\n"

			// Display repository list with checkbox selection
			// Calculate a reasonable width for the repo list box
			// Use the same approach as for the log section
			screenWidth := m.app.GetWidth()
			repoWidth := min(80, max(60, int(float64(screenWidth)*0.8)))
			
			repoListStyle := lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(theme.BorderColor)).
				Padding(1).
				Width(repoWidth)

			var repoList strings.Builder

			// Calculate how many repositories to show (limited by screen height)
			// Let's show 10 repositories at a time (or whatever fits)
			maxReposToShow := 10
			startIdx := 0

			// If cursor is below visible area, adjust startIdx
			if m.cursor >= maxReposToShow && m.cursor <= len(m.repositories) {
				startIdx = m.cursor - maxReposToShow + 1
			}

			endIdx := startIdx + maxReposToShow
			if endIdx > len(m.repositories) {
				endIdx = len(m.repositories)
			}

			// Add "Select All" option at the top
			selectAllText := "Select All"
			if m.selectAllRepos {
				selectAllText = "Deselect All"
			}

			cursor := " "
			checkbox := "[ ]"
			textStyle := theme.UnselectedItem

			if m.selectAllRepos {
				checkbox = "[×]"
			}

			if m.cursor == len(m.repositories) {
				cursor = ">"
				textStyle = theme.SelectedItem
			}

			repoList.WriteString(fmt.Sprintf("%s %s %s\n\n", cursor, checkbox, textStyle.Render(selectAllText)))

			// Display selected repositories
			for i := startIdx; i < endIdx; i++ {
				repo := m.repositories[i]
				cursor := " "
				checkbox := "[ ]"
				textStyle := theme.UnselectedItem

				if repo.Selected {
					checkbox = "[×]"
				}

				if m.cursor == i {
					cursor = ">"
					textStyle = theme.SelectedItem
				}

				line := fmt.Sprintf("%s %s %s", cursor, checkbox, textStyle.Render(repo.FullName))

				// Add stats/description if available
				if repo.Description != "" {
					desc := repo.Description
					if len(desc) > 50 {
						desc = desc[:47] + "..."
					}
					line += " - " + theme.Faint.Render(desc)
				}

				repoList.WriteString(line + "\n")
			}

			// Display pagination info if needed
			if len(m.repositories) > maxReposToShow {
				paginationInfo := fmt.Sprintf("Showing %d-%d of %d repositories",
					startIdx+1, endIdx, len(m.repositories))
				repoList.WriteString("\n" + theme.Faint.Render(paginationInfo))
			}

			content += repoListStyle.Render(repoList.String()) + "\n\n"

			// Instructions for repository selection
			content += theme.Text.Render("Press E to start monitoring selected repositories") + "\n"
			content += theme.Text.Render("Press Enter to select/deselect a repository") + "\n"
			content += theme.Text.Render("Use ↑/↓ to navigate") + "\n\n"

		} else {
			content += theme.Subtitle.Render("No repositories found") + "\n\n"
			content += theme.Text.Render("You don't have access to any GitHub repositories.") + "\n"
			content += theme.Text.Render("Please make sure your GitHub token has the correct permissions.") + "\n\n"
		}
	}

	// Monitoring status section
	if m.running {
		statusLine := m.spinner.View() + " "
		if m.runOnce {
			statusLine += "Checking issues once..."
		} else {
			nextPollIn := time.Until(m.nextPoll).Round(time.Second)
			statusLine += fmt.Sprintf("Monitoring (next poll in %s)", nextPollIn)
		}
		infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Info))
		content += infoStyle.Render(statusLine) + "\n\n"

		// Show which repositories are being monitored
		if len(m.repositories) > 0 {
			var selectedRepos []string
			for _, repo := range m.repositories {
				if repo.Selected {
					selectedRepos = append(selectedRepos, repo.FullName)
				}
			}

			if len(selectedRepos) > 0 {
				content += theme.Subtitle.Render("Monitoring Repositories:") + "\n"
				for i, repo := range selectedRepos {
					if i < 5 { // Show first 5 repos
						content += theme.Text.Render("• "+repo) + "\n"
					} else {
						content += theme.Text.Render(fmt.Sprintf("• ...and %d more", len(selectedRepos)-5)) + "\n"
						break
					}
				}
				content += "\n"
			}
		}
	} else if !m.repoListVisible {
		content += theme.Subtitle.Render("GitHub Issue Monitoring") + "\n\n"

		// Display monitoring mode and settings
		content += theme.Text.Render("Mode: Monitoring assigned issues only") + "\n"

		// Show poll interval
		pollIntervalSecs := m.app.GetConfig().Monitor.PollInterval * 60
		content += theme.Text.Render(fmt.Sprintf("Poll interval: %d seconds", pollIntervalSecs)) + "\n"

		content += theme.Text.Render("Press E to start continuous monitoring") + "\n"
		content += theme.Text.Render("Press Enter to check issues once") + "\n\n"
	}

	// Log section
	content += theme.Bold.Render("Activity Log:") + "\n\n"

	// Only show the last 10 log entries to prevent cluttering
	displayLogs := m.logs
	if len(displayLogs) > 10 {
		displayLogs = displayLogs[len(displayLogs)-10:]
	}

	logContent := strings.Join(displayLogs, "\n")
	// Calculate a reasonable width for the log box
	// On smaller screens, use a percentage of the screen width
	// On larger screens, cap at a reasonable width to prevent long lines
	screenWidth := m.app.GetWidth()
	logWidth := min(80, max(60, int(float64(screenWidth)*0.8)))
	
	logStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.BorderColor)).
		Padding(1).
		Width(logWidth)

	content += logStyle.Render(logContent) + "\n\n"

	// Instructions
	if !m.running {
		content += theme.Faint.Render("ESC to go back") + "\n\n"
	} else {
		content += theme.Faint.Render("ESC to stop monitoring and go back") + "\n\n"
	}

	// Footer
	content += m.RenderFooter()

	// Left-align in terminal
	return lipgloss.NewStyle().Width(m.app.GetWidth()).Align(lipgloss.Left).Render(content)
}

// ShortHelp returns keybindings to be shown in the help menu
func (m *MonitorScreen) ShortHelp() []key.Binding {
	if m.running {
		return []key.Binding{
			m.app.keyMap.Back,
			m.app.keyMap.Help,
			m.app.keyMap.Quit,
		}
	} else if m.repoListVisible {
		// Repository selection mode
		return []key.Binding{
			m.app.keyMap.Up,
			m.app.keyMap.Down,
			m.app.keyMap.Select,
			m.app.keyMap.Execute,
			m.app.keyMap.Back,
			m.app.keyMap.Help,
			m.app.keyMap.Quit,
		}
	} else {
		// Normal mode (not running)
		return []key.Binding{
			m.app.keyMap.Execute,
			m.app.keyMap.Select,
			m.app.keyMap.Back,
			m.app.keyMap.Help,
			m.app.keyMap.Quit,
		}
	}
}

// checkIssues checks GitHub issues
func (m *MonitorScreen) checkIssues() tea.Cmd {
	return func() tea.Msg {
		if m.monitor == nil {
			return monitorResultMsg{
				log: []string{"Monitoring not configured. Please run 'useful1 config' first."},
				err: fmt.Errorf("monitor not configured"),
			}
		}

		var logs []string
		var err error

		// Create a custom writer to capture log output
		logCapture := newLogCapture()

		// Setup a new logger that uses our custom writer
		originalOutput := m.app.GetConfig().Logging.Output
		m.app.GetConfig().Logging.Output = logCapture

		// Create proper logging config
		logConfig := &logging.Config{
			Level:      logging.LogLevel(m.app.GetConfig().Logging.Level),
			Output:     logCapture,
			JSONFormat: m.app.GetConfig().Logging.JSONFormat,
		}
		logging.Initialize(logConfig)

		if m.runOnce {
			logs = append(logs, "Running a one-time check for issues...")
			err = m.monitor.CheckOnce()
		} else {
			logs = append(logs, "Checking for new issues...")
			// Simulating a check that doesn't start continuous monitoring
			err = m.monitor.CheckOnce()
		}

		// Restore original logger
		origLogConfig := &logging.Config{
			Level:      logging.LogLevel(m.app.GetConfig().Logging.Level),
			Output:     originalOutput,
			JSONFormat: m.app.GetConfig().Logging.JSONFormat,
		}
		logging.Initialize(origLogConfig)

		// Add captured logs - get all important logs plus debug logs at higher verbosity
		capturedLogs := logCapture.GetImportantLogs()
		if len(capturedLogs) == 0 {
			// If no important logs were captured, add a general status message
			logs = append(logs, "GitHub check completed - no significant updates to report")
		} else {
			logs = append(logs, capturedLogs...)
		}

		if err != nil {
			logs = append(logs, "Error checking issues: "+err.Error())
		}

		return monitorResultMsg{
			log: logs,
			err: err,
		}
	}
}

// logCapture is a custom io.Writer that captures log output
type logCapture struct {
	lines []string
	mu    sync.Mutex
}

func newLogCapture() *logCapture {
	return &logCapture{
		lines: make([]string, 0),
	}
}

func (lc *logCapture) Write(p []byte) (n int, err error) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	// Convert bytes to string
	line := string(p)

	// Store the log line
	lc.lines = append(lc.lines, line)

	// Don't write to stdout - we'll display in the TUI instead
	// os.Stdout.Write(p)

	return len(p), nil
}

func (lc *logCapture) GetImportantLogs() []string {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	var important []string
	for _, line := range lc.lines {
		// Check for important log messages to capture - now including PR-related messages
		shouldCapture := strings.Contains(line, "Issues summary") ||
			strings.Contains(line, "Waiting before next check") ||
			strings.Contains(line, "draft PR") ||
			strings.Contains(line, "pull request") ||
			strings.Contains(line, "PR") ||
			strings.Contains(line, "branch") ||
			strings.Contains(line, "Issue") ||
			strings.Contains(line, "monitoring") ||
			strings.Contains(line, "Checking")

		if shouldCapture {
			// Clean up the log line (remove timestamp, level, etc.)
			// Try JSON format first
			if strings.Contains(line, "\"msg\":") {
				parts := strings.SplitN(line, "\"msg\":", 2)
				if len(parts) > 1 {
					msgPart := parts[1]
					// Extract the message value from JSON
					start := strings.Index(msgPart, "\"") + 1
					end := strings.Index(msgPart[start:], "\"")
					if start > 0 && end > 0 {
						logContent := msgPart[start : start+end]

						// For waiting message, format it nicely
						if strings.Contains(logContent, "Waiting before next check") {
							// Extract minutes value
							if strings.Contains(msgPart, "\"minutes\":") {
								minutesParts := strings.SplitN(msgPart, "\"minutes\":", 2)
								if len(minutesParts) > 1 {
									minutes := strings.TrimLeft(minutesParts[1], " ")
									end := strings.Index(minutes, ",")
									if end == -1 {
										end = strings.Index(minutes, "}")
									}
									if end > 0 {
										minutesVal := minutes[:end]
										logContent = fmt.Sprintf("Waiting %s minutes before next check", minutesVal)
									}
								}
							}
						}

						important = append(important, logContent)
					}
				}
			} else if strings.Contains(line, "msg=") {
				// Text format
				parts := strings.SplitN(line, "msg=", 2)
				if len(parts) > 1 {
					logContent := strings.Trim(parts[1], "\"")

					// For waiting message, format it nicely
					if strings.Contains(logContent, "Waiting before next check") {
						if strings.Contains(line, "minutes=") {
							minutesParts := strings.SplitN(line, "minutes=", 2)
							if len(minutesParts) > 1 {
								minutes := strings.TrimLeft(minutesParts[1], " ")
								end := strings.Index(minutes, " ")
								if end == -1 {
									end = len(minutes)
								}
								if end > 0 {
									minutesVal := minutes[:end]
									logContent = fmt.Sprintf("Waiting %s minutes before next check", minutesVal)
								}
							}
						}
					}

					important = append(important, logContent)
				}
			} else {
				// If we can't parse it, just add the whole line
				important = append(important, line)
			}
		}
	}

	return important
}

// scheduleNextPoll schedules the next poll
func (m *MonitorScreen) scheduleNextPoll() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// monitorResultMsg represents the result of monitoring
type monitorResultMsg struct {
	log []string
	err error
}

// tickMsg is a message sent when a tick occurs
type tickMsg time.Time

// fetchRepositoriesMsg is a message sent when repositories are fetched
type fetchRepositoriesMsg struct {
	repos []Repository
	err   error
}
