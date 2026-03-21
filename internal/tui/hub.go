package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type menuItem struct {
	key   string // "1"-"6"
	label string
	desc  string
	view  viewID
	ready bool
}

var hubMenuItems = []menuItem{
	{key: "1", label: "Dashboard", desc: "Live monitoring & graphs", view: viewDashboard, ready: true},
	{key: "2", label: "Fan Control", desc: "Override speeds per fan", view: viewFanControl, ready: false},
	{key: "3", label: "Curve Editor", desc: "Edit temperature thresholds", view: viewCurveEditor, ready: true},
	{key: "4", label: "Profiles", desc: "Save/load configurations", view: viewProfiles, ready: false},
	{key: "5", label: "Sensors", desc: "Configure temperature sources", view: viewSensors, ready: false},
	{key: "6", label: "Settings", desc: "Service & configuration", view: viewInstaller, ready: false},
}

type HubModel struct {
	selectedItem  int
	configName    string
	serviceOn     bool
	cpuTemp       float64
	fanSpeeds     []float64
	width, height int
}

// hubSelectMsg is sent when a hub menu item is selected for navigation.
type hubSelectMsg struct{ id viewID }

func NewHub() HubModel {
	return HubModel{}
}

func (h HubModel) Init() tea.Cmd {
	return nil
}

func (h HubModel) Update(msg tea.Msg) (HubModel, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "up", "k":
			if h.selectedItem > 0 {
				h.selectedItem--
			}
		case "down", "j":
			if h.selectedItem < len(hubMenuItems)-1 {
				h.selectedItem++
			}
		case "enter":
			item := hubMenuItems[h.selectedItem]
			if item.ready {
				return h, func() tea.Msg { return hubSelectMsg{id: item.view} }
			}
		case "1", "2", "3", "4", "5", "6":
			idx := int(msg.String()[0]-'0') - 1
			if idx >= 0 && idx < len(hubMenuItems) {
				h.selectedItem = idx
				item := hubMenuItems[idx]
				if item.ready {
					return h, func() tea.Msg { return hubSelectMsg{id: item.view} }
				}
			}
		}
	}
	return h, nil
}

func (h HubModel) View() string {
	var sections []string

	header := accentStyle.Bold(true).Render("freshMango")
	sections = append(sections, header)
	sections = append(sections, "")

	summary := h.renderSummary()
	sections = append(sections, summary)
	sections = append(sections, "")

	menu := h.renderMenu()
	sections = append(sections, menu)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (h HubModel) renderSummary() string {
	cfg := h.configName
	if cfg == "" {
		cfg = dimStyle.Render("none")
	} else {
		cfg = valueStyle.Render(cfg)
	}

	svc := redStyle.Render("stopped")
	if h.serviceOn {
		svc = greenStyle.Render("running")
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("%s %s", labelStyle.Render("Config:"), cfg))
	parts = append(parts, fmt.Sprintf("%s %s", labelStyle.Render("Service:"), svc))

	if h.cpuTemp > 0 {
		tempStr := tempColor(h.cpuTemp).Render(fmt.Sprintf("%.1f\u00b0C", h.cpuTemp))
		parts = append(parts, fmt.Sprintf("%s %s", labelStyle.Render("CPU:"), tempStr))
	}

	if len(h.fanSpeeds) > 0 {
		var fanStrs []string
		for i, spd := range h.fanSpeeds {
			fanStrs = append(fanStrs, fmt.Sprintf("Fan%d: %.0f%%", i+1, spd))
		}
		parts = append(parts, labelStyle.Render("Fans: ")+valueStyle.Render(strings.Join(fanStrs, "  ")))
	}

	content := strings.Join(parts, dimStyle.Render("  |  "))
	return boxStyle.Render(content)
}

func (h HubModel) renderMenu() string {
	var sb strings.Builder

	for i, item := range hubMenuItems {
		selected := i == h.selectedItem

		cursor := "  "
		if selected {
			cursor = accentStyle.Render("\u25b8 ")
		}

		key := fmt.Sprintf("[%s] ", item.key)

		label := item.label

		desc := item.desc

		if selected {
			sb.WriteString(cursor)
			sb.WriteString(accentStyle.Render(key))
			sb.WriteString(accentStyle.Bold(true).Render(
				fmt.Sprintf("%-16s", label)))
			if !item.ready {
				sb.WriteString(dimStyle.Render(desc))
				sb.WriteString(dimStyle.Render("  (coming soon)"))
			} else {
				sb.WriteString(valueStyle.Render(desc))
			}
		} else {
			sb.WriteString(cursor)
			if !item.ready {
				sb.WriteString(dimStyle.Render(key))
				sb.WriteString(dimStyle.Render(fmt.Sprintf("%-16s", label)))
				sb.WriteString(dimStyle.Render(desc))
				sb.WriteString(dimStyle.Render("  (coming soon)"))
			} else {
				sb.WriteString(labelStyle.Render(key))
				sb.WriteString(valueStyle.Render(fmt.Sprintf("%-16s", label)))
				sb.WriteString(labelStyle.Render(desc))
			}
		}

		sb.WriteString("\n")
	}

	return sb.String()
}
