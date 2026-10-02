package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
)

type ReviewComment struct {
	GraphQLID         string          `json:"id"`
	FullDatabaseID    json.RawMessage `json:"fullDatabaseId"`
	LegacyDatabaseID  json.RawMessage `json:"databaseId"`
	URL               string          `json:"url"`
	Path              string          `json:"path"`
	Body              string          `json:"body"`
	DiffHunk          string          `json:"diffHunk"`
	StartLine         *int            `json:"startLine"`
	OriginalStartLine *int            `json:"originalStartLine"`
	Line              *int            `json:"line"`
	OriginalLine      *int            `json:"originalLine"`
}

func (c ReviewComment) NumericID() (int64, error) {
	for _, raw := range []json.RawMessage{c.FullDatabaseID, c.LegacyDatabaseID} {
		if len(raw) == 0 || string(raw) == "null" {
			continue
		}
		value := strings.Trim(string(raw), `"`)
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id < 1 {
			return 0, fmt.Errorf("invalid review comment database ID %q", value)
		}
		return id, nil
	}
	if c.URL != "" {
		if target, err := ParseTarget(c.URL); err == nil && target.CommentID > 0 {
			return target.CommentID, nil
		}
	}
	return 0, fmt.Errorf("review comment %s has no database ID", c.GraphQLID)
}

type PageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

type ReviewThread struct {
	ID                string         `json:"id"`
	IsResolved        bool           `json:"isResolved"`
	IsOutdated        bool           `json:"isOutdated"`
	Path              string         `json:"path"`
	Line              *int           `json:"line"`
	OriginalLine      *int           `json:"originalLine"`
	StartLine         *int           `json:"startLine"`
	OriginalStartLine *int           `json:"originalStartLine"`
	DiffSide          string         `json:"diffSide"`
	StartDiffSide     string         `json:"startDiffSide"`
	Comments          reviewComments `json:"comments"`
}

type reviewComments struct {
	Nodes    []ReviewComment `json:"nodes"`
	PageInfo PageInfo        `json:"pageInfo"`
}

type PullRequest struct {
	Number     int
	State      string
	Repository string
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

func (c Client) ListReviewThreads(ctx context.Context, repository string, pullRequest int) ([]ReviewThread, error) {
	normalizedRepository, err := NormalizeRepository(repository)
	if err != nil {
		return nil, err
	}
	if pullRequest < 1 {
		return nil, errors.New("pull request number must be greater than zero")
	}
	parts := strings.Split(normalizedRepository, "/")

	var result []ReviewThread
	var after string
	seenCursors := make(map[string]struct{})
	for {
		page, err := c.reviewThreadPage(ctx, parts[0], parts[1], pullRequest, after)
		if err != nil {
			return nil, err
		}
		for i := range page.Nodes {
			thread := page.Nodes[i]
			if thread.ID == "" {
				return nil, errors.New("GitHub returned a review thread without an ID")
			}
			if err := c.completeThreadComments(ctx, &thread); err != nil {
				return nil, fmt.Errorf("paginate comments for review thread %s: %w", thread.ID, err)
			}
			result = append(result, thread)
		}
		if !page.PageInfo.HasNextPage {
			break
		}
		if page.PageInfo.EndCursor == "" {
			return nil, errors.New("GitHub returned hasNextPage without an endCursor for review threads")
		}
		if _, exists := seenCursors[page.PageInfo.EndCursor]; exists {
			return nil, fmt.Errorf("GitHub repeated review thread cursor %q", page.PageInfo.EndCursor)
		}
		seenCursors[page.PageInfo.EndCursor] = struct{}{}
		after = page.PageInfo.EndCursor
	}
	return result, nil
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

func (c Client) CurrentPullRequest(ctx context.Context, repository string) (PullRequest, error) {
	args := []string{"pr", "view", "--json", "number,state,url"}
	if repository != "" {
		normalizedRepository, err := NormalizeRepository(repository)
		if err != nil {
			return PullRequest{}, err
		}
		args = append(args, "--repo", normalizedRepository)
	}
	data, err := c.run(ctx, "gh", args...)
	if err != nil {
		return PullRequest{}, fmt.Errorf("resolve current branch pull request: %w", err)
	}
	var result struct {
		Number int    `json:"number"`
		State  string `json:"state"`
		URL    string `json:"url"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return PullRequest{}, fmt.Errorf("decode current branch pull request: %w", err)
	}
	if result.Number < 1 {
		return PullRequest{}, errors.New("current branch is not associated with a pull request")
	}
	if !strings.EqualFold(result.State, "OPEN") {
		return PullRequest{}, fmt.Errorf("current branch pull request #%d is %s, not open", result.Number, strings.ToLower(result.State))
	}
	selector, err := ParseTarget(result.URL)
	if err != nil {
		return PullRequest{}, fmt.Errorf("resolve pull request base repository: %w", err)
	}
	if selector.PullRequest != result.Number || selector.Repository == "" {
		return PullRequest{}, fmt.Errorf("GitHub returned an invalid pull request URL %q", result.URL)
	}
	if err := ValidateRepository(selector.Repository); err != nil {
		return PullRequest{}, fmt.Errorf("resolve pull request base repository: %w", err)
	}
	return PullRequest{Number: result.Number, State: result.State, Repository: selector.Repository}, nil
}

func (c Client) PullRequestForComment(ctx context.Context, repository string, commentID int64) (int, error) {
	normalizedRepository, err := NormalizeRepository(repository)
	if err != nil {
		return 0, err
	}
	if commentID < 1 {
		return 0, errors.New("review comment ID must be greater than zero")
	}
	data, err := c.run(ctx, "gh", "api", "--jq", ".pull_request_url", fmt.Sprintf("repos/%s/pulls/comments/%d", normalizedRepository, commentID))
	if err != nil {
		return 0, fmt.Errorf("resolve pull request for review comment %d: %w", commentID, err)
	}
	selector, err := ParseTarget(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("decode pull request URL for review comment %d: %w", commentID, err)
	}
	if selector.PullRequest < 1 {
		return 0, fmt.Errorf("GitHub returned no pull request number for review comment %d", commentID)
	}
	return selector.PullRequest, nil
}

func (c Client) reviewThreadPage(ctx context.Context, owner, name string, number int, after string) (reviewThreadPage, error) {
	variables := map[string]string{
		"owner":  owner,
		"name":   name,
		"number": fmt.Sprint(number),
	}
	if after != "" {
		variables["after"] = after
	}
	data, err := c.graphQL(ctx, reviewThreadsQuery, variables)
	if err != nil {
		return reviewThreadPage{}, err
	}
	var response struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
		Data struct {
			Repository *struct {
				PullRequest *struct {
					ReviewThreads reviewThreadConnection `json:"reviewThreads"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return reviewThreadPage{}, fmt.Errorf("decode GitHub review threads: %w", err)
	}
	if len(response.Errors) > 0 {
		messages := make([]string, 0, len(response.Errors))
		for _, graphQLError := range response.Errors {
			messages = append(messages, graphQLError.Message)
		}
		return reviewThreadPage{}, fmt.Errorf("GitHub GraphQL query failed: %s", strings.Join(messages, "; "))
	}
	if response.Data.Repository == nil || response.Data.Repository.PullRequest == nil {
		return reviewThreadPage{}, fmt.Errorf("pull request #%d was not found or is inaccessible in %s/%s", number, owner, name)
	}
	connection := response.Data.Repository.PullRequest.ReviewThreads
	return reviewThreadPage{Nodes: connection.Nodes, PageInfo: connection.PageInfo}, nil
}

func (c Client) completeThreadComments(ctx context.Context, thread *ReviewThread) error {
	var after string
	seenCursors := make(map[string]struct{})
	for thread.Comments.PageInfo.HasNextPage {
		if thread.Comments.PageInfo.EndCursor == "" {
			return errors.New("GitHub returned hasNextPage without an endCursor for comments")
		}
		if _, exists := seenCursors[thread.Comments.PageInfo.EndCursor]; exists {
			return fmt.Errorf("GitHub repeated review comment cursor %q", thread.Comments.PageInfo.EndCursor)
		}
		after = thread.Comments.PageInfo.EndCursor
		seenCursors[after] = struct{}{}
		page, err := c.reviewCommentPage(ctx, thread.ID, after)
		if err != nil {
			return err
		}
		thread.Comments.Nodes = append(thread.Comments.Nodes, page.Nodes...)
		thread.Comments.PageInfo = page.PageInfo
	}
	return nil
}

func (c Client) reviewCommentPage(ctx context.Context, threadID, after string) (reviewComments, error) {
	variables := map[string]string{"id": threadID}
	if after != "" {
		variables["after"] = after
	}
	data, err := c.graphQL(ctx, reviewThreadCommentsQuery, variables)
	if err != nil {
		return reviewComments{}, err
	}
	var response struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
		Data struct {
			Node *struct {
				Comments reviewComments `json:"comments"`
			} `json:"node"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return reviewComments{}, fmt.Errorf("decode GitHub review comments: %w", err)
	}
	if len(response.Errors) > 0 {
		messages := make([]string, 0, len(response.Errors))
		for _, graphQLError := range response.Errors {
			messages = append(messages, graphQLError.Message)
		}
		return reviewComments{}, fmt.Errorf("GitHub GraphQL query failed: %s", strings.Join(messages, "; "))
	}
	if response.Data.Node == nil {
		return reviewComments{}, errors.New("GitHub review thread was not found while fetching comments")
	}
	return response.Data.Node.Comments, nil
}

func (c Client) graphQL(ctx context.Context, query string, variables map[string]string) ([]byte, error) {
	args := []string{"api", "graphql", "-f", "query=" + query}
	for _, key := range []string{"owner", "name", "number", "id", "after"} {
		if value, ok := variables[key]; ok {
			args = append(args, "-F", key+"="+value)
		}
	}
	data, err := c.run(ctx, "gh", args...)
	if err != nil {
		return nil, err
	}
	return data, nil
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

type reviewThreadConnection struct {
	Nodes    []ReviewThread `json:"nodes"`
	PageInfo PageInfo       `json:"pageInfo"`
}

type reviewThreadPage struct {
	Nodes    []ReviewThread
	PageInfo PageInfo
}

const reviewThreadsQuery = `query($owner: String!, $name: String!, $number: Int!, $after: String) {
  repository(owner: $owner, name: $name) {
    pullRequest(number: $number) {
      reviewThreads(first: 100, after: $after) {
        nodes {
          id isResolved isOutdated path line originalLine startLine originalStartLine diffSide startDiffSide
          comments(first: 100) {
            nodes { id fullDatabaseId url path body diffHunk line originalLine startLine originalStartLine }
            pageInfo { hasNextPage endCursor }
          }
        }
        pageInfo { hasNextPage endCursor }
      }
    }
  }
}`

const reviewThreadCommentsQuery = `query($id: ID!, $after: String) {
  node(id: $id) {
    ... on PullRequestReviewThread {
      comments(first: 100, after: $after) {
        nodes { id fullDatabaseId url path body diffHunk line originalLine startLine originalStartLine }
        pageInfo { hasNextPage endCursor }
      }
    }
  }
}`
