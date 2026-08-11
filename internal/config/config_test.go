package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	var loadTests = map[string]struct {
		configFile string
		want       int
	}{
		"config file": {
			configFile: "testdata/config.yaml",
			want:       1,
		},
		"default when file missing": {
			configFile: filepath.Join(t.TempDir(), "missing.yaml"),
			want:       1,
		},
		"invalid yaml": {
			configFile: func() string {
				dir := t.TempDir()
				path := filepath.Join(dir, "broken.yaml")
				_ = os.WriteFile(path, []byte(": invalid"), 0o644)
				return path
			}(),
			want: 0,
		},
	}

	for name, test := range loadTests {
		t.Run(name, func(t *testing.T) {
			cfg, err := Load(test.configFile)
			if name == "invalid yaml" {
				assert.Error(t, err)
				assert.Nil(t, cfg)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, test.want, cfg.Version)
		})
	}
}
