package tui

import "github.com/charmbracelet/lipgloss"

// Color palette — Warm Ember theme.
var (
	// Core palette
	colorAccent    = lipgloss.Color("#F59E0B") // amber/orange — primary accent
	colorGreen     = lipgloss.Color("#4ADE80") // fresh green — success
	colorYellow    = lipgloss.Color("#FBBF24") // bright yellow-amber — warning
	colorRed       = lipgloss.Color("#EF4444") // clear red — error/hot
	colorSubtle    = lipgloss.Color("#78716C") // warm gray — muted text, labels
	colorText      = lipgloss.Color("#C0CAF5") // cool lavender-white — primary text
	colorTabBg     = lipgloss.Color("#292524") // warm near-black — surface
	colorActiveBg  = lipgloss.Color("#3D2F1E") // dark amber tint — active surface
	colorDimText   = lipgloss.Color("#44403C") // dark warm gray — disabled/dim
	colorBorderDim = lipgloss.Color("#44403C") // warm gray — subtle borders

	statusBarStyle = lipgloss.NewStyle().
			Foreground(colorText).
			Background(colorTabBg).
			Padding(0, 1)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorderDim).
			Padding(0, 1)

	titleStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	labelStyle = lipgloss.NewStyle().
			Foreground(colorSubtle)

	valueStyle = lipgloss.NewStyle().
			Foreground(colorText)

	greenStyle  = lipgloss.NewStyle().Foreground(colorGreen)
	yellowStyle = lipgloss.NewStyle().Foreground(colorYellow)
	redStyle    = lipgloss.NewStyle().Foreground(colorRed)
	accentStyle = lipgloss.NewStyle().Foreground(colorAccent)
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
