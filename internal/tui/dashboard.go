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
	title := titleStyle.Render("Fans")
	if len(d.fans) == 0 {
		return boxStyle.Width(40).Render(title + "\n" + dimStyle.Render("  No data"))
	}
	var lines []string
	for _, f := range d.fans {
		bar := renderBar(f.CurrentSpeed, 100, 20)
		auto := greenStyle.Render("auto")
		if !f.AutoControl {
			auto = yellowStyle.Render("manual")
		}
		line := fmt.Sprintf("  %-5s %5.1f%%  %s  %s  target: %.0f%%",
			f.Name, f.CurrentSpeed, bar, auto, f.TargetSpeed)
		lines = append(lines, line)
	}
	return boxStyle.Width(40).Render(title + "\n" + strings.Join(lines, "\n"))
}

func (d DashboardModel) renderMiniCurve() string {
	thresholds := d.config.FanConfigurations[0].TemperatureThresholds
	if len(thresholds) == 0 {
		return ""
	}

	title := titleStyle.Render("Fan Curve")
	rows := 8
	cols := 30
	grid := make([][]rune, rows)
	for i := range grid {
		grid[i] = make([]rune, cols)
		for j := range grid[i] {
			grid[i][j] = ' '
		}
	}

	// Plot the stepped curve
	for _, t := range thresholds {
		x := int(t.UpThreshold / 100 * float64(cols-1))
		y := rows - 1 - int(t.FanSpeed/100*float64(rows-1))
		if x >= 0 && x < cols && y >= 0 && y < rows {
			// Fill from this point right to next threshold or edge
			for xi := x; xi < cols; xi++ {
				if grid[y][xi] == ' ' || y < findLowestFilled(grid, xi) {
					grid[y][xi] = '█'
				}
			}
		}
	}

	// Build the chart
	var sb strings.Builder
	for i, row := range grid {
		pct := 100 - (i * 100 / (rows - 1))
		sb.WriteString(dimStyle.Render(fmt.Sprintf("%3d%%", pct)))
		sb.WriteString(dimStyle.Render("│"))
		for _, c := range row {
			if c == '█' {
				sb.WriteString(accentStyle.Render("█"))
			} else {
				sb.WriteRune(' ')
			}
		}
		sb.WriteString("\n")
	}
	sb.WriteString(dimStyle.Render("    └" + strings.Repeat("─", cols)))
	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("     0°        50°       100°"))

	return boxStyle.Render(title + "\n" + sb.String())
}

func findLowestFilled(grid [][]rune, col int) int {
	for i := len(grid) - 1; i >= 0; i-- {
		if grid[i][col] == '█' {
			return i
		}
	}
	return len(grid)
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
