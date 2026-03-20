package tui

import "github.com/charmbracelet/lipgloss"

// Color palette — dark theme.
var (
	colorCyan     = lipgloss.Color("#00BCD4")
	colorGreen    = lipgloss.Color("#4CAF50")
	colorYellow   = lipgloss.Color("#FFC107")
	colorRed      = lipgloss.Color("#F44336")
	colorSubtle   = lipgloss.Color("#6C7086")
	colorText     = lipgloss.Color("#CDD6F4")
	colorTabBg    = lipgloss.Color("#313244")
	colorActiveBg = lipgloss.Color("#45475A")
	colorDimText  = lipgloss.Color("#585B70")

	tabStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Background(colorTabBg).
			Foreground(colorSubtle)

	activeTabStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Background(colorActiveBg).
			Foreground(colorCyan).
			Bold(true)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(colorText).
			Background(colorTabBg).
			Padding(0, 1)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorSubtle).
			Padding(0, 1)

	titleStyle = lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true)

	labelStyle = lipgloss.NewStyle().
			Foreground(colorSubtle)

	valueStyle = lipgloss.NewStyle().
			Foreground(colorText)

	greenStyle  = lipgloss.NewStyle().Foreground(colorGreen)
	yellowStyle = lipgloss.NewStyle().Foreground(colorYellow)
	redStyle    = lipgloss.NewStyle().Foreground(colorRed)
	cyanStyle   = lipgloss.NewStyle().Foreground(colorCyan)
	dimStyle    = lipgloss.NewStyle().Foreground(colorDimText)
)

func tempColor(temp float64) lipgloss.Style {
	switch {
	case temp >= 80:
		return redStyle
	case temp >= 60:
		return yellowStyle
	default:
		return greenStyle
	}
}
