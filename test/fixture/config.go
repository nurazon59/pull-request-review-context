package fixture

import (
	"os"
	"path/filepath"
)

func (f *Fixture) ConfigYAML(content string) string {
	path := filepath.Join(f.t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		f.t.Fatalf("failed to write config: %v", err)
	}
	return path
}
