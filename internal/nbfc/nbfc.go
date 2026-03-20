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

const configDir = "/usr/share/nbfc/configs"

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
	ReadRegister              int         `json:"ReadRegister"`
	WriteRegister             int         `json:"WriteRegister"`
	MinSpeedValue             int         `json:"MinSpeedValue"`
	MaxSpeedValue             int         `json:"MaxSpeedValue"`
	IndependentReadMinMaxValues bool      `json:"IndependentReadMinMaxValues,omitempty"`
	MinSpeedValueRead         int         `json:"MinSpeedValueRead,omitempty"`
	MaxSpeedValueRead         int         `json:"MaxSpeedValueRead,omitempty"`
	ResetRequired             bool        `json:"ResetRequired,omitempty"`
	FanSpeedResetValue        int         `json:"FanSpeedResetValue,omitempty"`
	FanDisplayName            string      `json:"FanDisplayName"`
	Sensors                   []string    `json:"Sensors,omitempty"`
	TemperatureAlgorithmType  string      `json:"TemperatureAlgorithmType,omitempty"`
	TemperatureThresholds     []Threshold `json:"TemperatureThresholds"`
	FanSpeedPercentageOverrides []json.RawMessage `json:"FanSpeedPercentageOverrides,omitempty"`
}

type Config struct {
	NotebookModel                      string             `json:"NotebookModel"`
	Author                             string             `json:"Author,omitempty"`
	EcPollInterval                     int                `json:"EcPollInterval,omitempty"`
	ReadWriteWords                     bool               `json:"ReadWriteWords,omitempty"`
	CriticalTemperature                float64            `json:"CriticalTemperature"`
	LegacyTemperatureThresholdsBehaviour bool             `json:"LegacyTemperatureThresholdsBehaviour,omitempty"`
	FanConfigurations                  []FanConfiguration `json:"FanConfigurations"`
	RegisterWriteConfigurations        []json.RawMessage  `json:"RegisterWriteConfigurations,omitempty"`
}

func IsInstalled() bool {
	_, err := exec.LookPath("nbfc")
	return err == nil
}

func Install() error {
	cmd := exec.Command("bash", "-c",
		`cd /tmp && curl -L -o nbfc-linux.deb "$(curl -s https://api.github.com/repos/nbfc-linux/nbfc-linux/releases/latest | grep 'browser_download_url.*amd64.deb' | head -1 | cut -d'"' -f4)" && sudo dpkg -i nbfc-linux.deb; sudo apt install -f -y`)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func Status() ([]FanStatus, error) {
	out, err := exec.Command("nbfc", "status").Output()
	if err != nil {
		return nil, fmt.Errorf("nbfc status: %w", err)
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
	cmd := exec.Command("sudo", "nbfc", "start")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func Stop() error {
	cmd := exec.Command("sudo", "nbfc", "stop")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func Restart() error {
	if err := Stop(); err != nil {
		return err
	}
	return Start()
}

func ListConfigs() ([]string, error) {
	out, err := exec.Command("nbfc", "config", "-l").Output()
	if err != nil {
		return nil, fmt.Errorf("nbfc config -l: %w", err)
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
	out, err := exec.Command("nbfc", "status").Output()
	if err != nil {
		return "", err
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
	cmd := exec.Command("sudo", "nbfc", "config", "-s", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
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

func WriteConfigFile(config *Config) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
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

	dest := filepath.Join(configDir, config.NotebookModel+".json")
	cmd := exec.Command("sudo", "cp", tmpPath, dest)
	cmd.Stdin = os.Stdin
	err = cmd.Run()
	os.Remove(tmpPath)
	return err
}

// InstallDebian downloads the latest .deb from GitHub releases and installs via dpkg.
func InstallDebian() error {
	cmd := exec.Command("bash", "-c",
		`cd /tmp && curl -L -o nbfc-linux.deb "$(curl -s https://api.github.com/repos/nbfc-linux/nbfc-linux/releases/latest | grep 'browser_download_url.*amd64.deb' | head -1 | cut -d'"' -f4)" && sudo dpkg -i nbfc-linux.deb; sudo apt-get install -f -y`)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// InstallArch installs nbfc-linux on Arch-based distributions.
// Tries AUR helpers (yay, paru) first, then falls back to downloading the binary from GitHub releases.
func InstallArch() error {
	// Try yay first
	if _, err := exec.LookPath("yay"); err == nil {
		cmd := exec.Command("yay", "-S", "--noconfirm", "nbfc-linux")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		return cmd.Run()
	}

	// Try paru
	if _, err := exec.LookPath("paru"); err == nil {
		cmd := exec.Command("paru", "-S", "--noconfirm", "nbfc-linux")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		return cmd.Run()
	}

	// Fallback: download binary from GitHub releases
	cmd := exec.Command("bash", "-c",
		`cd /tmp && curl -L -o nbfc-linux.tar.gz "$(curl -s https://api.github.com/repos/nbfc-linux/nbfc-linux/releases/latest | grep 'browser_download_url.*x86_64.tar.gz' | head -1 | cut -d'"' -f4)" && sudo tar -xzf nbfc-linux.tar.gz -C /usr/local && sudo ln -sf /usr/local/bin/nbfc /usr/bin/nbfc`)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// RecommendConfigs runs `nbfc config -r` and parses the output as a list of config names.
// Falls back to keyword matching if the command is not available.
func RecommendConfigs(productName string) ([]string, error) {
	out, err := exec.Command("nbfc", "config", "-r").Output()
	if err == nil {
		var configs []string
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				configs = append(configs, line)
			}
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
	out, _ := exec.Command("systemctl", "is-enabled", "nbfc_service").Output()
	return strings.TrimSpace(string(out)) == "enabled"
}

func ServiceRunning() bool {
	out, _ := exec.Command("systemctl", "is-active", "nbfc_service").Output()
	return strings.TrimSpace(string(out)) == "active"
}

func EnableService() error {
	cmd := exec.Command("sudo", "bash", "-c", "systemctl enable nbfc_service && systemctl start nbfc_service")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
