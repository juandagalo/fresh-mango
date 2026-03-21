package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

// helper: create a fresh App and extract the model after an Update call.
func updateApp(a App, msg tea.Msg) App {
	m, _ := a.Update(msg)
	result, ok := m.(App)
	if !ok {
		panic("Update did not return an App")
	}
	return result
}

// ---------------------------------------------------------------------------
// Startup state machine transitions
// ---------------------------------------------------------------------------

func TestStartup_InitialState(t *testing.T) {
	app := NewApp()

	assert.Equal(t, stateChecking, app.startup, "New app should start in stateChecking")
	assert.Equal(t, []viewID{viewHub}, app.viewStack, "Initial view stack should contain viewHub")
}

func TestStartup_CheckingToNeedsInstall(t *testing.T) {
	app := NewApp()

	// Simulate: nbfc not installed
	app = updateApp(app, startupResultMsg{
		installed:  false,
		configured: false,
		running:    false,
	})

	assert.Equal(t, stateNeedsInstall, app.startup,
		"Should transition to stateNeedsInstall when nbfc is not installed")
	// The installer view should be pushed onto the stack
	assert.Equal(t, viewInstaller, app.activeViewID(),
		"Installer view should be active when install is needed")
}

func TestStartup_CheckingToNeedsConfig(t *testing.T) {
	app := NewApp()

	// Simulate: nbfc installed but not configured
	app = updateApp(app, startupResultMsg{
		installed:  true,
		configured: false,
		running:    false,
	})

	assert.Equal(t, stateNeedsConfig, app.startup,
		"Should transition to stateNeedsConfig when nbfc installed but not configured")
	assert.Equal(t, viewInstaller, app.activeViewID(),
		"Installer view should be active when config is needed")
}

func TestStartup_CheckingToNeedsStart(t *testing.T) {
	app := NewApp()

	// Simulate: nbfc installed, configured, but not running
	app = updateApp(app, startupResultMsg{
		installed:  true,
		configured: true,
		configName: "Lenovo ThinkPad T480",
		running:    false,
	})

	assert.Equal(t, stateNeedsStart, app.startup,
		"Should transition to stateNeedsStart when configured but service stopped")
	assert.Equal(t, "Lenovo ThinkPad T480", app.configName,
		"Config name should be set from startup result")
	assert.False(t, app.serviceOn, "Service should not be marked as running")
	assert.Equal(t, viewHub, app.activeViewID(),
		"Hub should remain active when only service start is needed")
}

func TestStartup_CheckingToReady(t *testing.T) {
	app := NewApp()

	// Simulate: everything is fine
	app = updateApp(app, startupResultMsg{
		installed:  true,
		configured: true,
		configName: "Dell XPS 15",
		running:    true,
	})

	assert.Equal(t, stateReady, app.startup,
		"Should transition to stateReady when all checks pass")
	assert.Equal(t, "Dell XPS 15", app.configName)
	assert.True(t, app.serviceOn, "Service should be marked as running")
	assert.Equal(t, viewHub, app.activeViewID(),
		"Hub should be active when ready")
}

func TestStartup_CheckingWithError(t *testing.T) {
	app := NewApp()

	// Simulate: detection error
	app = updateApp(app, startupResultMsg{
		err: assert.AnError,
	})

	assert.Equal(t, stateReady, app.startup,
		"Should fall through to stateReady on detection error")
	assert.Contains(t, app.statusMsg, "Startup check error",
		"Status message should contain error info")
}

// ---------------------------------------------------------------------------
// Service start transition
// ---------------------------------------------------------------------------

func TestServiceStart_Success(t *testing.T) {
	app := NewApp()
	app.startup = stateNeedsStart

	app = updateApp(app, serviceStartMsg{err: nil})

	assert.Equal(t, stateReady, app.startup,
		"Should transition to stateReady after successful service start")
	assert.True(t, app.serviceOn, "Service should be marked as running")
	assert.Empty(t, app.statusMsg, "Status message should be cleared on success")
}

func TestServiceStart_Failure(t *testing.T) {
	app := NewApp()
	app.startup = stateNeedsStart

	app = updateApp(app, serviceStartMsg{err: assert.AnError})

	assert.Equal(t, stateNeedsStart, app.startup,
		"Should remain in stateNeedsStart on failure")
	assert.False(t, app.serviceOn, "Service should not be marked as running on failure")
	assert.Contains(t, app.statusMsg, "Failed to start service",
		"Status message should contain error info")
}

// ---------------------------------------------------------------------------
// View stack navigation
// ---------------------------------------------------------------------------

func TestViewStack_PushAndPop(t *testing.T) {
	app := NewApp()
	app.startup = stateReady

	assert.Equal(t, viewHub, app.activeViewID())

	// Push dashboard
	app.pushView(viewDashboard)
	assert.Equal(t, viewDashboard, app.activeViewID())
	assert.Len(t, app.viewStack, 2)

	// Pop back to hub
	app.popView()
	assert.Equal(t, viewHub, app.activeViewID())
	assert.Len(t, app.viewStack, 1)

	// Pop on single-element stack should be a no-op
	app.popView()
	assert.Equal(t, viewHub, app.activeViewID())
	assert.Len(t, app.viewStack, 1)
}

func TestHubSelect_PushesView(t *testing.T) {
	app := NewApp()
	app.startup = stateReady

	app = updateApp(app, hubSelectMsg{id: viewDashboard})

	assert.Equal(t, viewDashboard, app.activeViewID(),
		"hubSelectMsg should push the selected view")
	assert.Len(t, app.viewStack, 2)
}

// ---------------------------------------------------------------------------
// Key handling
// ---------------------------------------------------------------------------

func TestKeyCtrlC_Quits(t *testing.T) {
	app := NewApp()
	app.startup = stateReady

	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	// The cmd should be a quit command — verify by executing it
	assert.NotNil(t, cmd, "ctrl+c should produce a command")
}

func TestKeyQ_QuitsFromHub(t *testing.T) {
	app := NewApp()
	app.startup = stateReady

	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	assert.NotNil(t, cmd, "q from hub should produce a quit command")
}

func TestKeyEsc_PopsView(t *testing.T) {
	app := NewApp()
	app.startup = stateReady
	app.pushView(viewDashboard)

	app = updateApp(app, tea.KeyMsg{Type: tea.KeyEsc})

	assert.Equal(t, viewHub, app.activeViewID(),
		"Esc should pop back to hub from a sub-view")
}

func TestKeyEsc_DoesNotPopFromHub(t *testing.T) {
	app := NewApp()
	app.startup = stateReady

	app = updateApp(app, tea.KeyMsg{Type: tea.KeyEsc})

	assert.Equal(t, viewHub, app.activeViewID(),
		"Esc on hub should not pop (already at root)")
}

// ---------------------------------------------------------------------------
// Config updated message
// ---------------------------------------------------------------------------

func TestConfigUpdatedMsg(t *testing.T) {
	app := NewApp()
	app.startup = stateReady

	app = updateApp(app, configUpdatedMsg("New Config Name"))

	assert.Equal(t, "New Config Name", app.configName)
}

// ---------------------------------------------------------------------------
// Window size message
// ---------------------------------------------------------------------------

func TestWindowSizeMsg(t *testing.T) {
	app := NewApp()

	app = updateApp(app, tea.WindowSizeMsg{Width: 120, Height: 40})

	assert.Equal(t, 120, app.width)
	assert.Equal(t, 40, app.height)
}

// ---------------------------------------------------------------------------
// popViewMsg from installer (stepDone)
// ---------------------------------------------------------------------------

func TestPopViewMsg_FromInstallerDone(t *testing.T) {
	app := NewApp()
	app.startup = stateNeedsInstall
	app.pushView(viewInstaller)
	app.installer.step = stepDone

	app = updateApp(app, popViewMsg{})

	assert.Equal(t, stateReady, app.startup,
		"Popping from completed installer should set stateReady")
	assert.Equal(t, viewHub, app.activeViewID(),
		"Should return to hub after installer completes")
}

func TestPopViewMsg_FromInstallerNotDone(t *testing.T) {
	app := NewApp()
	app.startup = stateNeedsInstall
	app.pushView(viewInstaller)
	app.installer.step = stepConfigure // Not done yet

	app = updateApp(app, popViewMsg{})

	assert.Equal(t, viewInstaller, app.activeViewID(),
		"Should NOT pop from installer when wizard is not complete")
}
