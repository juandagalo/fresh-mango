package nbfc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// loadFixture reads a file from the testdata directory.
func loadFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err, "failed to read fixture %s", name)
	return string(data)
}

// withTestConfigDir overrides the package-level configDir for the duration of
// a test and restores it on cleanup.
func withTestConfigDir(t *testing.T, dir string) {
	t.Helper()
	orig := configDir
	configDir = dir
	t.Cleanup(func() { configDir = orig })
}

// withTestHooks overrides backupFn and writeFn with simple filesystem
// implementations (no sudo) and restores the originals on cleanup.
func withTestHooks(t *testing.T) {
	t.Helper()

	origBackup := backupFn
	origWrite := writeFn

	backupFn = func(model string) error {
		src := configPath(model)
		dst := backupPath(model)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			return nil
		}
		data, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0644)
	}

	writeFn = func(data []byte, dest string) error {
		return os.WriteFile(dest, data, 0644)
	}

	t.Cleanup(func() {
		backupFn = origBackup
		writeFn = origWrite
	})
}

// writeTestConfig writes a Config as JSON into the given directory.
func writeTestConfig(t *testing.T, dir string, cfg *Config) {
	t.Helper()
	data, err := json.MarshalIndent(cfg, "", "  ")
	require.NoError(t, err, "marshal test config")
	path := filepath.Join(dir, cfg.NotebookModel+".json")
	require.NoError(t, os.WriteFile(path, data, 0644), "write test config")
}

// ===========================================================================
// TASK-05: Unit tests for parseStatus
// ===========================================================================

func TestParseStatus(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string // non-empty → load from testdata/
		raw      string // used when fixture is empty
		wantLen  int
		wantFans []FanStatus
	}{
		{
			name:    "single fan from fixture",
			fixture: "status_single_fan.txt",
			wantLen: 1,
			wantFans: []FanStatus{
				{
					Name:         "CPU Fan",
					Temperature:  52.00,
					CurrentSpeed: 35.12,
					TargetSpeed:  35.00,
					AutoControl:  true,
					CriticalMode: false,
					SpeedSteps:   7,
				},
			},
		},
		{
			name:    "dual fan from fixture",
			fixture: "status_dual_fan.txt",
			wantLen: 2,
			wantFans: []FanStatus{
				{
					Name:         "CPU Fan",
					Temperature:  67.50,
					CurrentSpeed: 58.30,
					TargetSpeed:  60.00,
					AutoControl:  true,
					CriticalMode: false,
					SpeedSteps:   10,
				},
				{
					Name:         "GPU Fan",
					Temperature:  72.00,
					CurrentSpeed: 75.00,
					TargetSpeed:  75.00,
					AutoControl:  true,
					CriticalMode: false,
					SpeedSteps:   8,
				},
			},
		},
		{
			name:    "empty output",
			raw:     "",
			wantLen: 0,
		},
		{
			name:    "whitespace only",
			raw:     "   \n\n   \n",
			wantLen: 0,
		},
		{
			name:    "malformed output — no colons",
			raw:     "some random text\nwithout any key-value pairs\njust garbage data",
			wantLen: 0,
		},
		{
			name:    "malformed — fields before Fan Display Name are ignored",
			raw:     "Temperature              : 52.00\nCurrent Fan Speed        : 35.12\n",
			wantLen: 0,
		},
		{
			name: "single fan without trailing newline",
			raw: `Fan Display Name         : Silent Fan
Temperature              : 45.00
Current Fan Speed        : 20.00
Target Fan Speed         : 20.00
Auto Control Enabled     : true
Critical Mode Enabled    : false
Fan Speed Steps          : 5`,
			wantLen: 1,
			wantFans: []FanStatus{
				{
					Name:         "Silent Fan",
					Temperature:  45.00,
					CurrentSpeed: 20.00,
					TargetSpeed:  20.00,
					AutoControl:  true,
					CriticalMode: false,
					SpeedSteps:   5,
				},
			},
		},
		{
			name:    "fan with critical mode enabled",
			raw:     "Fan Display Name         : Hot Fan\nTemperature              : 99.00\nCurrent Fan Speed        : 100.00\nTarget Fan Speed         : 100.00\nAuto Control Enabled     : false\nCritical Mode Enabled    : true\nFan Speed Steps          : 3\n",
			wantLen: 1,
			wantFans: []FanStatus{
				{
					Name:         "Hot Fan",
					Temperature:  99.00,
					CurrentSpeed: 100.00,
					TargetSpeed:  100.00,
					AutoControl:  false,
					CriticalMode: true,
					SpeedSteps:   3,
				},
			},
		},
		{
			name:    "partial fan — only name and temperature",
			raw:     "Fan Display Name         : Partial Fan\nTemperature              : 45.00\n",
			wantLen: 1,
			wantFans: []FanStatus{
				{
					Name:        "Partial Fan",
					Temperature: 45.00,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := tt.raw
			if tt.fixture != "" {
				raw = loadFixture(t, tt.fixture)
			}

			fans, err := parseStatus(raw)
			require.NoError(t, err)

			if tt.wantLen == 0 {
				assert.Empty(t, fans)
				return
			}

			require.Len(t, fans, tt.wantLen)
			for i, want := range tt.wantFans {
				got := fans[i]
				assert.Equal(t, want.Name, got.Name, "fan %d Name", i)
				assert.InDelta(t, want.Temperature, got.Temperature, 0.01, "fan %d Temperature", i)
				assert.InDelta(t, want.CurrentSpeed, got.CurrentSpeed, 0.01, "fan %d CurrentSpeed", i)
				assert.InDelta(t, want.TargetSpeed, got.TargetSpeed, 0.01, "fan %d TargetSpeed", i)
				assert.Equal(t, want.AutoControl, got.AutoControl, "fan %d AutoControl", i)
				assert.Equal(t, want.CriticalMode, got.CriticalMode, "fan %d CriticalMode", i)
				assert.Equal(t, want.SpeedSteps, got.SpeedSteps, "fan %d SpeedSteps", i)
			}
		})
	}
}

// ===========================================================================
// TASK-06: Unit tests for RateConfig
// ===========================================================================

func TestRateConfig(t *testing.T) {
	// Helper to create and write a config with specific properties.
	makeConfig := func(model string, fanCount int, critTemp float64, thresholds []Threshold) *Config {
		fans := make([]FanConfiguration, fanCount)
		for i := range fans {
			fans[i] = FanConfiguration{
				FanDisplayName:        "Fan",
				TemperatureThresholds: thresholds,
			}
		}
		return &Config{
			NotebookModel:       model,
			CriticalTemperature: critTemp,
			FanConfigurations:   fans,
		}
	}

	// Good thresholds: >=4 steps spanning <=50 to >=80 → +25
	goodThresholds := []Threshold{
		{UpThreshold: 40, DownThreshold: 35, FanSpeed: 20},
		{UpThreshold: 55, DownThreshold: 50, FanSpeed: 40},
		{UpThreshold: 70, DownThreshold: 65, FanSpeed: 60},
		{UpThreshold: 85, DownThreshold: 80, FanSpeed: 80},
		{UpThreshold: 95, DownThreshold: 90, FanSpeed: 100},
	}

	// Narrow thresholds: >=4 steps but NOT spanning min<=50 && max>=80 → +15
	narrowThresholds := []Threshold{
		{UpThreshold: 60, DownThreshold: 55, FanSpeed: 30},
		{UpThreshold: 65, DownThreshold: 60, FanSpeed: 50},
		{UpThreshold: 70, DownThreshold: 65, FanSpeed: 70},
		{UpThreshold: 75, DownThreshold: 70, FanSpeed: 90},
	}

	// Few thresholds: <4 steps → +0 for threshold coverage
	fewThresholds := []Threshold{
		{UpThreshold: 50, DownThreshold: 45, FanSpeed: 50},
		{UpThreshold: 80, DownThreshold: 75, FanSpeed: 100},
	}

	tests := []struct {
		name        string
		cfg         *Config // nil means don't write any config (file-not-found test)
		productName string
		fanCount    int
		wantScore   int
		wantReason  string // description for clarity
	}{
		{
			name:        "exact model match + fan count + good thresholds + critical temp → perfect score",
			cfg:         makeConfig("Lenovo IdeaPad 5 15ARE05", 1, 95, goodThresholds),
			productName: "Lenovo IdeaPad 5 15ARE05",
			fanCount:    1,
			wantScore:   100, // 30 + 25 + 25 + 20
		},
		{
			name:        "partial model match (1 word)",
			cfg:         makeConfig("Lenovo ThinkPad X1 Carbon", 1, 95, goodThresholds),
			productName: "Lenovo IdeaPad 5 15ARE05",
			fanCount:    1,
			wantScore:   85, // 15 + 25 + 25 + 20
		},
		{
			name:        "no model match",
			cfg:         makeConfig("Dell XPS 15 9570", 1, 95, goodThresholds),
			productName: "Lenovo IdeaPad 5 15ARE05",
			fanCount:    1,
			wantScore:   70, // 0 + 25 + 25 + 20
		},
		{
			name:        "wrong fan count gets partial credit",
			cfg:         makeConfig("Dell XPS 15 9570", 2, 95, goodThresholds),
			productName: "Lenovo IdeaPad 5 15ARE05",
			fanCount:    1,
			wantScore:   55, // 0 + 10 + 25 + 20
		},
		{
			name:        "narrow thresholds get partial credit",
			cfg:         makeConfig("Dell XPS 15 9570", 1, 95, narrowThresholds),
			productName: "Lenovo IdeaPad 5 15ARE05",
			fanCount:    1,
			wantScore:   60, // 0 + 25 + 15 + 20
		},
		{
			name:        "few thresholds get no threshold credit",
			cfg:         makeConfig("Dell XPS 15 9570", 1, 95, fewThresholds),
			productName: "Lenovo IdeaPad 5 15ARE05",
			fanCount:    1,
			wantScore:   45, // 0 + 25 + 0 + 20
		},
		{
			name:        "critical temp = 0 gets no critical credit",
			cfg:         makeConfig("Dell XPS 15 9570", 1, 0, goodThresholds),
			productName: "Lenovo IdeaPad 5 15ARE05",
			fanCount:    1,
			wantScore:   50, // 0 + 25 + 25 + 0
		},
		{
			name:        "critical temp > 100 gets no critical credit",
			cfg:         makeConfig("Dell XPS 15 9570", 1, 150, goodThresholds),
			productName: "Lenovo IdeaPad 5 15ARE05",
			fanCount:    1,
			wantScore:   50, // 0 + 25 + 25 + 0
		},
		{
			name:        "empty product name skips model matching",
			cfg:         makeConfig("Dell XPS 15 9570", 1, 95, goodThresholds),
			productName: "",
			fanCount:    1,
			wantScore:   70, // 0 + 25 + 25 + 20
		},
		{
			name:        "dual fan match",
			cfg:         makeConfig("ASUS ROG Zephyrus G14", 2, 95, goodThresholds),
			productName: "ASUS ROG Zephyrus G14",
			fanCount:    2,
			wantScore:   100, // 30 + 25 + 25 + 20
		},
		{
			name:        "fanCount 0 — has fan configs gets partial credit",
			cfg:         makeConfig("Dell XPS 15 9570", 1, 95, goodThresholds),
			productName: "Lenovo IdeaPad 5 15ARE05",
			fanCount:    0,
			wantScore:   55, // 0 + 10 + 25 + 20
		},
		{
			name:        "config file not found returns 0",
			cfg:         nil,
			productName: "Anything",
			fanCount:    1,
			wantScore:   0,
		},
		{
			name:        "no fan configurations at all",
			cfg:         makeConfig("Dell XPS 15 9570", 0, 95, goodThresholds),
			productName: "Lenovo IdeaPad 5 15ARE05",
			fanCount:    1,
			wantScore:   20, // 0 + 0 + 0 + 20  (no fans → no fan match, no threshold check)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			withTestConfigDir(t, dir)

			cfgName := "nonexistent-config"
			if tt.cfg != nil {
				cfgName = tt.cfg.NotebookModel
				writeTestConfig(t, dir, tt.cfg)
			}

			got := RateConfig(cfgName, tt.productName, tt.fanCount)
			assert.Equal(t, tt.wantScore, got, "score mismatch")
		})
	}
}

// ===========================================================================
// TASK-07: Unit tests for WriteConfigFile
// ===========================================================================

func TestWriteConfigFile(t *testing.T) {
	t.Run("write preserves unknown JSON fields", func(t *testing.T) {
		dir := t.TempDir()
		withTestConfigDir(t, dir)
		withTestHooks(t)

		// Copy the sample config fixture into the temp configDir.
		fixtureData, err := os.ReadFile(filepath.Join("testdata", "sample_config.json"))
		require.NoError(t, err)

		cfgPath := filepath.Join(dir, "Test Model X1.json")
		require.NoError(t, os.WriteFile(cfgPath, fixtureData, 0644))

		// Read config through our struct (unknown fields are dropped).
		cfg, err := ReadConfigFile("Test Model X1")
		require.NoError(t, err)

		// Modify a threshold to prove we're writing, not just copying.
		cfg.FanConfigurations[0].TemperatureThresholds[0].FanSpeed = 25

		// Write it back — should preserve unknown fields via surgical merge.
		err = WriteConfigFile(cfg)
		require.NoError(t, err)

		// Read the written file as a raw map.
		writtenData, err := os.ReadFile(cfgPath)
		require.NoError(t, err)

		var result map[string]interface{}
		require.NoError(t, json.Unmarshal(writtenData, &result))

		// Top-level unknown fields preserved.
		assert.Equal(t, "preserve-me", result["SomeUnknownField"],
			"top-level unknown field should be preserved")
		assert.Equal(t, float64(42), result["AnotherExtra"],
			"top-level unknown numeric field should be preserved")

		// Fan-level unknown fields preserved.
		fans, ok := result["FanConfigurations"].([]interface{})
		require.True(t, ok)
		require.Len(t, fans, 1)
		fan0 := fans[0].(map[string]interface{})
		assert.Equal(t, "also-preserve-me", fan0["ExtraFanField"],
			"fan-level unknown field should be preserved")

		// Verify our modification was applied.
		thresholds := fan0["TemperatureThresholds"].([]interface{})
		thresh0 := thresholds[0].(map[string]interface{})
		assert.Equal(t, float64(25), thresh0["FanSpeed"],
			"modified threshold should be reflected in output")
	})

	t.Run("backup created before write", func(t *testing.T) {
		dir := t.TempDir()
		withTestConfigDir(t, dir)
		withTestHooks(t)

		// Create an original config.
		originalCfg := &Config{
			NotebookModel:       "BackupTest",
			CriticalTemperature: 90,
			FanConfigurations: []FanConfiguration{
				{
					FanDisplayName: "Test Fan",
					TemperatureThresholds: []Threshold{
						{UpThreshold: 50, DownThreshold: 45, FanSpeed: 30},
					},
				},
			},
		}
		writeTestConfig(t, dir, originalCfg)

		origPath := filepath.Join(dir, "BackupTest.json")
		origData, err := os.ReadFile(origPath)
		require.NoError(t, err)

		// Modify and write.
		originalCfg.FanConfigurations[0].TemperatureThresholds[0].FanSpeed = 50
		err = WriteConfigFile(originalCfg)
		require.NoError(t, err)

		// Verify backup exists and matches original content.
		bakPath := filepath.Join(dir, "BackupTest.json.bak")
		bakData, err := os.ReadFile(bakPath)
		require.NoError(t, err)
		assert.Equal(t, origData, bakData, "backup should match original file content")

		// Verify the config file itself was updated (not identical to backup).
		updatedData, err := os.ReadFile(origPath)
		require.NoError(t, err)
		assert.NotEqual(t, origData, updatedData, "config file should differ from backup after write")
	})

	t.Run("file content matches expected structure", func(t *testing.T) {
		dir := t.TempDir()
		withTestConfigDir(t, dir)
		withTestHooks(t)

		cfg := &Config{
			NotebookModel:       "ContentTest",
			EcPollInterval:      3000,
			CriticalTemperature: 95,
			FanConfigurations: []FanConfiguration{
				{
					ReadRegister:   16,
					WriteRegister:  16,
					MinSpeedValue:  0,
					MaxSpeedValue:  255,
					FanDisplayName: "Main Fan",
					TemperatureThresholds: []Threshold{
						{UpThreshold: 45, DownThreshold: 40, FanSpeed: 20},
						{UpThreshold: 60, DownThreshold: 55, FanSpeed: 50},
						{UpThreshold: 80, DownThreshold: 75, FanSpeed: 80},
						{UpThreshold: 90, DownThreshold: 85, FanSpeed: 100},
					},
				},
			},
		}

		// No original file — should write struct directly.
		err := WriteConfigFile(cfg)
		require.NoError(t, err)

		// Read back and compare.
		resultPath := filepath.Join(dir, "ContentTest.json")
		data, err := os.ReadFile(resultPath)
		require.NoError(t, err)

		var result Config
		require.NoError(t, json.Unmarshal(data, &result))

		assert.Equal(t, "ContentTest", result.NotebookModel)
		assert.Equal(t, 3000, result.EcPollInterval)
		assert.Equal(t, float64(95), result.CriticalTemperature)
		require.Len(t, result.FanConfigurations, 1)
		assert.Equal(t, "Main Fan", result.FanConfigurations[0].FanDisplayName)
		require.Len(t, result.FanConfigurations[0].TemperatureThresholds, 4)
		assert.Equal(t, float64(20), result.FanConfigurations[0].TemperatureThresholds[0].FanSpeed)
		assert.Equal(t, float64(100), result.FanConfigurations[0].TemperatureThresholds[3].FanSpeed)
	})

	t.Run("write without original file creates new file — no backup", func(t *testing.T) {
		dir := t.TempDir()
		withTestConfigDir(t, dir)
		withTestHooks(t)

		cfg := &Config{
			NotebookModel:       "NewConfig",
			CriticalTemperature: 85,
			FanConfigurations: []FanConfiguration{
				{
					FanDisplayName: "CPU Fan",
					TemperatureThresholds: []Threshold{
						{UpThreshold: 50, DownThreshold: 45, FanSpeed: 40},
					},
				},
			},
		}

		err := WriteConfigFile(cfg)
		require.NoError(t, err)

		// File should exist.
		cfgPath := filepath.Join(dir, "NewConfig.json")
		_, err = os.Stat(cfgPath)
		assert.NoError(t, err, "config file should exist after write")

		// No backup should exist (no original file).
		bakPath := filepath.Join(dir, "NewConfig.json.bak")
		_, err = os.Stat(bakPath)
		assert.True(t, os.IsNotExist(err), "no backup expected when no original exists")
	})
}
