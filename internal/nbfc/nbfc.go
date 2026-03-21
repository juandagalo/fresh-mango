package nbfc

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

var configDir = "/usr/share/nbfc/configs"

// backupFn and writeFn are overridable for testing. In production they use
// sudo-based operations; tests replace them with simple file copies/writes.
var (
	backupFn = backupConfigFileSudo
	writeFn  = writeViaSudo
)

type FanStatus struct {
	Name         string
	Temperature  float64
	CurrentSpeed float64
	TargetSpeed  float64
	AutoControl  bool
	CriticalMode bool
	SpeedSteps   int
}

type Threshold struct {
	UpThreshold   float64 `json:"UpThreshold"`
	DownThreshold float64 `json:"DownThreshold"`
	FanSpeed      float64 `json:"FanSpeed"`
}

type FanConfiguration struct {
	ReadRegister                int               `json:"ReadRegister"`
	WriteRegister               int               `json:"WriteRegister"`
	MinSpeedValue               int               `json:"MinSpeedValue"`
	MaxSpeedValue               int               `json:"MaxSpeedValue"`
	IndependentReadMinMaxValues bool              `json:"IndependentReadMinMaxValues"`
	MinSpeedValueRead           int               `json:"MinSpeedValueRead"`
	MaxSpeedValueRead           int               `json:"MaxSpeedValueRead"`
	ResetRequired               bool              `json:"ResetRequired"`
	FanSpeedResetValue          int               `json:"FanSpeedResetValue"`
	FanDisplayName              string            `json:"FanDisplayName"`
	Sensors                     []string          `json:"Sensors,omitempty"`
	TemperatureAlgorithmType    string            `json:"TemperatureAlgorithmType,omitempty"`
	TemperatureThresholds       []Threshold       `json:"TemperatureThresholds"`
	FanSpeedPercentageOverrides []json.RawMessage `json:"FanSpeedPercentageOverrides,omitempty"`
}

type Config struct {
	NotebookModel                        string             `json:"NotebookModel"`
	Author                               string             `json:"Author,omitempty"`
	EcPollInterval                       int                `json:"EcPollInterval"`
	ReadWriteWords                       bool               `json:"ReadWriteWords"`
	CriticalTemperature                  float64            `json:"CriticalTemperature"`
	LegacyTemperatureThresholdsBehaviour bool               `json:"LegacyTemperatureThresholdsBehaviour"`
	FanConfigurations                    []FanConfiguration `json:"FanConfigurations"`
	RegisterWriteConfigurations          []json.RawMessage  `json:"RegisterWriteConfigurations,omitempty"`
}

func IsInstalled() bool {
	_, err := exec.LookPath("nbfc")
	return err == nil
}

// runSilent executes a non-sudo command, suppressing all output. On failure,
// the captured stderr/stdout is included in the returned error.
func runSilent(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w\n%s", cmd.Path, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// runSudo executes a command via sudo with stdin wired to the terminal so that
// password prompts work. Output is still captured (not sent to os.Stdout) to
// avoid corrupting the TUI.
func runSudo(args ...string) error {
	cmd := exec.Command("sudo", args...)
	cmd.Stdin = os.Stdin
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w\n%s", cmd.Path, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func Install() error {
	return runSudo("bash", "-c",
		`cd /tmp && curl -L -o nbfc-linux.deb "$(curl -s https://api.github.com/repos/nbfc-linux/nbfc-linux/releases/latest | grep 'browser_download_url.*amd64.deb' | head -1 | cut -d'"' -f4)" && dpkg -i nbfc-linux.deb; apt install -f -y`)
}

func Status() ([]FanStatus, error) {
	out, err := exec.Command("nbfc", "status").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("nbfc status: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return parseStatus(string(out))
}

func parseStatus(raw string) ([]FanStatus, error) {
	var fans []FanStatus
	var current *FanStatus

	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if current != nil {
				fans = append(fans, *current)
				current = nil
			}
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "Fan Display Name":
			current = &FanStatus{Name: val}
		case "Temperature":
			if current != nil {
				current.Temperature, _ = strconv.ParseFloat(val, 64)
			}
		case "Current Fan Speed":
			if current != nil {
				current.CurrentSpeed, _ = strconv.ParseFloat(val, 64)
			}
		case "Target Fan Speed":
			if current != nil {
				current.TargetSpeed, _ = strconv.ParseFloat(val, 64)
			}
		case "Auto Control Enabled":
			if current != nil {
				current.AutoControl = val == "true"
			}
		case "Critical Mode Enabled":
			if current != nil {
				current.CriticalMode = val == "true"
			}
		case "Fan Speed Steps":
			if current != nil {
				current.SpeedSteps, _ = strconv.Atoi(val)
			}
		}
	}
	if current != nil {
		fans = append(fans, *current)
	}
	return fans, nil
}

func Start() error {
	return runSudo("nbfc", "start")
}

func Stop() error {
	return runSudo("nbfc", "stop")
}

func Restart() error {
	if err := Stop(); err != nil {
		return err
	}
	return Start()
}

func ListConfigs() ([]string, error) {
	out, err := exec.Command("nbfc", "config", "-l").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("nbfc config -l: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	var configs []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			configs = append(configs, line)
		}
	}
	return configs, nil
}

func GetSelectedConfig() (string, error) {
	out, err := exec.Command("nbfc", "status").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("nbfc status: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == "Selected Config Name" {
			return strings.TrimSpace(parts[1]), nil
		}
	}
	return "", nil
}

func SetConfig(name string) error {
	return runSudo("nbfc", "config", "-s", name)
}

func ReadConfigFile(name string) (*Config, error) {
	path := filepath.Join(configDir, name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// configPath returns the full path for a config file by name.
func configPath(model string) string {
	return filepath.Join(configDir, model+".json")
}

// backupPath returns the .bak path for a config file.
func backupPath(model string) string {
	return configPath(model) + ".bak"
}

// backupConfigFileSudo copies the original config to a .bak file via sudo.
// Returns nil if the original file does not exist (nothing to back up).
func backupConfigFileSudo(model string) error {
	src := configPath(model)
	dst := backupPath(model)

	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil // nothing to back up
	}

	return runSudo("cp", src, dst)
}

// backupConfigFile delegates to backupFn (overridable for testing).
func backupConfigFile(model string) error {
	return backupFn(model)
}

// RestoreBackup copies the .bak file back over the config and restarts nbfc.
func RestoreBackup(model string) error {
	src := backupPath(model)
	dst := configPath(model)

	if _, err := os.Stat(src); os.IsNotExist(err) {
		return fmt.Errorf("no backup file found for %s", model)
	}

	if err := runSudo("cp", src, dst); err != nil {
		return fmt.Errorf("restore backup: %w", err)
	}
	if err := Restart(); err != nil {
		return fmt.Errorf("restart after restore: %w", err)
	}
	return nil
}

// SaveAndRestart wraps the full save flow with automatic rollback.
// It writes the config (which creates a backup first), restarts nbfc,
// and if the restart fails, restores the backup and restarts again.
func SaveAndRestart(cfg *Config) error {
	if err := WriteConfigFile(cfg); err != nil {
		return err
	}
	if err := Restart(); err != nil {
		// Restart failed — attempt to restore the backup.
		src := backupPath(cfg.NotebookModel)
		dst := configPath(cfg.NotebookModel)
		restoreErr := runSudo("cp", src, dst)
		if restoreErr != nil {
			return fmt.Errorf("config failed AND restore failed: %w. Manual restore needed from .bak file", restoreErr)
		}
		if restartErr := Restart(); restartErr != nil {
			return fmt.Errorf("config failed, original restored but service won't start: %w", restartErr)
		}
		return fmt.Errorf("config failed, original restored successfully")
	}
	return nil
}

// WriteConfigFile writes the config back to disk, preserving any unknown fields
// from the original JSON file that our Go structs don't model. It reads the
// original file as a generic map, overlays the fields we may have changed
// (FanConfigurations with TemperatureThresholds), and writes the merged result.
// A backup of the original file is created before writing.
func WriteConfigFile(config *Config) error {
	dest := configPath(config.NotebookModel)

	// Back up the original config before overwriting.
	if err := backupConfigFile(config.NotebookModel); err != nil {
		return fmt.Errorf("backup before write: %w", err)
	}

	// Try to read the original file as a generic map to preserve unknown fields.
	var original map[string]interface{}
	if origData, err := os.ReadFile(dest); err == nil {
		_ = json.Unmarshal(origData, &original)
	}

	if original != nil {
		// Surgically update only TemperatureThresholds within each
		// FanConfiguration so that optional fields we don't model in Go
		// (EcPollInterval, CriticalTemperatureOffset, etc.) are preserved
		// exactly as they appear in the original JSON.
		if origFans, ok := original["FanConfigurations"].([]interface{}); ok {
			for i, fan := range config.FanConfigurations {
				if i >= len(origFans) {
					break
				}
				origFan, ok := origFans[i].(map[string]interface{})
				if !ok {
					continue
				}
				// Marshal only the new TemperatureThresholds from our struct.
				threshData, err := json.Marshal(fan.TemperatureThresholds)
				if err != nil {
					return err
				}
				var thresholds interface{}
				if err := json.Unmarshal(threshData, &thresholds); err != nil {
					return err
				}
				origFan["TemperatureThresholds"] = thresholds
			}
		}

		data, err := json.MarshalIndent(original, "", "  ")
		if err != nil {
			return err
		}
		return writeFn(data, dest)
	}

	// No original file — just marshal our struct directly.
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return writeFn(data, dest)
}

// writeViaSudo writes data to a destination path via a temp file + sudo cp,
// capturing all output silently.
func writeViaSudo(data []byte, dest string) error {
	tmp, err := os.CreateTemp("", "nbfc-config-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	tmp.Close()

	err = runSudo("cp", tmpPath, dest)
	os.Remove(tmpPath)
	return err
}

// InstallDebian downloads the latest .deb from GitHub releases and installs via dpkg.
func InstallDebian() error {
	return runSudo("bash", "-c",
		`cd /tmp && curl -L -o nbfc-linux.deb "$(curl -s https://api.github.com/repos/nbfc-linux/nbfc-linux/releases/latest | grep 'browser_download_url.*amd64.deb' | head -1 | cut -d'"' -f4)" && dpkg -i nbfc-linux.deb; apt-get install -f -y`)
}

// InstallArch installs nbfc-linux on Arch-based distributions.
// Tries AUR helpers (yay, paru) first, then falls back to downloading the binary from GitHub releases.
func InstallArch() error {
	// Try yay first
	if _, err := exec.LookPath("yay"); err == nil {
		return runSilent("yay", "-S", "--noconfirm", "nbfc-linux")
	}

	// Try paru
	if _, err := exec.LookPath("paru"); err == nil {
		return runSilent("paru", "-S", "--noconfirm", "nbfc-linux")
	}

	// Fallback: download binary from GitHub releases
	return runSudo("bash", "-c",
		`cd /tmp && curl -L -o nbfc-linux.tar.gz "$(curl -s https://api.github.com/repos/nbfc-linux/nbfc-linux/releases/latest | grep 'browser_download_url.*x86_64.tar.gz' | head -1 | cut -d'"' -f4)" && tar -xzf nbfc-linux.tar.gz -C /usr/local && ln -sf /usr/local/bin/nbfc /usr/bin/nbfc`)
}

// RecommendConfigs runs `nbfc config -r` and parses the output as a list of config names.
// Falls back to keyword matching if the command is not available.
func RecommendConfigs(productName string) ([]string, error) {
	out, err := exec.Command("nbfc", "config", "-r").CombinedOutput()
	// Non-zero exit code is normal when there are no recommendations — just skip to fallback.
	if err == nil {
		var configs []string
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			upper := strings.ToUpper(line)
			if strings.HasPrefix(upper, "ERROR:") || strings.HasPrefix(upper, "INFO:") {
				continue
			}
			configs = append(configs, line)
		}
		if len(configs) > 0 {
			return configs, nil
		}
	}

	// Fallback to keyword matching
	allConfigs, err := ListConfigs()
	if err != nil {
		return nil, err
	}

	words := strings.Fields(strings.ToLower(productName))
	var matches []string
	for _, c := range allConfigs {
		cl := strings.ToLower(c)
		score := 0
		for _, w := range words {
			if len(w) <= 2 {
				continue
			}
			if strings.Contains(cl, w) {
				score++
			}
		}
		if score >= 2 {
			matches = append(matches, c)
		}
	}
	return matches, nil
}

// SensorInfo describes a hardware temperature sensor.
type SensorInfo struct {
	Name string
	Temp float64
	Path string
}

// ListSensors reads /sys/class/hwmon/*/name to discover available temperature sensors.
func ListSensors() ([]SensorInfo, error) {
	hwmonDirs, err := filepath.Glob("/sys/class/hwmon/hwmon*")
	if err != nil {
		return nil, fmt.Errorf("glob hwmon: %w", err)
	}

	var sensors []SensorInfo
	for _, dir := range hwmonDirs {
		nameData, err := os.ReadFile(filepath.Join(dir, "name"))
		if err != nil {
			continue
		}
		name := strings.TrimSpace(string(nameData))

		// Read temp1_input if it exists
		var temp float64
		tempPath := filepath.Join(dir, "temp1_input")
		if data, err := os.ReadFile(tempPath); err == nil {
			if v, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64); err == nil {
				temp = v / 1000.0 // millidegrees to degrees
			}
		}

		sensors = append(sensors, SensorInfo{
			Name: name,
			Temp: temp,
			Path: dir,
		})
	}

	if len(sensors) == 0 {
		return nil, fmt.Errorf("no hardware sensors found")
	}
	return sensors, nil
}

// ShowSensors returns a formatted string describing per-fan sensor assignments.
// Reads the selected config to determine which sensors each fan uses.
func ShowSensors() (string, error) {
	cfgName, err := GetSelectedConfig()
	if err != nil || cfgName == "" {
		return "", fmt.Errorf("no config selected")
	}
	cfg, err := ReadConfigFile(cfgName)
	if err != nil {
		return "", fmt.Errorf("read config: %w", err)
	}

	var sb strings.Builder
	for i, fan := range cfg.FanConfigurations {
		sb.WriteString(fmt.Sprintf("Fan %d: %s\n", i+1, fan.FanDisplayName))
		if len(fan.Sensors) > 0 {
			sb.WriteString(fmt.Sprintf("  Sensors: %s\n", strings.Join(fan.Sensors, ", ")))
		} else {
			sb.WriteString("  Sensors: (default — CPU temperature)\n")
		}
		sb.WriteString(fmt.Sprintf("  Algorithm: %s\n", fan.TemperatureAlgorithmType))
	}
	return sb.String(), nil
}

// RateConfig scores a config's compatibility with the current system from 0-100.
// +30: config NotebookModel fuzzy-matches system ProductName
// +25: FanConfigurations count matches detected fan count
// +25: TemperatureThresholds cover range 40-90 with >=4 steps
// +20: CriticalTemperature is set and <= 100
func RateConfig(name string, productName string, fanCount int) int {
	cfg, err := ReadConfigFile(name)
	if err != nil {
		return 0
	}

	score := 0

	// Model name match (+30)
	if productName != "" && cfg.NotebookModel != "" {
		modelLower := strings.ToLower(cfg.NotebookModel)
		productLower := strings.ToLower(productName)
		words := strings.Fields(productLower)
		matchCount := 0
		for _, w := range words {
			if len(w) <= 2 {
				continue
			}
			if strings.Contains(modelLower, w) {
				matchCount++
			}
		}
		if matchCount >= 2 {
			score += 30
		} else if matchCount == 1 {
			score += 15
		}
	}

	// Fan count match (+25)
	if fanCount > 0 && len(cfg.FanConfigurations) == fanCount {
		score += 25
	} else if len(cfg.FanConfigurations) > 0 {
		score += 10 // at least has fan configs
	}

	// Threshold coverage (+25)
	for _, fan := range cfg.FanConfigurations {
		if len(fan.TemperatureThresholds) >= 4 {
			// Check if thresholds span a reasonable range (40-90)
			minT := 200.0
			maxT := 0.0
			for _, t := range fan.TemperatureThresholds {
				if t.UpThreshold < minT {
					minT = t.UpThreshold
				}
				if t.UpThreshold > maxT {
					maxT = t.UpThreshold
				}
			}
			if minT <= 50 && maxT >= 80 {
				score += 25
			} else {
				score += 15
			}
			break // only check first fan
		}
	}

	// Critical temperature (+20)
	if cfg.CriticalTemperature > 0 && cfg.CriticalTemperature <= 100 {
		score += 20
	}

	return score
}

func ServiceEnabled() bool {
	out, _ := exec.Command("systemctl", "is-enabled", "nbfc_service").CombinedOutput()
	return strings.TrimSpace(string(out)) == "enabled"
}

func ServiceRunning() bool {
	out, _ := exec.Command("systemctl", "is-active", "nbfc_service").CombinedOutput()
	return strings.TrimSpace(string(out)) == "active"
}

func EnableService() error {
	return runSudo("bash", "-c", "systemctl enable nbfc_service && systemctl start nbfc_service")
}
