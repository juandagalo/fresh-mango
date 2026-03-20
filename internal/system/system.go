package system

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type Info struct {
	ProductName string
	Distro      string
	DistroID    string // raw ID from /etc/os-release: "ubuntu", "arch", "debian", etc.
	DistroIDLike string // ID_LIKE fallback from /etc/os-release
	Kernel      string
	CPUModel    string
	CPUCores    int
	CPUThreads  int
}

// DistroFamily returns a normalized distribution family string based on
// the ID and ID_LIKE fields from /etc/os-release.
// Returns "debian", "arch", or "unknown".
func (i *Info) DistroFamily() string {
	id := strings.ToLower(i.DistroID)
	idLike := strings.ToLower(i.DistroIDLike)

	// Check ID first
	switch id {
	case "debian", "ubuntu", "linuxmint", "pop", "elementary", "zorin", "kali", "raspbian":
		return "debian"
	case "arch", "manjaro", "endeavouros", "garuda", "artix", "cachyos":
		return "arch"
	}

	// Fallback to ID_LIKE
	for _, token := range strings.Fields(idLike) {
		switch token {
		case "debian", "ubuntu":
			return "debian"
		case "arch":
			return "arch"
		}
	}

	return "unknown"
}

func Detect() (*Info, error) {
	info := &Info{
		CPUThreads: runtime.NumCPU(),
	}

	if data, err := os.ReadFile("/sys/devices/virtual/dmi/id/product_name"); err == nil {
		info.ProductName = strings.TrimSpace(string(data))
	}

	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				info.Distro = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
			}
			if strings.HasPrefix(line, "ID=") {
				info.DistroID = strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
			}
			if strings.HasPrefix(line, "ID_LIKE=") {
				info.DistroIDLike = strings.Trim(strings.TrimPrefix(line, "ID_LIKE="), "\"")
			}
		}
	}

	if out, err := exec.Command("uname", "-r").Output(); err == nil {
		info.Kernel = strings.TrimSpace(string(out))
	}

	if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		cores := make(map[string]bool)
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "model name") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 && info.CPUModel == "" {
					info.CPUModel = strings.TrimSpace(parts[1])
				}
			}
			if strings.HasPrefix(line, "core id") {
				cores[strings.TrimSpace(line)] = true
			}
		}
		info.CPUCores = len(cores)
		if info.CPUCores == 0 {
			info.CPUCores = info.CPUThreads
		}
	}

	return info, nil
}

func FindMatchingConfigs(productName string, configs []string) []string {
	words := strings.Fields(strings.ToLower(productName))
	var matches []string
	for _, c := range configs {
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
	return matches
}
