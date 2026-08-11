package pullrequestreviewcontext

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Songmu/skillsmith"
)

const (
	skillInstallCommand   = "install"
	skillUpdateCommand    = "update"
	skillReinstallCommand = "reinstall"
	skillUninstallCommand = "uninstall"
	skillStatusCommand    = "status"
)

func runSkills(ctx context.Context, args []string, output, errorOutput io.Writer) error {
	normalizedArgs, err := normalizeSkillArguments(args)
	if err != nil {
		return err
	}

	smith, err := skillsmith.New("pull-request-review-context", appVersion, embeddedSkills)
	if err != nil {
		return fmt.Errorf("create skill manager: %w", err)
	}
	smith.OutWriter = output
	smith.ErrWriter = errorOutput
	return smith.Run(ctx, normalizedArgs)
}

func normalizeSkillArguments(args []string) ([]string, error) {
	if len(args) == 0 {
		return args, nil
	}

	command := args[0]
	mutating := command == skillInstallCommand ||
		command == skillUpdateCommand ||
		command == skillReinstallCommand ||
		command == skillUninstallCommand
	needsInstallDirectory := mutating || command == skillStatusCommand
	if !needsInstallDirectory {
		return args, nil
	}

	hasApply := false
	hasDryRun := false
	hasInstallDirectory := false
	for _, arg := range args[1:] {
		switch {
		case arg == "--apply":
			hasApply = true
		case arg == "--dry-run":
			hasDryRun = true
		case arg == "--prefix", arg == "--scope":
			hasInstallDirectory = true
		case strings.HasPrefix(arg, "--prefix="), strings.HasPrefix(arg, "--scope="):
			hasInstallDirectory = true
		case strings.HasPrefix(arg, "--apply="):
			return nil, fmt.Errorf("--apply does not accept a value")
		}
	}
	if hasApply && hasDryRun {
		return nil, fmt.Errorf("--apply and --dry-run cannot be used together")
	}

	normalizedArgs := make([]string, 0, len(args)+3)
	normalizedArgs = append(normalizedArgs, command)
	for _, arg := range args[1:] {
		if arg != "--apply" {
			normalizedArgs = append(normalizedArgs, arg)
		}
	}
	if !hasInstallDirectory {
		prefix, err := defaultSkillPrefix()
		if err != nil {
			return nil, err
		}
		normalizedArgs = append(normalizedArgs, "--prefix", prefix)
	}
	if mutating && !hasApply && !hasDryRun {
		normalizedArgs = append(normalizedArgs, "--dry-run")
	}
	return normalizedArgs, nil
}

func defaultSkillPrefix() (string, error) {
	if codexHome := os.Getenv("CODEX_HOME"); codexHome != "" {
		return filepath.Join(codexHome, "skills"), nil
	}

	homeDirectory, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve Codex home directory: %w", err)
	}
	return filepath.Join(homeDirectory, ".codex", "skills"), nil
}
