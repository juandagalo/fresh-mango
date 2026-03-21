package system

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// parseOsRelease tests
// ---------------------------------------------------------------------------

func TestParseOsRelease_Ubuntu(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "os-release-ubuntu"))
	require.NoError(t, err)

	info := &Info{}
	parseOsRelease(string(data), info)

	assert.Equal(t, "Ubuntu 24.04.1 LTS", info.Distro)
	assert.Equal(t, "ubuntu", info.DistroID)
	assert.Equal(t, "debian", info.DistroIDLike)
}

func TestParseOsRelease_Arch(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "os-release-arch"))
	require.NoError(t, err)

	info := &Info{}
	parseOsRelease(string(data), info)

	assert.Equal(t, "Arch Linux", info.Distro)
	assert.Equal(t, "arch", info.DistroID)
	assert.Equal(t, "", info.DistroIDLike, "Arch has no ID_LIKE field")
}

func TestParseOsRelease_Empty(t *testing.T) {
	info := &Info{}
	parseOsRelease("", info)

	assert.Empty(t, info.Distro)
	assert.Empty(t, info.DistroID)
	assert.Empty(t, info.DistroIDLike)
}

func TestParseOsRelease_QuotedValues(t *testing.T) {
	data := `PRETTY_NAME="Pop!_OS 22.04 LTS"
ID=pop
ID_LIKE="ubuntu debian"
`
	info := &Info{}
	parseOsRelease(data, info)

	assert.Equal(t, "Pop!_OS 22.04 LTS", info.Distro)
	assert.Equal(t, "pop", info.DistroID)
	assert.Equal(t, "ubuntu debian", info.DistroIDLike)
}

// ---------------------------------------------------------------------------
// DistroFamily tests
// ---------------------------------------------------------------------------

func TestDistroFamily_DirectID(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		idLike   string
		expected string
	}{
		{"Ubuntu", "ubuntu", "debian", "debian"},
		{"Debian", "debian", "", "debian"},
		{"LinuxMint", "linuxmint", "ubuntu debian", "debian"},
		{"Pop!_OS", "pop", "ubuntu debian", "debian"},
		{"Kali", "kali", "debian", "debian"},
		{"Arch", "arch", "", "arch"},
		{"Manjaro", "manjaro", "arch", "arch"},
		{"EndeavourOS", "endeavouros", "arch", "arch"},
		{"Garuda", "garuda", "arch linux", "arch"},
		{"CachyOS", "cachyos", "arch", "arch"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			info := &Info{DistroID: tc.id, DistroIDLike: tc.idLike}
			assert.Equal(t, tc.expected, info.DistroFamily())
		})
	}
}

func TestDistroFamily_FallbackToIDLike(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		idLike   string
		expected string
	}{
		{"CustomDebianDerivative", "customos", "debian", "debian"},
		{"CustomUbuntuDerivative", "customos", "ubuntu", "debian"},
		{"CustomArchDerivative", "customos", "arch", "arch"},
		{"MultipleIDLike_DebianFirst", "customos", "debian ubuntu", "debian"},
		{"MultipleIDLike_ArchFirst", "customos", "arch linux", "arch"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			info := &Info{DistroID: tc.id, DistroIDLike: tc.idLike}
			assert.Equal(t, tc.expected, info.DistroFamily())
		})
	}
}

func TestDistroFamily_Unknown(t *testing.T) {
	tests := []struct {
		name   string
		id     string
		idLike string
	}{
		{"EmptyFields", "", ""},
		{"UnknownDistro", "nixos", ""},
		{"UnknownIDLike", "nixos", "nixos"},
		{"Fedora", "fedora", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			info := &Info{DistroID: tc.id, DistroIDLike: tc.idLike}
			assert.Equal(t, "unknown", info.DistroFamily())
		})
	}
}

func TestDistroFamily_CaseInsensitive(t *testing.T) {
	info := &Info{DistroID: "Ubuntu", DistroIDLike: "Debian"}
	assert.Equal(t, "debian", info.DistroFamily())

	info2 := &Info{DistroID: "ARCH", DistroIDLike: ""}
	assert.Equal(t, "arch", info2.DistroFamily())
}

// ---------------------------------------------------------------------------
// parseCPUInfo tests
// ---------------------------------------------------------------------------

func TestParseCPUInfo_SingleCore(t *testing.T) {
	data := `processor	: 0
vendor_id	: GenuineIntel
cpu family	: 6
model		: 142
model name	: Intel(R) Core(TM) i5-8250U CPU @ 1.60GHz
stepping	: 10
cpu MHz		: 800.000
core id		: 0
`
	info := &Info{CPUThreads: 4}
	parseCPUInfo(data, info)

	assert.Equal(t, "Intel(R) Core(TM) i5-8250U CPU @ 1.60GHz", info.CPUModel)
	assert.Equal(t, 1, info.CPUCores)
}

func TestParseCPUInfo_MultiCore(t *testing.T) {
	data := `processor	: 0
model name	: AMD Ryzen 7 5800H
core id		: 0

processor	: 1
model name	: AMD Ryzen 7 5800H
core id		: 1

processor	: 2
model name	: AMD Ryzen 7 5800H
core id		: 2

processor	: 3
model name	: AMD Ryzen 7 5800H
core id		: 3
`
	info := &Info{CPUThreads: 8}
	parseCPUInfo(data, info)

	assert.Equal(t, "AMD Ryzen 7 5800H", info.CPUModel)
	assert.Equal(t, 4, info.CPUCores)
}

func TestParseCPUInfo_NoCoreID_FallsBackToThreads(t *testing.T) {
	data := `processor	: 0
model name	: ARM Cortex-A72

processor	: 1
model name	: ARM Cortex-A72
`
	info := &Info{CPUThreads: 4}
	parseCPUInfo(data, info)

	assert.Equal(t, "ARM Cortex-A72", info.CPUModel)
	assert.Equal(t, 4, info.CPUCores, "Should fallback to CPUThreads when no core id lines")
}

func TestParseCPUInfo_Empty(t *testing.T) {
	info := &Info{CPUThreads: 2}
	parseCPUInfo("", info)

	assert.Empty(t, info.CPUModel)
	assert.Equal(t, 2, info.CPUCores, "Should fallback to CPUThreads for empty input")
}

func TestParseCPUInfo_ModelNameOnlyFirst(t *testing.T) {
	// Ensure only the first model name is captured (not overwritten by subsequent)
	data := `processor	: 0
model name	: Intel Core i7-1165G7
core id		: 0

processor	: 1
model name	: Intel Core i7-1165G7
core id		: 1
`
	info := &Info{CPUThreads: 8}
	parseCPUInfo(data, info)

	assert.Equal(t, "Intel Core i7-1165G7", info.CPUModel)
}

// ---------------------------------------------------------------------------
// FindMatchingConfigs tests
// ---------------------------------------------------------------------------

func TestFindMatchingConfigs_MatchesTwoWords(t *testing.T) {
	configs := []string{
		"Lenovo ThinkPad T480",
		"Lenovo IdeaPad 5",
		"Dell XPS 13",
		"HP Pavilion 15",
	}
	matches := FindMatchingConfigs("Lenovo ThinkPad T480s", configs)
	assert.Contains(t, matches, "Lenovo ThinkPad T480")
}

func TestFindMatchingConfigs_IgnoresShortWords(t *testing.T) {
	configs := []string{
		"HP Pavilion 15",
		"HP Spectre x360",
	}
	// "HP" is only 2 chars so it's ignored. "Pavilion" alone gives score=1,
	// and the function requires score >= 2, so neither config matches.
	matches := FindMatchingConfigs("HP Pavilion", configs)
	assert.Empty(t, matches, "Single long word match should not be enough (need >= 2)")
}

func TestFindMatchingConfigs_RequiresTwoWordMatch(t *testing.T) {
	configs := []string{
		"Lenovo Legion 5 Pro",
		"Lenovo IdeaPad 5",
	}
	// "Lenovo" and "Legion" both match the first config (score=2)
	matches := FindMatchingConfigs("Lenovo Legion", configs)
	assert.Contains(t, matches, "Lenovo Legion 5 Pro")
	assert.NotContains(t, matches, "Lenovo IdeaPad 5",
		"Only 'Lenovo' matches IdeaPad config (score=1, need >=2)")
}

func TestFindMatchingConfigs_NoMatch(t *testing.T) {
	configs := []string{
		"Dell XPS 13",
		"Dell XPS 15",
	}
	matches := FindMatchingConfigs("Lenovo ThinkPad", configs)
	assert.Empty(t, matches)
}

func TestFindMatchingConfigs_CaseInsensitive(t *testing.T) {
	configs := []string{
		"lenovo thinkpad t480",
	}
	matches := FindMatchingConfigs("LENOVO THINKPAD T480", configs)
	assert.Contains(t, matches, "lenovo thinkpad t480")
}

func TestFindMatchingConfigs_EmptyInputs(t *testing.T) {
	assert.Empty(t, FindMatchingConfigs("", []string{"Dell XPS 13"}))
	assert.Empty(t, FindMatchingConfigs("Dell XPS 13", nil))
	assert.Empty(t, FindMatchingConfigs("", nil))
}
