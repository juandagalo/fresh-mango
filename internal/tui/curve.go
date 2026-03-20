package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mango/freshMango/internal/nbfc"
)

type CurveEditorModel struct {
	config        *nbfc.Config
	fanIdx        int
	selectedRow   int
	selectedCol   int
	editing       bool
	editBuf       string
	width, height int
	statusMsg     string
	err           error
}

type configLoadedMsg struct {
	config *nbfc.Config
	err    error
}

type configSavedMsg struct{ err error }
type configRestoredMsg struct{ err error }

func NewCurveEditor() CurveEditorModel {
	return CurveEditorModel{}
}

func (c CurveEditorModel) loadConfig() tea.Cmd {
	return func() tea.Msg {
		cfgName, err := nbfc.GetSelectedConfig()
		if err != nil || cfgName == "" {
			return configLoadedMsg{err: fmt.Errorf("no config selected")}
		}
		cfg, err := nbfc.ReadConfigFile(cfgName)
		return configLoadedMsg{config: cfg, err: err}
	}
}

func (c CurveEditorModel) Update(msg tea.Msg) (CurveEditorModel, tea.Cmd) {
	switch msg := msg.(type) {
	case configLoadedMsg:
		if msg.err != nil {
			c.err = msg.err
		} else {
			c.config = msg.config
			c.err = nil
			c.statusMsg = "Config loaded"
		}
		return c, nil

	case configSavedMsg:
		if msg.err != nil {
			c.statusMsg = "⚠ Save failed: " + msg.err.Error()
		} else {
			c.statusMsg = "Saved & restarted nbfc"
		}
		return c, nil

	case configRestoredMsg:
		if msg.err != nil {
			c.statusMsg = "Restore failed: " + msg.err.Error()
		} else {
			c.statusMsg = "Backup restored & restarted nbfc"
		}
		return c, nil

	case tea.KeyMsg:
		if c.config == nil {
			if msg.String() == "r" {
				return c, c.loadConfig()
			}
			return c, nil
		}

		thresholds := c.thresholds()

		if c.editing {
			return c.handleEditing(msg)
		}

		switch msg.String() {
		case "up", "k":
			if c.selectedRow > 0 {
				c.selectedRow--
			}
		case "down", "j":
			if c.selectedRow < len(thresholds)-1 {
				c.selectedRow++
			}
		case "left", "h":
			if c.selectedCol > 0 {
				c.selectedCol--
			}
		case "right", "l":
			if c.selectedCol < 2 {
				c.selectedCol++
			}
		case "enter":
			c.editing = true
			c.editBuf = c.currentCellValue()
		case "a":
			c.addRow()
			c.statusMsg = "Row added"
		case "d":
			c.deleteRow()
			c.statusMsg = "Row deleted"
		case "s":
			return c, c.save()
		case "b":
			return c, c.restoreBackup()
		case "r":
			return c, c.loadConfig()
		case "f":
			if c.config != nil && len(c.config.FanConfigurations) > 1 {
				c.fanIdx = (c.fanIdx + 1) % len(c.config.FanConfigurations)
				c.selectedRow = 0
				c.statusMsg = fmt.Sprintf("Switched to %s", c.config.FanConfigurations[c.fanIdx].FanDisplayName)
			}
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			num, _ := strconv.Atoi(msg.String())
			idx := num - 1
			if c.config != nil && idx < len(c.config.FanConfigurations) {
				c.fanIdx = idx
				c.selectedRow = 0
				c.statusMsg = fmt.Sprintf("Switched to %s", c.config.FanConfigurations[c.fanIdx].FanDisplayName)
			}
		}
	}
	return c, nil
}

func (c *CurveEditorModel) handleEditing(msg tea.KeyMsg) (CurveEditorModel, tea.Cmd) {
	switch msg.String() {
	case "enter":
		c.applyCellValue()
		c.editing = false
		c.editBuf = ""
	case "escape":
		c.editing = false
		c.editBuf = ""
	case "backspace":
		if len(c.editBuf) > 0 {
			c.editBuf = c.editBuf[:len(c.editBuf)-1]
		}
	default:
		ch := msg.String()
		if len(ch) == 1 && (ch[0] >= '0' && ch[0] <= '9' || ch[0] == '.') {
			c.editBuf += ch
		}
	}
	return *c, nil
}

func (c CurveEditorModel) thresholds() []nbfc.Threshold {
	if c.config == nil || len(c.config.FanConfigurations) == 0 {
		return nil
	}
	return c.config.FanConfigurations[c.fanIdx].TemperatureThresholds
}

func (c CurveEditorModel) currentCellValue() string {
	t := c.thresholds()
	if c.selectedRow >= len(t) {
		return ""
	}
	row := t[c.selectedRow]
	switch c.selectedCol {
	case 0:
		return fmt.Sprintf("%.0f", row.UpThreshold)
	case 1:
		return fmt.Sprintf("%.0f", row.DownThreshold)
	case 2:
		return fmt.Sprintf("%.0f", row.FanSpeed)
	}
	return ""
}

func (c *CurveEditorModel) applyCellValue() {
	val, err := strconv.ParseFloat(c.editBuf, 64)
	if err != nil {
		return
	}
	t := &c.config.FanConfigurations[c.fanIdx].TemperatureThresholds[c.selectedRow]
	switch c.selectedCol {
	case 0:
		t.UpThreshold = val
	case 1:
		t.DownThreshold = val
	case 2:
		t.FanSpeed = val
	}
	c.sortThresholds()
}

func (c *CurveEditorModel) sortThresholds() {
	fan := &c.config.FanConfigurations[c.fanIdx]
	sort.Slice(fan.TemperatureThresholds, func(i, j int) bool {
		return fan.TemperatureThresholds[i].UpThreshold < fan.TemperatureThresholds[j].UpThreshold
	})
	if c.selectedRow >= len(fan.TemperatureThresholds) {
		c.selectedRow = len(fan.TemperatureThresholds) - 1
	}
}

func (c *CurveEditorModel) addRow() {
	fan := &c.config.FanConfigurations[c.fanIdx]
	newT := nbfc.Threshold{UpThreshold: 75, DownThreshold: 70, FanSpeed: 50}
	fan.TemperatureThresholds = append(fan.TemperatureThresholds, newT)
	c.sortThresholds()
	// Select the newly added row (will be at its sorted position)
	for i, t := range fan.TemperatureThresholds {
		if t.UpThreshold == newT.UpThreshold && t.FanSpeed == newT.FanSpeed {
			c.selectedRow = i
			break
		}
	}
}

func (c *CurveEditorModel) deleteRow() {
	fan := &c.config.FanConfigurations[c.fanIdx]
	if len(fan.TemperatureThresholds) <= 1 {
		return
	}
	idx := c.selectedRow
	fan.TemperatureThresholds = append(
		fan.TemperatureThresholds[:idx],
		fan.TemperatureThresholds[idx+1:]...,
	)
	if c.selectedRow >= len(fan.TemperatureThresholds) {
		c.selectedRow = len(fan.TemperatureThresholds) - 1
	}
}

func (c CurveEditorModel) save() tea.Cmd {
	return func() tea.Msg {
		return configSavedMsg{err: nbfc.SaveAndRestart(c.config)}
	}
}

func (c CurveEditorModel) restoreBackup() tea.Cmd {
	return func() tea.Msg {
		if c.config == nil {
			return configRestoredMsg{err: fmt.Errorf("no config loaded")}
		}
		return configRestoredMsg{err: nbfc.RestoreBackup(c.config.NotebookModel)}
	}
}

func (c CurveEditorModel) View() string {
	if c.config == nil {
		msg := "No config loaded."
		if c.err != nil {
			msg += " " + c.err.Error()
		}
		return dimStyle.Render(msg) + "\n\n" + dimStyle.Render("Press 'r' to reload")
	}

	fanName := ""
	if c.fanIdx < len(c.config.FanConfigurations) {
		fanName = c.config.FanConfigurations[c.fanIdx].FanDisplayName
	}
	header := titleStyle.Render(fmt.Sprintf("Curve Editor — %s (%s)", c.config.NotebookModel, fanName))

	table := c.renderTable()
	chart := c.renderChart()

	cols := lipgloss.JoinHorizontal(lipgloss.Top, table, "   ", chart)

	fanBar := c.renderFanSelector()

	help := dimStyle.Render("↑↓: navigate  ←→: columns  enter: edit  a: add  d: delete  s: save  b: restore backup  r: reload  f/1-9: fan")
	status := ""
	if c.statusMsg != "" {
		status = accentStyle.Render(c.statusMsg)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, fanBar, cols, "", help, status)
}

func (c CurveEditorModel) renderFanSelector() string {
	if c.config == nil || len(c.config.FanConfigurations) <= 1 {
		return ""
	}
	activeFanStyle := accentStyle.Bold(true)
	var parts []string
	for i, fan := range c.config.FanConfigurations {
		name := fan.FanDisplayName
		if i == c.fanIdx {
			parts = append(parts, activeFanStyle.Render("▸ "+name))
		} else {
			parts = append(parts, dimStyle.Render(name))
		}
	}
	return strings.Join(parts, "    ")
}

func (c CurveEditorModel) renderTable() string {
	thresholds := c.thresholds()
	headers := []string{"Start °C", "Stop °C", "Speed %"}
	colW := []int{10, 10, 10}

	var sb strings.Builder
	sb.WriteString(dimStyle.Render("  # "))
	for i, h := range headers {
		sb.WriteString(dimStyle.Render(fmt.Sprintf("%-*s", colW[i], h)))
	}
	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("  " + strings.Repeat("─", 34)))
	sb.WriteString("\n")

	for i, t := range thresholds {
		vals := []string{
			fmt.Sprintf("%.0f", t.UpThreshold),
			fmt.Sprintf("%.0f", t.DownThreshold),
			fmt.Sprintf("%.0f", t.FanSpeed),
		}
		selected := i == c.selectedRow

		if selected {
			sb.WriteString(accentStyle.Render(fmt.Sprintf("▸ %d ", i+1)))
		} else {
			sb.WriteString(dimStyle.Render(fmt.Sprintf("  %d ", i+1)))
		}

		for j, v := range vals {
			cell := fmt.Sprintf("%-*s", colW[j], v)
			if selected && j == c.selectedCol {
				if c.editing {
					// Show edit buffer with cursor
					display := c.editBuf + "▏"
					cell = fmt.Sprintf("%-*s", colW[j], display)
					sb.WriteString(lipgloss.NewStyle().
						Background(colorActiveBg).
						Foreground(colorAccent).
						Render(cell))
				} else {
					sb.WriteString(lipgloss.NewStyle().
						Background(colorActiveBg).
						Foreground(colorText).
						Render(cell))
				}
			} else if selected {
				sb.WriteString(accentStyle.Render(cell))
			} else {
				sb.WriteString(valueStyle.Render(cell))
			}
		}
		sb.WriteString("\n")
	}

	return boxStyle.Width(40).Render(sb.String())
}

func (c CurveEditorModel) renderChart() string {
	thresholds := c.thresholds()
	rows := 12

	// Calculate chart width dynamically based on terminal width
	// Table is boxStyle.Width(40) = ~44 chars with borders, plus 3 chars gap
	chartBoxWidth := c.width - 48
	if chartBoxWidth < 40 {
		chartBoxWidth = 40
	}

	// cols = chart box inner width minus y-axis labels and box borders
	cols := chartBoxWidth - 12
	if cols < 30 {
		cols = 30
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
		filledRows := int(speed / 100.0 * float64(rows))
		for r := rows - 1; r >= rows-filledRows; r-- {
			if r >= 0 {
				grid[r][col] = true
			}
		}
	}

	var sb strings.Builder
	for i := 0; i < rows; i++ {
		pct := 100 - (i * 100 / (rows - 1))
		sb.WriteString(dimStyle.Render(fmt.Sprintf("%4d%%", pct)))
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
	sb.WriteString(dimStyle.Render("     └" + strings.Repeat("─", cols)))
	sb.WriteString("\n")

	// Build dynamic x-axis labels to match column count
	labelLine := "      "
	positions := []struct {
		label string
		col   int
	}{
		{"0°C", 0},
		{"25°C", cols / 4},
		{"50°C", cols / 2},
		{"75°C", cols * 3 / 4},
		{"100°C", cols - 1},
	}
	buf := make([]rune, cols)
	for i := range buf {
		buf[i] = ' '
	}
	for _, p := range positions {
		pos := p.col
		for i, ch := range p.label {
			idx := pos + i
			if idx >= 0 && idx < cols {
				buf[idx] = ch
			}
		}
	}
	labelLine += string(buf)
	sb.WriteString(dimStyle.Render(labelLine))

	return boxStyle.Width(chartBoxWidth).Render(titleStyle.Render("Fan Curve") + "\n" + sb.String())
}
