package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
)

type ReviewComment struct {
	ID        int64  `json:"id"`
	Path      string `json:"path"`
	Body      string `json:"body"`
	DiffHunk  string `json:"diff_hunk"`
	StartLine *int   `json:"start_line"`
	Line      *int   `json:"line"`
	StartSide string `json:"start_side"`
	Side      string `json:"side"`
}

type CommandRunner func(context.Context, string, ...string) ([]byte, error)

type Client struct {
	run CommandRunner
}

func NewClient() Client {
	return Client{run: runCommand}
}

func NewClientWithRunner(run CommandRunner) Client {
	return Client{run: run}
}

func (c Client) ListReviewComments(ctx context.Context, repository string, pullRequest int) ([]ReviewComment, error) {
	normalizedRepository, err := NormalizeRepository(repository)
	if err != nil {
		return nil, err
	}
	if pullRequest < 1 {
		return nil, errors.New("pull request number must be greater than zero")
	}

	endpoint := fmt.Sprintf("repos/%s/pulls/%d/comments?per_page=100", normalizedRepository, pullRequest)
	data, err := c.run(ctx, "gh", "api", "--paginate", "--slurp", "-H", "Accept: application/vnd.github+json", endpoint)
	if err != nil {
		return nil, err
	}

	var pages [][]ReviewComment
	if err := json.Unmarshal(data, &pages); err != nil {
		return nil, fmt.Errorf("decode review comments: %w", err)
	}
	comments := make([]ReviewComment, 0)
	for _, page := range pages {
		comments = append(comments, page...)
	}
	return comments, nil
}

func (c Client) CurrentRepository(ctx context.Context) (string, error) {
	data, err := c.run(ctx, "gh", "repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner")
	if err != nil {
		return "", err
	}
	repository := strings.TrimSpace(string(data))
	if err := ValidateRepository(repository); err != nil {
		return "", err
	}
	return repository, nil
}

func ValidateRepository(repository string) error {
	_, err := NormalizeRepository(repository)
	return err
}

func NormalizeRepository(repository string) (string, error) {
	if repository == "" {
		return "", errors.New("repository is required")
	}
	if strings.Contains(repository, "://") {
		parsed, err := url.Parse(repository)
		if err != nil {
			return "", fmt.Errorf("invalid repository: %w", err)
		}
		repository = strings.Trim(parsed.Path, "/")
	}
	repository = strings.TrimSuffix(repository, ".git")
	parts := strings.Split(repository, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.Contains(parts[0], " ") || strings.Contains(parts[1], " ") {
		return "", fmt.Errorf("repository must be in OWNER/REPOSITORY format: %q", repository)
	}
	return repository, nil
}

func runCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return nil, fmt.Errorf("%s failed: %w", name, err)
		}
		return nil, fmt.Errorf("%s failed: %w: %s", name, err, message)
	}
	return output, nil
}
