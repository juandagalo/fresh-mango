package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mango/freshMango/internal/nbfc"
	"github.com/mango/freshMango/internal/system"
)

type DashboardModel struct {
	sysInfo       *system.Info
	fans          []nbfc.FanStatus
	config        *nbfc.Config
	width, height int
	err           error
}

func NewDashboard() DashboardModel {
	return DashboardModel{}
}

type sysInfoMsg *system.Info

func (d DashboardModel) Init() tea.Cmd {
	return func() tea.Msg {
		info, _ := system.Detect()
		return sysInfoMsg(info)
	}
}

func (d DashboardModel) Update(msg tea.Msg) (DashboardModel, tea.Cmd) {
	switch msg := msg.(type) {
	case sysInfoMsg:
		d.sysInfo = msg
	case tickMsg:
		fans, err := nbfc.Status()
		if err != nil {
			d.err = err
		} else {
			d.fans = fans
			d.err = nil
		}
		if d.config == nil {
			if cfgName, err := nbfc.GetSelectedConfig(); err == nil && cfgName != "" {
				if cfg, err := nbfc.ReadConfigFile(cfgName); err == nil {
					d.config = cfg
				}
			}
		}
	}
	return d, nil
}

func (d DashboardModel) View() string {
	header := d.renderSysInfo()
	fanCards := d.renderFanCards()

	sections := []string{header, "", fanCards}

	if d.config != nil && len(d.config.FanConfigurations) > 0 {
		sections = append(sections, "", d.renderMiniCurve())
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (d DashboardModel) renderSysInfo() string {
	if d.sysInfo == nil {
		return boxStyle.Render(dimStyle.Render("Detecting system..."))
	}
	s := d.sysInfo
	content := fmt.Sprintf("%s  %s  %s  %s (%d cores / %d threads)",
		titleStyle.Render("freshMango"),
		labelStyle.Render("Model:"), valueStyle.Render(s.ProductName),
		s.CPUModel, s.CPUCores, s.CPUThreads,
	)
	return content
}

func (d DashboardModel) contentWidth() int {
	w := d.width - 4
	if w < 40 {
		w = 40
	}
	return w
}

func (d DashboardModel) renderFanCards() string {
	numFans := len(d.fans)
	if numFans == 0 {
		return boxStyle.Width(40).Render(dimStyle.Render("No fan data"))
	}

	minCardWidth := 30
	cardHeight := 5
	gapWidth := 2

	// Use the same contentWidth as the curve box for consistent alignment
	contentWidth := d.contentWidth()

	// Determine card width and layout direction
	// Each bordered box with Padding(0,1) adds 4 chars: 2 border + 2 padding
	// Width() is set to the inner content width (excludes border+padding)
	var cardWidth int
	stackVertically := false

	if numFans == 1 {
		// Single fan: use the full content width
		cardWidth = contentWidth - 4
		if cardWidth < minCardWidth {
			cardWidth = minCardWidth
		}
	} else {
		// Multiple fans: divide contentWidth equally, accounting for gaps and border+padding
		totalGaps := gapWidth * (numFans - 1)
		cardWidth = (contentWidth-totalGaps)/numFans - 4
		if cardWidth < minCardWidth {
			stackVertically = true
			cardWidth = contentWidth - 4
			if cardWidth < minCardWidth {
				cardWidth = minCardWidth
			}
		}
	}

	// Build one card per fan
	var cards []string
	for _, f := range d.fans {
		// Temperature line
		var tempStr string
		if f.Temperature == 0 {
			tempStr = dimStyle.Render("N/A")
		} else {
			tempStr = tempColor(f.Temperature).Render(fmt.Sprintf("%.1f°C", f.Temperature))
		}

		// Speed bar — leave room for label and value text inside the card
		// cardWidth is now the inner content width directly
		barWidth := cardWidth - 20
		if barWidth < 8 {
			barWidth = 8
		}
		bar := renderBar(f.CurrentSpeed, 100, barWidth)

		// Mode and target
		mode := greenStyle.Render("auto")
		if !f.AutoControl {
			mode = yellowStyle.Render("manual")
		}

		content := fmt.Sprintf("  %s  %s\n  %s  %s   %s\n  %s  %s    %s %s",
			labelStyle.Render("Temp:"),
			tempStr,
			labelStyle.Render("Speed:"),
			valueStyle.Render(fmt.Sprintf("%5.1f%%", f.CurrentSpeed)),
			bar,
			labelStyle.Render("Mode:"),
			mode,
			labelStyle.Render("Target:"),
			valueStyle.Render(fmt.Sprintf("%.0f%%", f.TargetSpeed)),
		)

		card := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorderDim).
			Padding(0, 1).
			Width(cardWidth).
			Height(cardHeight).
			Render(titleStyle.Render(f.Name) + "\n" + content)

		cards = append(cards, card)
	}

	if stackVertically {
		return lipgloss.JoinVertical(lipgloss.Left, cards...)
	}

	// Join side-by-side with gaps
	if len(cards) == 1 {
		return cards[0]
	}

	gap := strings.Repeat(" ", gapWidth)
	parts := []string{cards[0]}
	for i := 1; i < len(cards); i++ {
		parts = append(parts, gap, cards[i])
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (d DashboardModel) renderMiniCurve() string {
	thresholds := d.config.FanConfigurations[0].TemperatureThresholds
	if len(thresholds) == 0 {
		return ""
	}

	title := titleStyle.Render("Fan Curve")
	rows := 8

	curveWidth := d.contentWidth()
	cols := curveWidth - 10
	if cols < 20 {
		cols = 20
	}

	grid := make([][]bool, rows)
	for i := range grid {
		grid[i] = make([]bool, cols)
	}

	// For each column (temperature), find the fan speed from the curve
	for col := 0; col < cols; col++ {
		temp := float64(col) / float64(cols-1) * 100.0
		speed := 0.0
		for _, t := range thresholds {
			if temp >= t.UpThreshold {
				speed = t.FanSpeed
			}
		}
		// Fill from bottom up to speed level
		filledRows := int(speed / 100.0 * float64(rows))
		for r := rows - 1; r >= rows-filledRows; r-- {
			if r >= 0 {
				grid[r][col] = true
			}
		}
	}

	// Build the chart
	var sb strings.Builder
	for i := 0; i < rows; i++ {
		pct := 100 - (i * 100 / (rows - 1))
		sb.WriteString(dimStyle.Render(fmt.Sprintf("%3d%%", pct)))
		sb.WriteString(dimStyle.Render("│"))
		for j := 0; j < cols; j++ {
			if grid[i][j] {
				sb.WriteString(accentStyle.Render("█"))
			} else {
				sb.WriteString(" ")
			}
		}
		sb.WriteString("\n")
	}
	sb.WriteString(dimStyle.Render("    └" + strings.Repeat("─", cols)))
	sb.WriteString("\n")

	// Build dynamic x-axis labels
	labelLine := "     "
	positions := []struct {
		label string
		col   int
	}{
		{"0°C", 0},
		{"25°", cols / 4},
		{"50°", cols / 2},
		{"75°", cols * 3 / 4},
		{"100°", cols - 1},
	}
	buf := make([]byte, cols)
	for i := range buf {
		buf[i] = ' '
	}
	for _, p := range positions {
		pos := p.col
		for i, ch := range p.label {
			idx := pos + i
			if idx >= 0 && idx < cols {
				buf[idx] = byte(ch)
			}
		}
	}
	labelLine += string(buf)
	sb.WriteString(dimStyle.Render(labelLine))

	return boxStyle.Width(curveWidth).Render(title + "\n" + sb.String())
}

func renderBar(value, max float64, width int) string {
	if max <= 0 {
		max = 100
	}
	filled := int(value / max * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	empty := width - filled

	var style lipgloss.Style
	switch {
	case value/max >= 0.8:
		style = redStyle
	case value/max >= 0.6:
		style = yellowStyle
	default:
		style = greenStyle
	}

	bar := style.Render(strings.Repeat("█", filled)) +
		dimStyle.Render(strings.Repeat("░", empty))
	return "[" + bar + "]"
}
