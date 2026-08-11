package pullrequestreviewcontext

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunSkillsInstallDefaultsToDryRun(t *testing.T) {
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)
	var output bytes.Buffer
	var errorOutput bytes.Buffer

	err := runSkills(context.Background(), []string{"install"}, &output, &errorOutput)
	if err != nil {
		t.Fatalf("runSkills() error = %v", err)
	}
	if !strings.Contains(output.String(), "installed (dry-run): pull-request-review-context") {
		t.Fatalf("runSkills() output = %q", output.String())
	}
	if _, err := os.Stat(filepath.Join(codexHome, "skills")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created the installation directory: %v", err)
	}
}

func TestRunSkillsInstallWithApplyWritesEmbeddedSkill(t *testing.T) {
	installDirectory := t.TempDir()
	var output bytes.Buffer
	var errorOutput bytes.Buffer

	err := runSkills(context.Background(), []string{
		"install",
		"--prefix",
		installDirectory,
		"--apply",
	}, &output, &errorOutput)
	if err != nil {
		t.Fatalf("runSkills() error = %v", err)
	}

	skillPath := filepath.Join(installDirectory, "pull-request-review-context", "SKILL.md")
	content, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", skillPath, err)
	}
	if !strings.Contains(string(content), "name: pull-request-review-context") {
		t.Fatalf("SKILL.md does not contain the skill name: %q", content)
	}
	if !strings.Contains(output.String(), "installed: pull-request-review-context") {
		t.Fatalf("runSkills() output = %q", output.String())
	}
}

func TestRunSkillsInstallWithApplyUsesCodexHome(t *testing.T) {
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)
	var output bytes.Buffer
	var errorOutput bytes.Buffer

	err := runSkills(context.Background(), []string{"install", "--apply"}, &output, &errorOutput)
	if err != nil {
		t.Fatalf("runSkills() error = %v", err)
	}

	skillPath := filepath.Join(codexHome, "skills", "pull-request-review-context", "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("Stat(%q) error = %v", skillPath, err)
	}
}

func TestRunSkillsRejectsApplyWithDryRun(t *testing.T) {
	err := runSkills(context.Background(), []string{"install", "--apply", "--dry-run"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("runSkills() error = nil")
	}
	if !strings.Contains(err.Error(), "--apply and --dry-run cannot be used together") {
		t.Fatalf("runSkills() error = %v", err)
	}
}
