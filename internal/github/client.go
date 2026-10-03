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
	ID                int64  `json:"id"`
	Path              string `json:"path"`
	Body              string `json:"body"`
	URL               string `json:"html_url"`
	DiffHunk          string `json:"diff_hunk"`
	StartLine         *int   `json:"start_line"`
	Line              *int   `json:"line"`
	OriginalStartLine *int   `json:"original_start_line"`
	OriginalLine      *int   `json:"original_line"`
	StartSide         string `json:"start_side"`
	Side              string `json:"side"`
	ThreadID          string
	Resolved          bool
	Outdated          bool
}

type CommandRunner func(context.Context, string, ...string) ([]byte, error)
type Client struct{ run CommandRunner }

func NewClient() Client                            { return Client{run: runCommand} }
func NewClientWithRunner(run CommandRunner) Client { return Client{run: run} }

func (c Client) ListReviewComments(ctx context.Context, repository string, pullRequest int) ([]ReviewComment, error) {
	repository, err := NormalizeRepository(repository)
	if err != nil {
		return nil, err
	}
	if pullRequest < 1 {
		return nil, errors.New("pull request number must be greater than zero")
	}
	var pages [][]ReviewComment
	endpoint := fmt.Sprintf("repos/%s/pulls/%d/comments?per_page=100", repository, pullRequest)
	if err := c.getJSON(ctx, &pages, "api", "--paginate", "--slurp", endpoint); err != nil {
		return nil, err
	}
	var comments []ReviewComment
	for _, page := range pages {
		comments = append(comments, page...)
	}
	if len(comments) == 0 {
		return comments, nil
	}
	states, err := c.reviewStates(ctx, repository, pullRequest)
	if err != nil {
		return nil, err
	}
	for i := range comments {
		comment := &comments[i]
		state, ok := states[comment.ID]
		if !ok {
			return nil, fmt.Errorf("review comment %d has no thread state; retry the request", comment.ID)
		}
		comment.ThreadID, comment.Resolved, comment.Outdated = state.ID, state.IsResolved, state.IsOutdated
		// REST diff_hunk retains the original commit's coordinates after lines move.
		if comment.OriginalLine != nil {
			comment.Line = comment.OriginalLine
		}
		if comment.OriginalStartLine != nil {
			comment.StartLine = comment.OriginalStartLine
		}
	}
	return comments, nil
}

type commentIDs struct {
	Nodes []struct {
		ID json.Number `json:"fullDatabaseId"`
	}
	TotalCount int
}
type reviewThread struct {
	ID         string
	IsResolved bool
	IsOutdated bool
	Comments   commentIDs
}
type graphQLPage struct {
	Data struct {
		Repository struct {
			PullRequest *struct {
				ReviewThreads struct{ Nodes []reviewThread }
			}
		}
		Node *reviewThread
	}
}

func (c Client) reviewStates(ctx context.Context, repository string, number int) (map[int64]reviewThread, error) {
	parts := strings.Split(repository, "/")
	var pages []graphQLPage
	if err := c.getJSON(ctx, &pages,
		"api", "graphql", "--paginate", "--slurp",
		"-f", "query="+reviewThreadsQuery,
		"-f", "owner="+parts[0], "-f", "name="+parts[1],
		"-F", fmt.Sprintf("number=%d", number),
	); err != nil {
		return nil, err
	}
	states := make(map[int64]reviewThread)
	for _, page := range pages {
		if page.Data.Repository.PullRequest == nil {
			return nil, fmt.Errorf("pull request #%d was not found in %s", number, repository)
		}
		for _, thread := range page.Data.Repository.PullRequest.ReviewThreads.Nodes {
			ids := thread.Comments.Nodes
			if len(ids) < thread.Comments.TotalCount {
				// Only long threads need a separate paginated request. Keep one pageInfo
				// per query so gh's automatic pagination follows the correct connection.
				var commentPages []graphQLPage
				if err := c.getJSON(ctx, &commentPages,
					"api", "graphql", "--paginate", "--slurp",
					"-f", "query="+reviewThreadCommentsQuery, "-f", "id="+thread.ID,
				); err != nil {
					return nil, err
				}
				ids = nil
				for _, cp := range commentPages {
					if cp.Data.Node == nil {
						return nil, errors.New("review thread disappeared while fetching comments")
					}
					ids = append(ids, cp.Data.Node.Comments.Nodes...)
				}
			}
			for _, node := range ids {
				id, err := node.ID.Int64()
				if err != nil || id < 1 {
					return nil, fmt.Errorf("invalid review comment ID %q", node.ID)
				}
				states[id] = thread
			}
		}
	}
	return states, nil
}

func (c Client) getJSON(ctx context.Context, result any, args ...string) error {
	data, err := c.run(ctx, "gh", args...)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, result); err != nil {
		return fmt.Errorf("decode gh output: %w", err)
	}
	return nil
}

func (c Client) CurrentRepository(ctx context.Context) (string, error) {
	data, err := c.run(ctx, "gh", "repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner")
	if err != nil {
		return "", err
	}
	return NormalizeRepository(strings.TrimSpace(string(data)))
}

func (c Client) CurrentPullRequest(ctx context.Context, repository string) (Target, error) {
	args := []string{"pr", "view", "--json", "state,url", "--jq",
		`if .state == "OPEN" then .url else error("current branch has no open pull request") end`}
	if repository != "" {
		repo, err := NormalizeRepository(repository)
		if err != nil {
			return Target{}, err
		}
		branch, err := c.run(ctx, "git", "symbolic-ref", "--quiet", "--short", "HEAD")
		if err != nil {
			return Target{}, fmt.Errorf("resolve current branch: %w", err)
		}
		args = []string{"pr", "list", "--repo", repo, "--head", strings.TrimSpace(string(branch)),
			"--state", "open", "--limit", "2", "--json", "url", "--jq",
			`if length == 1 then .[0].url else error("expected one open pull request; specify a PR number or URL") end`}
	}
	data, err := c.run(ctx, "gh", args...)
	if err != nil {
		return Target{}, fmt.Errorf("resolve current branch pull request: %w", err)
	}
	// The canonical PR URL identifies the base repository for fork PRs.
	return ParseTarget(strings.TrimSpace(string(data)))
}

func (c Client) PullRequestForComment(ctx context.Context, repository string, commentID int64) (int, error) {
	data, err := c.run(ctx, "gh", "api", "--jq", ".pull_request_url", fmt.Sprintf("repos/%s/pulls/comments/%d", repository, commentID))
	if err != nil {
		return 0, err
	}
	target, err := ParseTarget(strings.TrimSpace(string(data)))
	return target.PullRequest, err
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
		if parsed.User != nil || !strings.EqualFold(parsed.Host, "github.com") || (parsed.Scheme != "https" && parsed.Scheme != "http") {
			return "", errors.New("repository URL must use github.com")
		}
		repository = strings.Trim(parsed.Path, "/")
	}
	repository = strings.TrimSuffix(repository, ".git")
	parts := strings.Split(repository, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.ContainsAny(repository, " \t\r\n") {
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

const reviewThreadsQuery = `query($owner: String!, $name: String!, $number: Int!, $endCursor: String) {
 repository(owner: $owner, name: $name) {
  pullRequest(number: $number) {
   reviewThreads(first: 100, after: $endCursor) {
    nodes { id isResolved isOutdated comments(first: 100) { totalCount nodes { fullDatabaseId } } }
    pageInfo { hasNextPage endCursor }
   }
  }
 }
}`

const reviewThreadCommentsQuery = `query($id: ID!, $endCursor: String) {
 node(id: $id) {
  ... on PullRequestReviewThread {
   comments(first: 100, after: $endCursor) {
    nodes { fullDatabaseId }
    pageInfo { hasNextPage endCursor }
   }
  }
 }
}`
