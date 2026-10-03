package pullrequestreviewcontext

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	gh "github.com/nurazon59/pull-request-review-context/internal/github"
)

func TestExecuteDefaultsToCurrentBranchOpenPRAndFiltersResolvedOrOutdated(t *testing.T) {
	var calls [][]string
	runner := func(_ context.Context, name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string(nil), args...))
		if name != "gh" {
			t.Fatalf("command = %q, want gh", name)
		}
		if len(args) > 0 && args[0] == "pr" {
			return []byte(`{"number":42,"state":"OPEN","url":"https://github.com/upstream/project/pull/42"}`), nil
		}
		if len(args) > 1 && args[1] == "graphql" {
			return []byte(reviewThreadsResponse), nil
		}
		t.Fatalf("unexpected command arguments: %#v", args)
		return nil, nil
	}
	var buffer bytes.Buffer
	err := Execute(context.Background(), &buffer, gh.NewClientWithRunner(runner), Options{Format: "llm"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := "file: main.go\nline: 12\n\ncode:\nnew\n\nreview:\n未解決の指摘\n"
	if buffer.String() != want {
		t.Fatalf("Execute() = %q, want %q", buffer.String(), want)
	}
	if len(calls) != 2 || calls[0][0] != "pr" {
		t.Fatalf("commands = %#v, want current PR lookup and one GraphQL request", calls)
	}
	graphqlArgs := strings.Join(calls[1], " ")
	if !strings.Contains(graphqlArgs, "owner=upstream") || !strings.Contains(graphqlArgs, "name=project") {
		t.Fatalf("GraphQL query did not use PR base repository: %#v", calls[1])
	}
}

func TestExecuteAllIncludesStatusesAndCommentMetadata(t *testing.T) {
	runner := func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if len(args) > 1 && args[1] == "graphql" {
			return []byte(reviewThreadsResponse), nil
		}
		t.Fatalf("unexpected command arguments: %#v", args)
		return nil, nil
	}
	var buffer bytes.Buffer
	err := Execute(context.Background(), &buffer, gh.NewClientWithRunner(runner), Options{
		Repository: "upstream/project",
		Target:     "42",
		Format:     "json",
		All:        true,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var items []struct {
		Comments []struct {
			ID       int64  `json:"id"`
			ThreadID string `json:"thread_id"`
			URL      string `json:"url"`
			Path     string `json:"path"`
			Line     int    `json:"line"`
			Body     string `json:"body"`
			Resolved bool   `json:"resolved"`
			Outdated bool   `json:"outdated"`
		} `json:"comments"`
	}
	if err := json.Unmarshal(buffer.Bytes(), &items); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	var comments []struct {
		ID       int64  `json:"id"`
		ThreadID string `json:"thread_id"`
		URL      string `json:"url"`
		Path     string `json:"path"`
		Line     int    `json:"line"`
		Body     string `json:"body"`
		Resolved bool   `json:"resolved"`
		Outdated bool   `json:"outdated"`
	}
	for _, item := range items {
		comments = append(comments, item.Comments...)
	}
	if len(comments) != 3 {
		t.Fatalf("comment metadata count = %d, want 3; output=%s", len(comments), buffer.String())
	}
	if comments[0].ID != 101 || comments[0].ThreadID != "THREAD_UNRESOLVED" || comments[0].Path != "main.go" || comments[0].Line != 12 || comments[0].Resolved || comments[0].Outdated {
		t.Fatalf("unresolved comment = %#v", comments[0])
	}
	if !comments[1].Resolved || comments[1].Outdated || !comments[2].Outdated || comments[2].Line != 12 {
		t.Fatalf("resolved/outdated metadata = %#v, %#v", comments[1], comments[2])
	}
}

func TestExecuteCommentURLSelectsOnlyRequestedComment(t *testing.T) {
	response := `{"data":{"repository":{"pullRequest":{"reviewThreads":{"nodes":[{"id":"THREAD","isResolved":false,"isOutdated":false,"path":"main.go","line":12,"diffSide":"RIGHT","comments":{"nodes":[{"id":"COMMENT_1","fullDatabaseId":"101","url":"https://github.com/upstream/project/pull/42#discussion_r101","path":"main.go","body":"first","diffHunk":"@@ -11,1 +12,1 @@\n-old\n+new","line":12,"side":"RIGHT"},{"id":"COMMENT_2","fullDatabaseId":"102","url":"https://github.com/upstream/project/pull/42#discussion_r102","path":"main.go","body":"selected","diffHunk":"@@ -11,1 +12,1 @@\n-old\n+new","line":12,"side":"RIGHT"}],"pageInfo":{"hasNextPage":false,"endCursor":null}}}],"pageInfo":{"hasNextPage":false,"endCursor":null}}}}}}`
	runner := func(_ context.Context, _ string, args ...string) ([]byte, error) {
		return []byte(response), nil
	}
	var buffer bytes.Buffer
	err := Execute(context.Background(), &buffer, gh.NewClientWithRunner(runner), Options{
		Target: "https://github.com/upstream/project/pull/42#discussion_r102",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(buffer.String(), "selected") || strings.Contains(buffer.String(), "first") {
		t.Fatalf("output = %q, want only selected review comment", buffer.String())
	}
}

func TestExecuteAPICommentURLResolvesPullRequest(t *testing.T) {
	runner := func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if len(args) > 1 && args[0] == "api" && args[1] != "graphql" {
			return []byte("https://api.github.com/repos/upstream/project/pulls/42\n"), nil
		}
		if len(args) > 1 && args[1] == "graphql" {
			return []byte(reviewThreadsResponse), nil
		}
		t.Fatalf("unexpected command arguments: %#v", args)
		return nil, nil
	}
	var buffer bytes.Buffer
	err := Execute(context.Background(), &buffer, gh.NewClientWithRunner(runner), Options{
		Target: "https://api.github.com/repos/upstream/project/pulls/comments/101",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(buffer.String(), "未解決の指摘") || strings.Contains(buffer.String(), "解決済み") {
		t.Fatalf("output = %q, want only the API URL's unresolved comment", buffer.String())
	}
}

func TestExecuteExplicitCommentURLIncludesResolvedOrOutdated(t *testing.T) {
	runner := func(_ context.Context, _ string, args ...string) ([]byte, error) {
		return []byte(reviewThreadsResponse), nil
	}
	for _, test := range []struct{ id, body, status string }{
		{"102", "解決済み", "resolved"},
		{"103", "古い指摘", "outdated"},
	} {
		t.Run(test.status, func(t *testing.T) {
			var buffer bytes.Buffer
			err := Execute(context.Background(), &buffer, gh.NewClientWithRunner(runner), Options{
				Target: "https://github.com/upstream/project/pull/42#discussion_r" + test.id,
			})
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if !strings.Contains(buffer.String(), test.body) || !strings.Contains(buffer.String(), "review status: "+test.status) || strings.Contains(buffer.String(), "未解決の指摘") {
				t.Fatalf("output = %q, want only selected %s comment", buffer.String(), test.status)
			}
		})
	}
}

func TestExecuteMovedCurrentCommentUsesOriginalHunkCoordinates(t *testing.T) {
	response := `{"data":{"repository":{"pullRequest":{"reviewThreads":{"nodes":[{"id":"THREAD","isResolved":false,"isOutdated":false,"path":"main.go","line":13,"originalLine":12,"diffSide":"RIGHT","comments":{"nodes":[{"fullDatabaseId":"101","body":"moved review","line":13,"originalLine":12,"diffHunk":"@@ -12,1 +12,2 @@\n-original\n+target\n+unrelated"}]}}]}}}}}`
	runner := func(context.Context, string, ...string) ([]byte, error) { return []byte(response), nil }
	var buffer bytes.Buffer
	err := Execute(context.Background(), &buffer, gh.NewClientWithRunner(runner), Options{Target: "42", Repository: "owner/repository"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buffer.String(), "code:\ntarget\n") || strings.Contains(buffer.String(), "unrelated") {
		t.Fatalf("output = %q, want the original target code", buffer.String())
	}
}

func TestExecuteReportsMissingSelectedComment(t *testing.T) {
	runner := func(_ context.Context, _ string, args ...string) ([]byte, error) {
		return []byte(reviewThreadsResponse), nil
	}
	var buffer bytes.Buffer
	err := Execute(context.Background(), &buffer, gh.NewClientWithRunner(runner), Options{
		Target: "https://github.com/upstream/project/pull/42#discussion_r999",
	})
	if err == nil || !strings.Contains(err.Error(), "comment 999 was not found") {
		t.Fatalf("Execute() error = %v, want missing comment error", err)
	}
}

func TestExecutePaginationFailureProducesNoPartialOutput(t *testing.T) {
	calls := 0
	runner := func(context.Context, string, ...string) ([]byte, error) {
		calls++
		if calls > 1 {
			return nil, errors.New("GitHub API unavailable")
		}
		return []byte(strings.ReplaceAll(reviewThreadsResponse, `"hasNextPage":false,"endCursor":null`, `"hasNextPage":true,"endCursor":"next"`)), nil
	}
	var buffer bytes.Buffer
	err := Execute(context.Background(), &buffer, gh.NewClientWithRunner(runner), Options{Target: "42", Repository: "owner/project"})
	if err == nil || !strings.Contains(err.Error(), "GitHub API unavailable") {
		t.Fatalf("error = %v, want API failure", err)
	}
	if buffer.Len() != 0 {
		t.Fatalf("partial output = %q, want empty stdout on fetch failure", buffer.String())
	}
}

func TestCommentTargetUsesOriginalLineForOutdatedThread(t *testing.T) {
	currentLine := 18
	originalLine := 12
	thread := gh.ReviewThread{
		IsOutdated:   true,
		Line:         &currentLine,
		OriginalLine: &originalLine,
		DiffSide:     "RIGHT",
	}
	comment := gh.ReviewComment{
		Line:         &currentLine,
		OriginalLine: &originalLine,
		DiffHunk:     "@@ -11,1 +12,1 @@\n-old\n+new",
	}
	target, ok := commentTargetFor(thread, comment)
	if !ok || target.Line == nil || *target.Line != originalLine {
		t.Fatalf("comment target = %#v, ok=%v; want original line %d", target, ok, originalLine)
	}
}

func TestExecuteRejectsAmbiguousArgumentsAndUnknownFormat(t *testing.T) {
	tests := []struct {
		name    string
		options Options
		wantErr string
	}{
		{name: "target and legacy PR number", options: Options{Target: "42", PullRequest: 43}, wantErr: "either a target or pull request number"},
		{name: "unknown format", options: Options{Target: "42", Repository: "owner/repository", Format: "yaml"}, wantErr: "unsupported output format"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := Execute(context.Background(), &bytes.Buffer{}, gh.NewClientWithRunner(func(context.Context, string, ...string) ([]byte, error) {
				return nil, nil
			}), test.options)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("Execute() error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

const reviewThreadsResponse = `{"data":{"repository":{"pullRequest":{"reviewThreads":{"nodes":[{"id":"THREAD_UNRESOLVED","isResolved":false,"isOutdated":false,"path":"main.go","line":12,"diffSide":"RIGHT","comments":{"nodes":[{"id":"COMMENT_UNRESOLVED","fullDatabaseId":"101","url":"https://github.com/upstream/project/pull/42#discussion_r101","path":"main.go","body":"未解決の指摘","diffHunk":"@@ -11,1 +12,1 @@\n-old\n+new","line":12,"side":"RIGHT"}],"pageInfo":{"hasNextPage":false,"endCursor":null}}},{"id":"THREAD_RESOLVED","isResolved":true,"isOutdated":false,"path":"main.go","line":12,"diffSide":"RIGHT","comments":{"nodes":[{"id":"COMMENT_RESOLVED","fullDatabaseId":"102","url":"https://github.com/upstream/project/pull/42#discussion_r102","path":"main.go","body":"解決済み","diffHunk":"@@ -11,1 +12,1 @@\n-old\n+new","line":12,"side":"RIGHT"}],"pageInfo":{"hasNextPage":false,"endCursor":null}}},{"id":"THREAD_OUTDATED","isResolved":false,"isOutdated":true,"path":"main.go","line":null,"originalLine":12,"diffSide":"RIGHT","comments":{"nodes":[{"id":"COMMENT_OUTDATED","fullDatabaseId":"103","url":"https://github.com/upstream/project/pull/42#discussion_r103","path":"main.go","body":"古い指摘","diffHunk":"@@ -11,1 +12,1 @@\n-old\n+new","line":null,"originalLine":12,"side":"RIGHT"}],"pageInfo":{"hasNextPage":false,"endCursor":null}}}],"pageInfo":{"hasNextPage":false,"endCursor":null}}}}}}`
