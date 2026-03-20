package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mango/freshMango/internal/nbfc"
	"github.com/mango/freshMango/internal/system"
)

type installerStep int

const (
	stepInstall   installerStep = iota // Download & install nbfc
	stepDetect                          // Auto-detect system & find configs
	stepConfigure                       // Pick config (with safety scores)
	stepSensors                         // Verify/configure sensor assignments
	stepStart                           // Start the service
	stepEnable                          // Enable on boot
	stepDone                            // Completion
)

// InstallerModel is a Bubble Tea model for the distro-aware installer wizard.
type InstallerModel struct {
	step         installerStep
	startStep    installerStep // step to start from (allows skipping install if already present)
	sysInfo      *system.Info
	distroFamily string
	installed    bool
	allConfigs   []string
	matchConfigs []string
	configScores map[string]int // config name -> 0-100 score
	selectedCfg  int
	showAll      bool
	configSet    bool
	started      bool
	enabled      bool
	inputActive  bool
	sensors      []nbfc.SensorInfo
	sensorInfo   string // formatted sensor assignment output
	sensorStatus string // "checking" | "ok" | "unavailable" | "error"
	aurHelper    string // cached AUR helper detection result (set once on entering stepInstall)
	aurDetected  bool   // whether AUR helper detection has been done
	width, height int
	statusMsg    string
	err          error
}

// Messages for async installer operations
type installerInstallDoneMsg struct{ err error }
type installerDetectDoneMsg struct {
	info    *system.Info
	configs []string
	matches []string
	scores  map[string]int
}
type installerApplyConfigMsg struct{ err error }
type installerSensorCheckMsg struct {
	sensors    []nbfc.SensorInfo
	sensorInfo string
	err        error
}
type installerStartDoneMsg struct{ err error }
type installerEnableDoneMsg struct{ err error }

func NewInstaller() InstallerModel {
	return InstallerModel{
		step:         stepInstall,
		startStep:    stepInstall,
		configScores: make(map[string]int),
		sensorStatus: "checking",
	}
}

// NewInstallerAt creates an installer starting at a specific step.
func NewInstallerAt(step installerStep) InstallerModel {
	m := NewInstaller()
	m.step = step
	m.startStep = step
	if step > stepInstall {
		m.installed = true
	}
	return m
}

func (m InstallerModel) Init() tea.Cmd {
	return func() tea.Msg {
		return installerCheckInstallMsg(nbfc.IsInstalled())
	}
}

type installerCheckInstallMsg bool

func (m InstallerModel) Update(msg tea.Msg) (InstallerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case installerCheckInstallMsg:
		m.installed = bool(msg)
		if m.installed {
			// Already installed — skip to detect
			m.step = stepDetect
			return m, m.detect()
		}
		// Detect distro family for install guidance
		if info, err := system.Detect(); err == nil && info != nil {
			m.sysInfo = info
			m.distroFamily = info.DistroFamily()
		}
		// Pre-detect AUR helper for arch so renderInstallStep doesn't call it per frame
		if m.distroFamily == "arch" && !m.aurDetected {
			m.aurDetected = true
			m.aurHelper = "GitHub binary"
		}
		// If started past stepInstall but nbfc is not installed, force back to install
		if m.step > stepInstall {
			m.step = stepInstall
			m.startStep = stepInstall
			m.statusMsg = "nbfc not installed — starting from install step"
		}

	case installerInstallDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			m.statusMsg = "Install failed: " + msg.err.Error()
		} else {
			m.installed = true
			m.step = stepDetect
			m.statusMsg = "nbfc-linux installed successfully"
			m.err = nil
			return m, m.detect()
		}

	case installerDetectDoneMsg:
		m.sysInfo = msg.info
		m.allConfigs = msg.configs
		m.matchConfigs = msg.matches
		m.configScores = msg.scores
		if m.sysInfo != nil {
			m.distroFamily = m.sysInfo.DistroFamily()
		}
		m.step = stepConfigure

	case installerApplyConfigMsg:
		if msg.err != nil {
			m.err = msg.err
			m.statusMsg = "Config failed: " + msg.err.Error()
		} else {
			m.configSet = true
			m.step = stepSensors
			m.statusMsg = "Config applied"
			m.err = nil
			return m, m.checkSensors()
		}

	case installerSensorCheckMsg:
		if m.step != stepSensors {
			// Ignore late-arriving sensor results if user moved past this step
			return m, nil
		}
		if msg.err != nil {
			m.sensorStatus = "unavailable"
			m.statusMsg = "Sensor detection unavailable — skipping"
		} else {
			m.sensors = msg.sensors
			m.sensorInfo = msg.sensorInfo
			m.sensorStatus = "ok"
			m.statusMsg = ""
		}

	case installerStartDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			m.statusMsg = "Start failed: " + msg.err.Error()
		} else {
			m.started = true
			m.step = stepEnable
			m.statusMsg = "nbfc started"
			m.err = nil
		}

	case installerEnableDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			m.statusMsg = "Enable failed: " + msg.err.Error()
		} else {
			m.enabled = true
			m.step = stepDone
			m.statusMsg = "All set! nbfc is running and enabled on boot."
			m.err = nil
		}

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m InstallerModel) handleKey(msg tea.KeyMsg) (InstallerModel, tea.Cmd) {
	// Esc goes back one step (on first step, ignore — user must complete the wizard)
	if msg.String() == "esc" {
		if m.step <= m.startStep {
			// Don't allow escaping the installer before completing setup
			return m, nil
		}
		// Go back one step
		switch m.step {
		case stepDetect:
			// Can't really go back to install if already installed
			if !m.installed {
				m.step = stepInstall
			}
		case stepConfigure:
			m.step = stepDetect
		case stepSensors:
			m.step = stepConfigure
		case stepStart:
			m.step = stepSensors
		case stepEnable:
			m.step = stepStart
		}
		m.statusMsg = ""
		m.err = nil
		return m, nil
	}

	switch m.step {
	case stepInstall:
		if msg.String() == "enter" && !m.installed {
			if m.distroFamily == "" || m.distroFamily == "unknown" {
				m.statusMsg = "Unsupported distro. Please install nbfc manually, then press 'r' to re-check."
				return m, nil
			}
			m.statusMsg = "Installing nbfc-linux... (may ask for sudo password)"
			m.err = nil
			return m, m.installCmd()
		}
		if msg.String() == "r" && !m.installed {
			m.statusMsg = "Re-checking nbfc installation..."
			return m, func() tea.Msg {
				return installerCheckInstallMsg(nbfc.IsInstalled())
			}
		}

	case stepDetect:
		// Auto-advances, no key handling needed

	case stepConfigure:
		configs := m.visibleConfigs()
		switch msg.String() {
		case "up", "k":
			if m.selectedCfg > 0 {
				m.selectedCfg--
			}
		case "down", "j":
			if m.selectedCfg < len(configs)-1 {
				m.selectedCfg++
			}
		case "enter":
			if len(configs) > 0 {
				name := configs[m.selectedCfg]
				m.statusMsg = "Applying: " + name
				m.err = nil
				return m, func() tea.Msg {
					return installerApplyConfigMsg{err: nbfc.SetConfig(name)}
				}
			}
		case "tab":
			m.showAll = !m.showAll
			m.selectedCfg = 0
		}

	case stepSensors:
		switch msg.String() {
		case "enter":
			if m.sensorStatus == "checking" {
				m.statusMsg = "Waiting for sensor check..."
				return m, nil
			}
			// Accept sensor assignments and proceed
			m.step = stepStart
			m.statusMsg = ""
			return m, nil
		}

	case stepStart:
		if msg.String() == "enter" {
			m.statusMsg = "Starting nbfc service..."
			m.err = nil
			return m, func() tea.Msg {
				return installerStartDoneMsg{err: nbfc.Start()}
			}
		}

	case stepEnable:
		switch msg.String() {
		case "enter":
			m.statusMsg = "Enabling nbfc service on boot..."
			m.err = nil
			return m, func() tea.Msg {
				return installerEnableDoneMsg{err: nbfc.EnableService()}
			}
		case "s":
			// Skip enable step
			m.step = stepDone
			m.statusMsg = "Setup complete! (service not enabled on boot)"
			return m, nil
		}

	case stepDone:
		if msg.String() == "enter" {
			return m, func() tea.Msg { return popViewMsg{} }
		}
	}
	return m, nil
}

func (m InstallerModel) installCmd() tea.Cmd {
	return func() tea.Msg {
		var err error
		switch m.distroFamily {
		case "debian":
			err = nbfc.InstallDebian()
		case "arch":
			err = nbfc.InstallArch()
		default:
			// Fallback to debian-style install
			err = nbfc.Install()
		}
		return installerInstallDoneMsg{err: err}
	}
}

func (m InstallerModel) detect() tea.Cmd {
	return func() tea.Msg {
		info, _ := system.Detect()
		configs, _ := nbfc.ListConfigs()

		var matches []string
		if info != nil {
			matches = system.FindMatchingConfigs(info.ProductName, configs)
		}

		// Try nbfc config -r first
		if info != nil {
			if recommended, err := nbfc.RecommendConfigs(info.ProductName); err == nil && len(recommended) > 0 {
				matches = recommended
			}
		}

		// Compute config scores
		scores := make(map[string]int)
		if info != nil {
			// Detect fan count from current status (may fail if service not running)
			fanCount := 0
			if fans, err := nbfc.Status(); err == nil {
				fanCount = len(fans)
			}

			// Score the recommended configs
			for _, c := range matches {
				scores[c] = nbfc.RateConfig(c, info.ProductName, fanCount)
			}
			// Also score a sample of all configs (limit to avoid slowness)
			limit := 50
			if len(configs) < limit {
				limit = len(configs)
			}
			for _, c := range configs[:limit] {
				if _, ok := scores[c]; !ok {
					scores[c] = nbfc.RateConfig(c, info.ProductName, fanCount)
				}
			}
		}

		return installerDetectDoneMsg{
			info:    info,
			configs: configs,
			matches: matches,
			scores:  scores,
		}
	}
}

func (m InstallerModel) checkSensors() tea.Cmd {
	return func() tea.Msg {
		sensors, err := nbfc.ListSensors()
		if err != nil {
			return installerSensorCheckMsg{err: err}
		}
		sensorInfo, _ := nbfc.ShowSensors()
		return installerSensorCheckMsg{
			sensors:    sensors,
			sensorInfo: sensorInfo,
		}
	}
}

func (m InstallerModel) visibleConfigs() []string {
	if m.showAll {
		return m.sortedConfigs(m.allConfigs)
	}
	if len(m.matchConfigs) > 0 {
		return m.sortedConfigs(m.matchConfigs)
	}
	return m.sortedConfigs(m.allConfigs)
}

// sortedConfigs returns configs sorted by score descending.
func (m InstallerModel) sortedConfigs(configs []string) []string {
	if len(m.configScores) == 0 {
		return configs
	}
	sorted := make([]string, len(configs))
	copy(sorted, configs)
	sort.Slice(sorted, func(i, j int) bool {
		si := m.configScores[sorted[i]]
		sj := m.configScores[sorted[j]]
		if si != sj {
			return si > sj
		}
		return sorted[i] < sorted[j]
	})
	return sorted
}

func (m InstallerModel) View() string {
	title := titleStyle.Render("Installer Wizard")

	// Step progress indicator
	indicator := m.renderStepIndicator()

	content := m.renderStep()

	status := ""
	if m.statusMsg != "" {
		status = "\n" + accentStyle.Render(m.statusMsg)
	}
	if m.err != nil {
		status = "\n" + redStyle.Render("Error: "+m.err.Error())
	}

	help := m.renderHelp()

	return lipgloss.JoinVertical(lipgloss.Left,
		title, "", indicator, "", content, status, "", help)
}

func (m InstallerModel) renderStepIndicator() string {
	steps := []struct {
		name string
		step installerStep
	}{
		{"Install", stepInstall},
		{"Detect", stepDetect},
		{"Configure", stepConfigure},
		{"Sensors", stepSensors},
		{"Start", stepStart},
		{"Enable", stepEnable},
	}

	var parts []string
	for _, s := range steps {
		label := s.name
		if s.step < m.step {
			parts = append(parts, greenStyle.Render("✓ "+label))
		} else if s.step == m.step {
			parts = append(parts, accentStyle.Copy().Bold(true).Render("▸ "+label))
		} else {
			parts = append(parts, dimStyle.Render("○ "+label))
		}
	}
	return strings.Join(parts, dimStyle.Render(" → "))
}

func (m InstallerModel) renderStep() string {
	switch m.step {
	case stepInstall:
		return m.renderInstallStep()
	case stepDetect:
		return dimStyle.Render("Detecting system and available configs...")
	case stepConfigure:
		return m.renderConfigPicker()
	case stepSensors:
		return m.renderSensorStep()
	case stepStart:
		return m.renderStartStep()
	case stepEnable:
		return m.renderEnableStep()
	case stepDone:
		return m.renderDoneStep()
	}
	return ""
}

func (m InstallerModel) renderInstallStep() string {
	if m.installed {
		return greenStyle.Render("✓ nbfc-linux is installed")
	}

	var sb strings.Builder

	sb.WriteString(yellowStyle.Render("nbfc-linux is not installed"))
	sb.WriteString("\n\n")

	// Show detected distro
	if m.distroFamily != "" && m.distroFamily != "unknown" {
		sb.WriteString(labelStyle.Render("Detected: "))
		sb.WriteString(valueStyle.Render(m.distroFamily))
		sb.WriteString("\n\n")
	}

	switch m.distroFamily {
	case "debian":
		sb.WriteString(dimStyle.Render("Will download the latest .deb from GitHub releases\n"))
		sb.WriteString(dimStyle.Render("and install via dpkg + apt for dependencies.\n"))
	case "arch":
		helper := m.aurHelper
		if helper == "" {
			helper = "GitHub binary"
		}
		sb.WriteString(dimStyle.Render(fmt.Sprintf("Will install via AUR helper or %s.\n", helper)))
	default:
		sb.WriteString(redStyle.Render("Unsupported distribution detected.\n\n"))
		sb.WriteString(dimStyle.Render("Please install nbfc-linux manually:\n"))
		sb.WriteString(accentStyle.Render("  https://github.com/nbfc-linux/nbfc-linux/releases\n\n"))
		sb.WriteString(dimStyle.Render("After installing, restart freshMango."))
		return boxStyle.Render(sb.String())
	}

	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("Press Enter to install. This may prompt for sudo password."))

	return boxStyle.Render(sb.String())
}

func (m InstallerModel) renderConfigPicker() string {
	var sb strings.Builder

	if m.sysInfo != nil {
		sb.WriteString(labelStyle.Render("Detected: "))
		sb.WriteString(valueStyle.Render(m.sysInfo.ProductName))
		sb.WriteString("\n\n")
	}

	configs := m.visibleConfigs()
	if m.showAll {
		sb.WriteString(titleStyle.Render("All Configs"))
		sb.WriteString(dimStyle.Render(fmt.Sprintf("  (%d total, Tab for recommended)", len(configs))))
	} else if len(m.matchConfigs) > 0 {
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
	if m.selectedCfg >= visible {
		start = m.selectedCfg - visible + 1
	}
	end := start + visible
	if end > len(configs) {
		end = len(configs)
	}

	if start > 0 {
		sb.WriteString(dimStyle.Render("  ↑ more...\n"))
	}
	for i := start; i < end; i++ {
		name := configs[i]
		scoreStr := ""
		if score, ok := m.configScores[name]; ok && score > 0 {
			scoreStyle := dimStyle
			if score >= 70 {
				scoreStyle = greenStyle
			} else if score >= 40 {
				scoreStyle = yellowStyle
			}
			scoreStr = scoreStyle.Render(fmt.Sprintf(" [%d%%]", score))
		}

		if i == m.selectedCfg {
			sb.WriteString(accentStyle.Render("  ▸ " + name))
			sb.WriteString(scoreStr)
		} else {
			sb.WriteString(dimStyle.Render("    " + name))
			sb.WriteString(scoreStr)
		}
		sb.WriteString("\n")
	}
	if end < len(configs) {
		sb.WriteString(dimStyle.Render("  ↓ more..."))
	}

	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("↑↓/jk: select  Enter: apply  Tab: toggle all/recommended"))

	return boxStyle.Render(sb.String())
}

func (m InstallerModel) renderSensorStep() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Sensor Verification"))
	sb.WriteString("\n\n")

	switch m.sensorStatus {
	case "checking":
		sb.WriteString(dimStyle.Render("Checking sensor assignments..."))

	case "unavailable":
		sb.WriteString(yellowStyle.Render("Sensor detection is not available."))
		sb.WriteString("\n\n")
		sb.WriteString(dimStyle.Render("This is normal — sensors can be configured later.\n"))
		sb.WriteString(dimStyle.Render("Press Enter to continue."))

	case "ok":
		// Show detected hardware sensors
		if len(m.sensors) > 0 {
			sb.WriteString(labelStyle.Render("Detected hardware sensors:"))
			sb.WriteString("\n")
			for _, s := range m.sensors {
				tempStr := ""
				if s.Temp > 0 {
					tempStr = tempColor(s.Temp).Render(fmt.Sprintf(" (%.1f°C)", s.Temp))
				}
				sb.WriteString(fmt.Sprintf("  %s %s%s\n",
					accentStyle.Render("•"),
					valueStyle.Render(s.Name),
					tempStr))
			}
			sb.WriteString("\n")
		}

		// Show per-fan sensor assignments from config
		if m.sensorInfo != "" {
			sb.WriteString(labelStyle.Render("Fan sensor assignments:"))
			sb.WriteString("\n")
			for _, line := range strings.Split(m.sensorInfo, "\n") {
				if line != "" {
					sb.WriteString("  " + valueStyle.Render(line) + "\n")
				}
			}
			sb.WriteString("\n")
		}

		sb.WriteString(dimStyle.Render("Press Enter to accept sensor assignments."))

	default:
		sb.WriteString(dimStyle.Render("Press Enter to continue."))
	}

	return boxStyle.Render(sb.String())
}

func (m InstallerModel) renderStartStep() string {
	return boxStyle.Render(
		titleStyle.Render("Start nbfc service") + "\n\n" +
			dimStyle.Render("Press Enter to start the fan control service."))
}

func (m InstallerModel) renderEnableStep() string {
	return boxStyle.Render(
		titleStyle.Render("Enable on boot") + "\n\n" +
			dimStyle.Render("Press Enter to enable nbfc service at startup.\n"+
				"Press 's' to skip."))
}

func (m InstallerModel) renderDoneStep() string {
	return boxStyle.Render(
		greenStyle.Render("✓ Setup complete!") + "\n\n" +
			dimStyle.Render("nbfc is running") +
			func() string {
				if m.enabled {
					return dimStyle.Render(" and will start on boot")
				}
				return ""
			}() +
			dimStyle.Render(".\n") +
			dimStyle.Render("Press Enter to go to the Dashboard."))
}

func (m InstallerModel) renderHelp() string {
	isFirstStep := m.step <= m.startStep

	switch m.step {
	case stepInstall:
		if m.distroFamily == "unknown" || m.distroFamily == "" {
			help := "r: re-check"
			if !isFirstStep {
				help += "  Esc: back"
			}
			return dimStyle.Render(help)
		}
		help := "Enter: install  r: re-check"
		if !isFirstStep {
			help += "  Esc: back"
		}
		return dimStyle.Render(help)
	case stepDetect:
		return dimStyle.Render("Detecting...")
	case stepConfigure:
		help := "↑↓/jk: navigate  Enter: select  Tab: toggle"
		if !isFirstStep {
			help += "  Esc: back"
		}
		return dimStyle.Render(help)
	case stepSensors:
		help := "Enter: accept"
		if !isFirstStep {
			help += "  Esc: back to configure"
		}
		return dimStyle.Render(help)
	case stepStart:
		help := "Enter: start service"
		if !isFirstStep {
			help += "  Esc: back"
		}
		return dimStyle.Render(help)
	case stepEnable:
		help := "Enter: enable  s: skip"
		if !isFirstStep {
			help += "  Esc: back"
		}
		return dimStyle.Render(help)
	case stepDone:
		return dimStyle.Render("Enter: go to Dashboard")
	}
	return ""
}
