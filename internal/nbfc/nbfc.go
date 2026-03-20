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
