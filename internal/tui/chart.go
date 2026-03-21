package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mango/freshMango/internal/nbfc"
)

// ChartOptions configures the shared curve chart renderer.
type ChartOptions struct {
	// Title rendered at the top of the chart box.
	Title string
	// BoxWidth is the outer width of the box (including border+padding).
	BoxWidth int
	// Rows is the number of vertical cells in the chart grid.
	Rows int
	// Marker draws a crosshair at the given temperature/speed if non-nil.
	Marker *ChartMarker
}

// ChartMarker represents the current operating point shown as a crosshair.
type ChartMarker struct {
	Temperature  float64
	CurrentSpeed float64
}

// RenderCurveChart draws a temperature-to-fan-speed step chart from thresholds.
// The visual output is a filled-bar chart with optional crosshair overlay,
// x-axis labels (0°C–100°C), and an optional status annotation line.
func RenderCurveChart(thresholds []nbfc.Threshold, opts ChartOptions) string {
	rows := opts.Rows
	if rows < 2 {
		rows = 8
	}

	// Column count: box border (2) + padding (2) + y-axis labels (6+2=8) = 12 chars overhead.
	cols := opts.BoxWidth - 12
	if cols < 15 {
		cols = 15
	}

	// Build the boolean grid (rows × cols).
	grid := make([][]bool, rows)
	for i := range grid {
		grid[i] = make([]bool, cols)
	}

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

	// Resolve marker position (if provided).
	markerCol := -1
	markerRow := -1
	currentSpeed := 0.0
	if opts.Marker != nil && opts.Marker.Temperature > 0 {
		markerCol = int(opts.Marker.Temperature / 100.0 * float64(cols-1))
		if markerCol < 0 {
			markerCol = 0
		}
		if markerCol >= cols {
			markerCol = cols - 1
		}
		currentSpeed = opts.Marker.CurrentSpeed
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

			switch {
			case isMarkerCol && isMarkerRow:
				sb.WriteString(crosshairStyle.Render("◆"))
			case isMarkerCol:
				if grid[i][j] {
					sb.WriteString(crosshairStyle.Render("█"))
				} else {
					sb.WriteString(crosshairStyle.Render("┊"))
				}
			case isMarkerRow:
				if grid[i][j] {
					sb.WriteString(crosshairStyle.Render("█"))
				} else {
					sb.WriteString(crosshairStyle.Render("╌"))
				}
			case grid[i][j]:
				sb.WriteString(accentStyle.Render("█"))
			default:
				sb.WriteString(" ")
			}
		}
		sb.WriteString("\n")
	}

	// X-axis — 5 spaces to align with "%4d%%│" (5 visible chars + 1 border).
	sb.WriteString(dimStyle.Render("     └" + strings.Repeat("─", cols)))
	sb.WriteString("\n")

	// Build dynamic x-axis labels to match column count.
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
	sb.WriteString(dimStyle.Render("      " + string(buf)))

	// Status annotation line (only when marker is present).
	if opts.Marker != nil && opts.Marker.Temperature > 0 {
		sb.WriteString("\n")
		sb.WriteString(crosshairStyle.Render(fmt.Sprintf("      ▲ %.0f°C @ %.0f%%", opts.Marker.Temperature, currentSpeed)))
	}

	return boxStyle.Width(opts.BoxWidth - 4).Render(titleStyle.Render(opts.Title) + "\n" + sb.String())
}
