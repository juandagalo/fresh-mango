package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mango/freshMango/internal/nbfc"
	"github.com/mango/freshMango/internal/system"
)

type setupStep int

const (
	stepInstall setupStep = iota
	stepDetect
	stepConfigure
	stepStart
	stepEnable
	stepDone
)

type SetupModel struct {
	step         setupStep
	sysInfo      *system.Info
	installed    bool
	allConfigs   []string
	matchConfigs []string
	selectedCfg  int
	showAll      bool
	configSet    bool
	started      bool
	enabled      bool
	inputActive  bool
	width, height int
	statusMsg    string
	err          error
}

// Messages for async operations
type installDoneMsg struct{ err error }
type detectDoneMsg struct {
	info    *system.Info
	configs []string
	matches []string
}
type applyConfigMsg struct{ err error }
type startDoneMsg struct{ err error }
type enableDoneMsg struct{ err error }

func NewSetup() SetupModel {
	return SetupModel{step: stepInstall}
}

func (s SetupModel) Init() tea.Cmd {
	return func() tea.Msg {
		return checkInstallMsg(nbfc.IsInstalled())
	}
}

type checkInstallMsg bool

func (s SetupModel) Update(msg tea.Msg) (SetupModel, tea.Cmd) {
	switch msg := msg.(type) {
	case checkInstallMsg:
		s.installed = bool(msg)
		if s.installed {
			s.step = stepDetect
			return s, s.detect()
		}

	case installDoneMsg:
		if msg.err != nil {
			s.err = msg.err
			s.statusMsg = "Install failed: " + msg.err.Error()
		} else {
			s.installed = true
			s.step = stepDetect
			s.statusMsg = "nbfc installed successfully"
			return s, s.detect()
		}

	case detectDoneMsg:
		s.sysInfo = msg.info
		s.allConfigs = msg.configs
		s.matchConfigs = msg.matches
		s.step = stepConfigure

	case applyConfigMsg:
		if msg.err != nil {
			s.err = msg.err
			s.statusMsg = "Config failed: " + msg.err.Error()
		} else {
			s.configSet = true
			s.step = stepStart
			s.statusMsg = "Config applied"
		}

	case startDoneMsg:
		if msg.err != nil {
			s.err = msg.err
			s.statusMsg = "Start failed: " + msg.err.Error()
		} else {
			s.started = true
			s.step = stepEnable
			s.statusMsg = "nbfc started"
		}

	case enableDoneMsg:
		if msg.err != nil {
			s.err = msg.err
			s.statusMsg = "Enable failed: " + msg.err.Error()
		} else {
			s.enabled = true
			s.step = stepDone
			s.statusMsg = "All set! nbfc is running and enabled on boot."
		}

	case tea.KeyMsg:
		return s.handleKey(msg)
	}
	return s, nil
}

func (s SetupModel) handleKey(msg tea.KeyMsg) (SetupModel, tea.Cmd) {
	switch s.step {
	case stepInstall:
		if msg.String() == "enter" && !s.installed {
			s.statusMsg = "Installing nbfc-linux... (may ask for sudo password)"
			return s, func() tea.Msg {
				return installDoneMsg{err: nbfc.Install()}
			}
		}
	case stepDetect:
		// Auto-advances
	case stepConfigure:
		configs := s.visibleConfigs()
		switch msg.String() {
		case "up", "k":
			if s.selectedCfg > 0 {
				s.selectedCfg--
			}
		case "down", "j":
			if s.selectedCfg < len(configs)-1 {
				s.selectedCfg++
			}
		case "enter":
			if len(configs) > 0 {
				name := configs[s.selectedCfg]
				s.statusMsg = "Applying: " + name
				return s, func() tea.Msg {
					return applyConfigMsg{err: nbfc.SetConfig(name)}
				}
			}
		case "tab":
			s.showAll = !s.showAll
			s.selectedCfg = 0
		}
	case stepStart:
		if msg.String() == "enter" {
			s.statusMsg = "Starting nbfc service..."
			return s, func() tea.Msg {
				return startDoneMsg{err: nbfc.Start()}
			}
		}
	case stepEnable:
		if msg.String() == "enter" {
			s.statusMsg = "Enabling nbfc service on boot..."
			return s, func() tea.Msg {
				return enableDoneMsg{err: nbfc.EnableService()}
			}
		}
	}
	return s, nil
}

func (s SetupModel) detect() tea.Cmd {
	return func() tea.Msg {
		info, _ := system.Detect()
		configs, _ := nbfc.ListConfigs()
		var matches []string
		if info != nil {
			matches = system.FindMatchingConfigs(info.ProductName, configs)
		}
		return detectDoneMsg{info: info, configs: configs, matches: matches}
	}
}

func (s SetupModel) visibleConfigs() []string {
	if s.showAll {
		return s.allConfigs
	}
	if len(s.matchConfigs) > 0 {
		return s.matchConfigs
	}
	return s.allConfigs
}

func (s SetupModel) View() string {
	title := titleStyle.Render("Setup Wizard")

	// Step indicator
	steps := []string{"Install", "Detect", "Configure", "Start", "Enable"}
	var stepParts []string
	for i, name := range steps {
		label := fmt.Sprintf("[%d] %s", i+1, name)
		if setupStep(i) < s.step {
			stepParts = append(stepParts, greenStyle.Render("✓ "+label))
		} else if setupStep(i) == s.step {
			stepParts = append(stepParts, cyanStyle.Render("▸ "+label))
		} else {
			stepParts = append(stepParts, dimStyle.Render("  "+label))
		}
	}
	indicator := strings.Join(stepParts, dimStyle.Render(" → "))

	content := s.renderStep()

	status := ""
	if s.statusMsg != "" {
		status = "\n" + cyanStyle.Render(s.statusMsg)
	}
	if s.err != nil {
		status = "\n" + redStyle.Render("Error: "+s.err.Error())
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		title, "", indicator, "", content, status)
}

func (s SetupModel) renderStep() string {
	switch s.step {
	case stepInstall:
		if s.installed {
			return greenStyle.Render("✓ nbfc-linux is installed")
		}
		return boxStyle.Render(
			yellowStyle.Render("nbfc-linux is not installed") + "\n\n" +
				dimStyle.Render("Press Enter to install nbfc-linux from GitHub releases.\n"+
					"This will download the .deb package and install dependencies."))

	case stepDetect:
		return dimStyle.Render("Detecting system...")

	case stepConfigure:
		return s.renderConfigPicker()

	case stepStart:
		return boxStyle.Render(
			titleStyle.Render("Start nbfc service") + "\n\n" +
				dimStyle.Render("Press Enter to start the fan control service."))

	case stepEnable:
		return boxStyle.Render(
			titleStyle.Render("Enable on boot") + "\n\n" +
				dimStyle.Render("Press Enter to enable nbfc service at startup."))

	case stepDone:
		return boxStyle.Render(
			greenStyle.Render("✓ Setup complete!") + "\n\n" +
				dimStyle.Render("nbfc is running and will start on boot.\n"+
					"Switch to the Dashboard tab to monitor your fans.\n"+
					"Use the Curve Editor tab to customize the fan curve."))
	}
	return ""
}

func (s SetupModel) renderConfigPicker() string {
	var sb strings.Builder

	if s.sysInfo != nil {
		sb.WriteString(labelStyle.Render("Detected: "))
		sb.WriteString(valueStyle.Render(s.sysInfo.ProductName))
		sb.WriteString("\n\n")
	}

	configs := s.visibleConfigs()
	if s.showAll {
		sb.WriteString(titleStyle.Render("All Configs"))
		sb.WriteString(dimStyle.Render(fmt.Sprintf("  (%d total, Tab for recommended)", len(configs))))
	} else if len(s.matchConfigs) > 0 {
		sb.WriteString(titleStyle.Render("Recommended Configs"))
		sb.WriteString(dimStyle.Render("  (Tab for all)"))
	} else {
		sb.WriteString(titleStyle.Render("All Configs"))
		sb.WriteString(dimStyle.Render("  (no matches found for your model)"))
	}
	sb.WriteString("\n\n")

	// Show a scrollable window of configs
	visible := 15
	start := 0
	if s.selectedCfg >= visible {
		start = s.selectedCfg - visible + 1
	}
	end := start + visible
	if end > len(configs) {
		end = len(configs)
	}

	if start > 0 {
		sb.WriteString(dimStyle.Render("  ↑ more...\n"))
	}
	for i := start; i < end; i++ {
		if i == s.selectedCfg {
			sb.WriteString(cyanStyle.Render("  ▸ " + configs[i]))
		} else {
			sb.WriteString(dimStyle.Render("    " + configs[i]))
		}
		sb.WriteString("\n")
	}
	if end < len(configs) {
		sb.WriteString(dimStyle.Render("  ↓ more..."))
	}

	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("↑↓: select  Enter: apply  Tab: toggle all/recommended"))

	return boxStyle.Render(sb.String())
}
