package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mango/freshMango/internal/nbfc"
)

type view int

const (
	viewDashboard view = iota
	viewCurveEditor
	viewSetup
)

type tickMsg time.Time
type configUpdatedMsg string

type App struct {
	active        view
	dashboard     DashboardModel
	curveEditor   CurveEditorModel
	setup         SetupModel
	width, height int
	configName    string
	serviceOn     bool
}

func NewApp() App {
	return App{
		active:      viewDashboard,
		dashboard:   NewDashboard(),
		curveEditor: NewCurveEditor(),
		setup:       NewSetup(),
	}
}

func (a App) Init() tea.Cmd {
	return tea.Batch(tickCmd(), a.dashboard.Init(), a.setup.Init())
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.dashboard.width = msg.Width
		a.dashboard.height = msg.Height
		a.curveEditor.width = msg.Width
		a.curveEditor.height = msg.Height
		a.setup.width = msg.Width
		a.setup.height = msg.Height

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return a, tea.Quit
		}
		if msg.String() == "q" && !a.isEditing() {
			return a, tea.Quit
		}
		if msg.String() == "tab" && !a.isEditing() {
			a.active = (a.active + 1) % 3
			if a.active == viewCurveEditor {
				cmds = append(cmds, a.curveEditor.loadConfig())
			}
			return a, tea.Batch(cmds...)
		}
		if msg.String() == "shift+tab" && !a.isEditing() {
			a.active = (a.active + 2) % 3
			if a.active == viewCurveEditor {
				cmds = append(cmds, a.curveEditor.loadConfig())
			}
			return a, tea.Batch(cmds...)
		}

	case tickMsg:
		a.refreshStatus()
		cmds = append(cmds, tickCmd())

	case configUpdatedMsg:
		a.configName = string(msg)
	}

	switch a.active {
	case viewDashboard:
		m, cmd := a.dashboard.Update(msg)
		a.dashboard = m
		cmds = append(cmds, cmd)
	case viewCurveEditor:
		m, cmd := a.curveEditor.Update(msg)
		a.curveEditor = m
		cmds = append(cmds, cmd)
	case viewSetup:
		m, cmd := a.setup.Update(msg)
		a.setup = m
		cmds = append(cmds, cmd)
	}

	return a, tea.Batch(cmds...)
}

func (a App) View() string {
	tabs := a.renderTabBar()

	var content string
	switch a.active {
	case viewDashboard:
		content = a.dashboard.View()
	case viewCurveEditor:
		content = a.curveEditor.View()
	case viewSetup:
		content = a.setup.View()
	}

	// Leave room for tab bar (1) + status bar (1) + padding (2)
	availH := a.height - 4
	if availH < 5 {
		availH = 5
	}
	body := lipgloss.NewStyle().
		Padding(1, 2).
		Height(availH).
		MaxHeight(availH).
		Render(content)

	status := a.renderStatusBar()
	return lipgloss.JoinVertical(lipgloss.Left, tabs, body, status)
}

func (a App) isEditing() bool {
	return (a.active == viewCurveEditor && a.curveEditor.editing) ||
		(a.active == viewSetup && a.setup.inputActive)
}

func (a *App) refreshStatus() {
	if cfg, err := nbfc.GetSelectedConfig(); err == nil && cfg != "" {
		a.configName = cfg
	}
	a.serviceOn = nbfc.ServiceRunning()
}

func (a App) renderTabBar() string {
	labels := []string{"Dashboard", "Curve Editor", "Setup"}
	var parts []string
	for i, l := range labels {
		if view(i) == a.active {
			parts = append(parts, activeTabStyle.Render(" "+l+" "))
		} else {
			parts = append(parts, tabStyle.Render(" "+l+" "))
		}
	}
	bar := lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	if bw := lipgloss.Width(bar); a.width > bw {
		bar += tabStyle.Render(strings.Repeat(" ", a.width-bw))
	}
	return bar
}

func (a App) renderStatusBar() string {
	cfg := a.configName
	if cfg == "" {
		cfg = "none"
	}
	svc := redStyle.Render("stopped")
	if a.serviceOn {
		svc = greenStyle.Render("running")
	}
	left := fmt.Sprintf("  Config: %s  |  Service: %s", cfg, svc)
	right := "  Tab: switch  q: quit  "
	gap := ""
	if lw, rw := lipgloss.Width(left), lipgloss.Width(right); a.width > lw+rw {
		gap = strings.Repeat(" ", a.width-lw-rw)
	}
	return statusBarStyle.Width(a.width).Render(left + gap + right)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}
