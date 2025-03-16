package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// ColorblindFriendlyTheme defines a colorblind-friendly color scheme
// Using a palette that works well for all forms of color blindness
type ColorblindFriendlyTheme struct {
	// Primary colors
	Blue       lipgloss.Color
	Yellow     lipgloss.Color
	Black      lipgloss.Color
	White      lipgloss.Color
	DarkBlue   lipgloss.Color
	LightBlue  lipgloss.Color
	Orange     lipgloss.Color
	
	// Semantic colors (derived from primary)
	Success    lipgloss.Color
	Warning    lipgloss.Color
	Error      lipgloss.Color
	Info       lipgloss.Color
	Default    lipgloss.Color
	
	// Base styles
	Title      lipgloss.Style
	Subtitle   lipgloss.Style
	Text       lipgloss.Style
	Bold       lipgloss.Style
	Faint      lipgloss.Style
	
	// Component styles
	SelectedItem  lipgloss.Style
	UnselectedItem lipgloss.Style
	ActiveTab     lipgloss.Style
	InactiveTab   lipgloss.Style
	Button        lipgloss.Style
	SuccessButton lipgloss.Style
	CancelButton  lipgloss.Style
	BorderColor   lipgloss.Color
}

// NewTheme creates a new colorblind-friendly theme
func NewTheme() *ColorblindFriendlyTheme {
	t := &ColorblindFriendlyTheme{
		// Primary colors - colorblind friendly palette
		Blue:      "#0072B2", // Dark blue - distinctive in all color vision deficiencies
		Yellow:    "#E69F00", // Yellow - visible in most color vision deficiencies
		Black:     "#000000",
		White:     "#FFFFFF",
		DarkBlue:  "#004C99",
		LightBlue: "#56B4E9", // Light blue - visible in all types
		Orange:    "#D55E00", // Reddish/orange - visible in most types
		
		// Semantic colors
		Success:   "#009E73", // Bluish green - better than pure green for colorblindness
		Warning:   "#E69F00", // Yellow - good for warnings
		Error:     "#D55E00", // Red/orange - better than pure red for colorblindness
		Info:      "#0072B2", // Dark blue - good for info
		Default:   "#999999", // Gray - neutral
		
		BorderColor: "#56B4E9", // Light blue
	}
	
	// Initialize styles
	t.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(t.Blue)).
		MarginBottom(1)
		
	t.Subtitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.DarkBlue)).
		MarginBottom(1)
		
	t.Text = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Black))
		
	t.Bold = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(t.Black))
		
	t.Faint = lipgloss.NewStyle().
		Faint(true).
		Foreground(lipgloss.Color(t.Default))
	
	// Component styles
	t.SelectedItem = lipgloss.NewStyle().
		Bold(true).
		Background(lipgloss.Color(t.LightBlue)).
		Foreground(lipgloss.Color(t.White)).
		Padding(0, 1)
		
	t.UnselectedItem = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Black)).
		Padding(0, 1)
		
	t.ActiveTab = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(t.White)).
		Background(lipgloss.Color(t.Blue)).
		Padding(0, 3)
		
	t.InactiveTab = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Black)).
		Background(lipgloss.Color(t.Default)).
		Padding(0, 3)
		
	t.Button = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(t.White)).
		Background(lipgloss.Color(t.Blue)).
		Padding(0, 3).
		Margin(0, 1).
		BorderStyle(lipgloss.RoundedBorder())
		
	t.SuccessButton = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(t.White)).
		Background(lipgloss.Color(t.Success)).
		Padding(0, 3).
		Margin(0, 1).
		BorderStyle(lipgloss.RoundedBorder())
		
	t.CancelButton = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(t.White)).
		Background(lipgloss.Color(t.Orange)).
		Padding(0, 3).
		Margin(0, 1).
		BorderStyle(lipgloss.RoundedBorder())
		
	return t
}