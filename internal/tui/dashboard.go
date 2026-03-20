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
	sysInfo      *system.Info
	fans         []nbfc.FanStatus
	config       *nbfc.Config
	width, height int
	err          error
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
	var sections []string

	// System info
	sections = append(sections, d.renderSysInfo())

	// Temperature & Fans side by side
	tempSection := d.renderTemperature()
	fanSection := d.renderFans()
	cols := lipgloss.JoinHorizontal(lipgloss.Top, tempSection, "  ", fanSection)
	sections = append(sections, cols)

	// Mini curve
	if d.config != nil && len(d.config.FanConfigurations) > 0 {
		sections = append(sections, d.renderMiniCurve())
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

func (d DashboardModel) renderTemperature() string {
	title := titleStyle.Render("Temperature")
	if len(d.fans) == 0 {
		return boxStyle.Width(38).Render(title + "\n" + dimStyle.Render("  No data"))
	}
	temp := d.fans[0].Temperature
	bar := renderBar(temp, 100, 24)
	line := fmt.Sprintf("  CPU  %s  %s",
		tempColor(temp).Render(fmt.Sprintf("%5.1f°C", temp)),
		bar,
	)
	return boxStyle.Width(38).Render(title + "\n" + line)
}

func (d DashboardModel) renderFans() string {
	if len(d.fans) == 0 {
		return boxStyle.Width(40).Render(titleStyle.Render("Fans") + "\n" + dimStyle.Render("  No data"))
	}

	fanBoxWidth := 30
	var fanBoxes []string
	for _, f := range d.fans {
		bar := renderBar(f.CurrentSpeed, 100, 20)
		mode := greenStyle.Render("auto")
		if !f.AutoControl {
			mode = yellowStyle.Render("manual")
		}

		content := fmt.Sprintf("%s %s\n%s\n%s %s  %s %s",
			labelStyle.Render("Speed:"),
			valueStyle.Render(fmt.Sprintf("%.1f%%", f.CurrentSpeed)),
			bar,
			labelStyle.Render("Mode:"), mode,
			labelStyle.Render("Target:"), valueStyle.Render(fmt.Sprintf("%.0f%%", f.TargetSpeed)),
		)

		fanBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorderDim).
			Padding(0, 1).
			Width(fanBoxWidth).
			Render(titleStyle.Render(f.Name) + "\n" + content)

		fanBoxes = append(fanBoxes, fanBox)
	}

	// Side-by-side if terminal is wide enough, otherwise stack vertically
	minWidthForSideBySide := (fanBoxWidth+4)*len(fanBoxes) + 2*(len(fanBoxes)-1)
	if d.width > minWidthForSideBySide && len(fanBoxes) > 1 {
		return lipgloss.JoinHorizontal(lipgloss.Top, fanBoxes[0], "  ", strings.Join(fanBoxes[1:], "  "))
	}
	return lipgloss.JoinVertical(lipgloss.Left, fanBoxes...)
}

func (d DashboardModel) renderMiniCurve() string {
	thresholds := d.config.FanConfigurations[0].TemperatureThresholds
	if len(thresholds) == 0 {
		return ""
	}

	title := titleStyle.Render("Fan Curve")
	rows := 8
	cols := 30

	grid := make([][]bool, rows)
	for i := range grid {
		grid[i] = make([]bool, cols)
	}

	// For each column (temperature), find the fan speed from the curve
	// Same approach as curve editor renderChart()
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
	sb.WriteString(dimStyle.Render("     0°C      25°      50°      75°   100°"))

	return boxStyle.Render(title + "\n" + sb.String())
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
