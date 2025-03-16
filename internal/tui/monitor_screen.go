package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hellausefulsoftware/useful1/internal/cli"
	"github.com/hellausefulsoftware/useful1/internal/github"
)

// MonitorScreen is the screen for monitoring GitHub issues
type MonitorScreen struct {
	BaseScreen
	spinner     spinner.Model
	executor    *cli.Executor
	monitor     *github.Monitor
	running     bool
	runOnce     bool
	pollTime    time.Time
	nextPoll    time.Time
	issues      []Issue
	logs        []string
	err         error
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
	
	return &MonitorScreen{
		BaseScreen: NewBaseScreen(app, "Monitor GitHub Issues"),
		spinner:    s,
		executor:   executor,
		monitor:    monitor,
		running:    false,
		logs:       []string{},
	}
}

// Init initializes the monitor screen
func (m *MonitorScreen) Init() tea.Cmd {
	m.running = false
	m.err = nil
	return m.spinner.Tick
}

// Update handles UI updates for the monitor screen
func (m *MonitorScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.app.keyMap.Back):
			// Stop monitoring and go back
			m.running = false
			return m, m.app.ChangeScreen(ScreenMainMenu)
			
		case key.Matches(msg, m.app.keyMap.Execute):
			if !m.running {
				// Start monitoring
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
				m.running = true
				m.runOnce = true
				m.logs = append(m.logs, "Checking issues once...")
				return m, m.checkIssues()
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
				m.logs = append(m.logs, "Polling for new issues...")
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
	
	if m.running {
		statusLine := m.spinner.View() + " "
		if m.runOnce {
			statusLine += "Checking issues once..."
		} else {
			nextPollIn := m.nextPoll.Sub(time.Now()).Round(time.Second)
			statusLine += fmt.Sprintf("Monitoring (next poll in %s)", nextPollIn)
		}
		infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Info))
		content += infoStyle.Render(statusLine) + "\n\n"
	} else {
		content += theme.Subtitle.Render("GitHub Issue Monitoring") + "\n\n"
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
	logStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.BorderColor)).
		Padding(1).
		Width(m.app.GetWidth() - 4) // Account for padding and border
	
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
	}
	return []key.Binding{
		m.app.keyMap.Execute,
		m.app.keyMap.Select,
		m.app.keyMap.Back,
		m.app.keyMap.Help,
		m.app.keyMap.Quit,
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
		
		if m.runOnce {
			logs = append(logs, "Running a one-time check for issues...")
			err = m.monitor.CheckOnce()
		} else {
			logs = append(logs, "Checking for new issues...")
			// Simulating a check that doesn't start continuous monitoring
			err = m.monitor.CheckOnce()
		}
		
		if err != nil {
			logs = append(logs, "Error checking issues: "+err.Error())
		} else {
			logs = append(logs, "Successfully checked for issues")
		}
		
		return monitorResultMsg{
			log: logs,
			err: err,
		}
	}
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