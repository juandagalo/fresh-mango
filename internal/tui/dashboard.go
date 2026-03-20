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
		// Fan data is populated by App.refreshStatus() to avoid duplicate nbfc.Status() calls.
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
		sections = append(sections, "", d.renderFanCurves())
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

// fanCardLayout computes the card inner width and whether to stack vertically.
// Used by both renderFanCards() and renderFanCurves() so widths always match.
func (d DashboardModel) fanCardLayout(numFans int) (cardWidth int, stackVertically bool) {
	minCardWidth := 30
	gapWidth := 2
	contentWidth := d.contentWidth()

	if numFans == 1 {
		cardWidth = contentWidth - 4
		if cardWidth < minCardWidth {
			cardWidth = minCardWidth
		}
	} else {
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
	return
}

func (d DashboardModel) renderFanCards() string {
	numFans := len(d.fans)
	if numFans == 0 {
		return boxStyle.Width(40).Render(dimStyle.Render("No fan data"))
	}

	cardHeight := 5
	gapWidth := 2

	cardWidth, stackVertically := d.fanCardLayout(numFans)

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

func (d DashboardModel) renderFanCurves() string {
	// Build a list of fans that exist in both config and status
	type fanPair struct {
		cfg    nbfc.FanConfiguration
		status *nbfc.FanStatus
	}

	var pairs []fanPair
	for i, fc := range d.config.FanConfigurations {
		if len(fc.TemperatureThresholds) == 0 {
			continue
		}
		var st *nbfc.FanStatus
		if i < len(d.fans) {
			s := d.fans[i]
			st = &s
		}
		pairs = append(pairs, fanPair{cfg: fc, status: st})
	}

	if len(pairs) == 0 {
		return boxStyle.Width(40).Render(dimStyle.Render("No curve data"))
	}

	numFans := len(pairs)
	gapWidth := 2
	rows := 8

	// Use the same card width as renderFanCards so curves align with cards above
	cardWidth, stackVertically := d.fanCardLayout(numFans)
	// perChartWidth = inner cardWidth + border+padding (4) to match the outer box width
	perChartWidth := cardWidth + 4

	// Render one chart per fan
	var charts []string
	for _, fp := range pairs {
		chart := d.renderSingleCurve(fp.cfg, fp.status, perChartWidth, rows)
		charts = append(charts, chart)
	}

	if stackVertically || len(charts) == 1 {
		return lipgloss.JoinVertical(lipgloss.Left, charts...)
	}

	// Join side by side with gaps
	gap := strings.Repeat(" ", gapWidth)
	parts := []string{charts[0]}
	for i := 1; i < len(charts); i++ {
		parts = append(parts, gap, charts[i])
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (d DashboardModel) renderSingleCurve(fc nbfc.FanConfiguration, status *nbfc.FanStatus, perChartWidth, rows int) string {
	thresholds := fc.TemperatureThresholds

	// Title from fan display name
	displayName := fc.FanDisplayName
	if displayName == "" {
		displayName = "Fan"
	}
	title := titleStyle.Render(displayName + " Curve")

	// Column count: box border (2) + padding (2) + y-axis labels (6+2=8) = 12 chars overhead.
	cols := perChartWidth - 12
	if cols < 15 {
		cols = 15
	}

	grid := make([][]bool, rows)
	for i := range grid {
		grid[i] = make([]bool, cols)
	}

	// Each COLUMN is a temperature from 0°C to 100°C
	for col := 0; col < cols; col++ {
		temp := float64(col) / float64(cols-1) * 100.0
		speed := 0.0
		for _, t := range thresholds {
			if temp >= t.UpThreshold {
				speed = t.FanSpeed
			}
		}
		// Fill from bottom up to the speed level
		filledRows := int(speed / 100.0 * float64(rows))
		for r := rows - 1; r >= rows-filledRows; r-- {
			if r >= 0 {
				grid[r][col] = true
			}
		}
	}

	markerCol := -1
	markerRow := -1
	currentSpeed := 0.0
	if status != nil && status.Temperature > 0 {
		markerCol = int(status.Temperature / 100.0 * float64(cols-1))
		if markerCol < 0 {
			markerCol = 0
		}
		if markerCol >= cols {
			markerCol = cols - 1
		}
		currentSpeed = status.CurrentSpeed
		markerRow = rows - 1 - int(currentSpeed/100.0*float64(rows-1))
		if markerRow < 0 {
			markerRow = 0
		}
		if markerRow >= rows {
			markerRow = rows - 1
		}
	}

	crosshairStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4A7C75"))
	var sb strings.Builder
	for i := 0; i < rows; i++ {
		pct := 100 - (i * 100 / (rows - 1))
		sb.WriteString(dimStyle.Render(fmt.Sprintf("%4d%%", pct)))
		sb.WriteString(dimStyle.Render("│"))
		for j := 0; j < cols; j++ {
			isMarkerCol := j == markerCol && markerCol >= 0
			isMarkerRow := i == markerRow && markerRow >= 0

			if isMarkerCol && isMarkerRow {
				sb.WriteString(crosshairStyle.Render("◆"))
			} else if isMarkerCol {
				if grid[i][j] {
					sb.WriteString(crosshairStyle.Render("█"))
				} else {
					sb.WriteString(crosshairStyle.Render("┊"))
				}
			} else if isMarkerRow {
				if grid[i][j] {
					sb.WriteString(crosshairStyle.Render("█"))
				} else {
					sb.WriteString(crosshairStyle.Render("╌"))
				}
			} else if grid[i][j] {
				sb.WriteString(accentStyle.Render("█"))
			} else {
				sb.WriteString(" ")
			}
		}
		sb.WriteString("\n")
	}

	// X-axis — 5 spaces to align with "%4d%│" (5 visible chars + 1 border)
	sb.WriteString(dimStyle.Render("     └" + strings.Repeat("─", cols)))
	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("      0°C     50°C   100°C"))

	if status != nil && status.Temperature > 0 {
		sb.WriteString("\n")
		sb.WriteString(crosshairStyle.Render(fmt.Sprintf("      ▲ %.0f°C @ %.0f%%", status.Temperature, currentSpeed)))
	}

	// Use cardWidth (perChartWidth - 4) so boxStyle's own border+padding
	// produces the same outer width as the fan cards above.
	return boxStyle.Width(perChartWidth - 4).Render(title + "\n" + sb.String())
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
