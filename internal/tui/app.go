package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mango/freshMango/internal/nbfc"
)

// viewID identifies each navigable view in the hub-and-spoke model.
type viewID int

const (
	viewHub         viewID = iota
	viewDashboard
	viewCurveEditor
	viewFanControl  // placeholder for Phase 2
	viewProfiles    // placeholder for Phase 3
	viewSensors     // placeholder for Phase 3
	viewSettings    // placeholder for Phase 3
	viewInstaller   // for Commit 4
)

type tickMsg time.Time
type configUpdatedMsg string

// pushViewMsg pushes a new view onto the stack.
type pushViewMsg struct{ id viewID }

// popViewMsg pops the current view, returning to the previous one.
type popViewMsg struct{}

type App struct {
	viewStack   []viewID
	hub         HubModel
	dashboard   DashboardModel
	curveEditor CurveEditorModel
	setup       SetupModel
	width, height int
	configName    string
	serviceOn     bool
}

func NewApp() App {
	return App{
		viewStack:   []viewID{viewHub},
		hub:         NewHub(),
		dashboard:   NewDashboard(),
		curveEditor: NewCurveEditor(),
		setup:       NewSetup(),
	}
}

func (a *App) pushView(v viewID) {
	a.viewStack = append(a.viewStack, v)
}

func (a *App) popView() {
	if len(a.viewStack) > 1 {
		a.viewStack = a.viewStack[:len(a.viewStack)-1]
	}
}

func (a App) activeViewID() viewID {
	return a.viewStack[len(a.viewStack)-1]
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
		a.hub.width = msg.Width
		a.hub.height = msg.Height

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return a, tea.Quit
		}
		// q quits from hub only (or when not editing)
		if msg.String() == "q" && !a.isEditing() && a.activeViewID() == viewHub {
			return a, tea.Quit
		}
		// Esc pops the view stack (unless editing or on hub)
		if msg.String() == "esc" && !a.isEditing() && a.activeViewID() != viewHub {
			a.popView()
			return a, nil
		}

	case hubSelectMsg:
		a.pushView(msg.id)
		// When entering curve editor, load config
		if msg.id == viewCurveEditor {
			cmds = append(cmds, a.curveEditor.loadConfig())
		}
		// When entering dashboard, trigger init for fresh data
		if msg.id == viewDashboard {
			cmds = append(cmds, a.dashboard.Init())
		}
		return a, tea.Batch(cmds...)

	case pushViewMsg:
		a.pushView(msg.id)
		return a, nil

	case popViewMsg:
		a.popView()
		return a, nil

	case tickMsg:
		a.refreshStatus()
		// Sync hub summary fields
		a.hub.configName = a.configName
		a.hub.serviceOn = a.serviceOn
		cmds = append(cmds, tickCmd())

	case configUpdatedMsg:
		a.configName = string(msg)
	}

	// Route messages to the active view
	switch a.activeViewID() {
	case viewHub:
		m, cmd := a.hub.Update(msg)
		a.hub = m
		cmds = append(cmds, cmd)
	case viewDashboard:
		m, cmd := a.dashboard.Update(msg)
		a.dashboard = m
		cmds = append(cmds, cmd)
	case viewCurveEditor:
		m, cmd := a.curveEditor.Update(msg)
		a.curveEditor = m
		cmds = append(cmds, cmd)
	}

	return a, tea.Batch(cmds...)
}

func (a App) View() string {
	var content string
	switch a.activeViewID() {
	case viewHub:
		content = a.hub.View()
	case viewDashboard:
		content = a.dashboard.View()
	case viewCurveEditor:
		content = a.curveEditor.View()
	case viewFanControl, viewProfiles, viewSensors, viewSettings:
		content = a.renderPlaceholder()
	}

	// Breadcrumb navigation bar
	nav := a.renderBreadcrumb()

	// Leave room for breadcrumb (1) + status bar (1) + padding (2)
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
	output := lipgloss.JoinVertical(lipgloss.Left, nav, body, status)

	// Apply root background fill
	return bgStyle.Width(a.width).Height(a.height).Render(output)
}

func (a App) isEditing() bool {
	return (a.activeViewID() == viewCurveEditor && a.curveEditor.editing) ||
		(a.activeViewID() == viewInstaller && a.setup.inputActive)
}

func (a *App) refreshStatus() {
	if cfg, err := nbfc.GetSelectedConfig(); err == nil && cfg != "" {
		a.configName = cfg
	}
	a.serviceOn = nbfc.ServiceRunning()

	// Update hub temperature and fan speed data
	if fans, err := nbfc.Status(); err == nil && len(fans) > 0 {
		a.hub.cpuTemp = fans[0].Temperature
		speeds := make([]float64, len(fans))
		for i, f := range fans {
			speeds[i] = f.CurrentSpeed
		}
		a.hub.fanSpeeds = speeds
	}
}

func (a App) viewName(v viewID) string {
	switch v {
	case viewHub:
		return "Hub"
	case viewDashboard:
		return "Dashboard"
	case viewCurveEditor:
		return "Curve Editor"
	case viewFanControl:
		return "Fan Control"
	case viewProfiles:
		return "Profiles"
	case viewSensors:
		return "Sensors"
	case viewSettings:
		return "Settings"
	case viewInstaller:
		return "Installer"
	}
	return "Unknown"
}

func (a App) renderBreadcrumb() string {
	var parts []string
	for i, v := range a.viewStack {
		name := a.viewName(v)
		if i == len(a.viewStack)-1 {
			parts = append(parts, accentStyle.Copy().Bold(true).Render(name))
		} else {
			parts = append(parts, labelStyle.Render(name))
		}
	}
	crumb := strings.Join(parts, dimStyle.Render(" > "))

	// Pad to full width
	if bw := lipgloss.Width(crumb); a.width > bw {
		crumb += tabStyle.Render(strings.Repeat(" ", a.width-bw))
	}
	return tabStyle.Render("  ") + crumb
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

	// Navigation hints based on current view
	var right string
	switch a.activeViewID() {
	case viewHub:
		right = "  1-6: select  q: quit  "
	default:
		right = "  Esc: back  q: quit (from hub)  "
	}

	gap := ""
	if lw, rw := lipgloss.Width(left), lipgloss.Width(right); a.width > lw+rw {
		gap = strings.Repeat(" ", a.width-lw-rw)
	}
	return statusBarStyle.Width(a.width).Render(left + gap + right)
}

func (a App) renderPlaceholder() string {
	name := a.viewName(a.activeViewID())
	title := titleStyle.Render(name)
	msg := accentStyle.Render("Coming soon")
	hint := dimStyle.Render("This feature is planned for a future release.")
	back := dimStyle.Render("Press Esc to return to the hub.")
	return lipgloss.JoinVertical(lipgloss.Left, title, "", msg, "", hint, "", back)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}
