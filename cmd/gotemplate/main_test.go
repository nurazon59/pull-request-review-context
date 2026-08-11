package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func buildBinary(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	bin := filepath.Join(dir, "go-template")

	cmd := exec.Command("go", "build", "-o", bin, "./cmd/gotemplate")
	cmd.Dir = "../.."
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")

	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))

	return bin
}

func TestVersion(t *testing.T) {
	bin := buildBinary(t)

	cmd := exec.Command(bin, "--version")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	require.Equal(t, "v0.1.0\n", string(out))
}

func TestHelp(t *testing.T) {
	bin := buildBinary(t)

	cmd := exec.Command(bin, "--help")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))

	require.True(t, bytes.Contains(out, []byte("Usage: go-template [flags]")))
	require.True(t, bytes.Contains(out, []byte("--config=STRING")))
	require.True(t, bytes.Contains(out, []byte("--version")))
}
