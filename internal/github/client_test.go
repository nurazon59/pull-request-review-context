package github

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestListReviewThreadsPaginatesThreadsAndComments(t *testing.T) {
	responses := []string{
		`{"data":{"repository":{"pullRequest":{"reviewThreads":{"nodes":[{"id":"THREAD_1","isResolved":false,"isOutdated":false,"path":"main.go","line":12,"diffSide":"RIGHT","comments":{"nodes":[{"id":"COMMENT_1","fullDatabaseId":"1","url":"https://github.com/owner/repository/pull/42#discussion_r1","path":"main.go","body":"first","diffHunk":"@@ -11,1 +11,1 @@\n-old\n+new","line":12,"side":"RIGHT"}],"pageInfo":{"hasNextPage":true,"endCursor":"comment-cursor"}}}],"pageInfo":{"hasNextPage":true,"endCursor":"thread-cursor"}}}}}}`,
		`{"data":{"node":{"comments":{"nodes":[{"id":"COMMENT_2","fullDatabaseId":"2","url":"https://github.com/owner/repository/pull/42#discussion_r2","path":"main.go","body":"reply","diffHunk":"@@ -11,1 +11,1 @@\n-old\n+new","line":12,"side":"RIGHT"}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}}`,
		`{"data":{"repository":{"pullRequest":{"reviewThreads":{"nodes":[{"id":"THREAD_2","isResolved":true,"isOutdated":true,"path":"main.go","line":null,"originalLine":12,"diffSide":"RIGHT","comments":{"nodes":[{"id":"COMMENT_3","fullDatabaseId":"3","url":"https://github.com/owner/repository/pull/42#discussion_r3","path":"main.go","body":"old","diffHunk":"@@ -11,1 +11,1 @@\n-old\n+new","line":null,"originalLine":12,"side":"RIGHT"}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}}}`,
	}
	var calls [][]string
	runner := func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "gh" {
			t.Fatalf("command = %q, want gh", name)
		}
		calls = append(calls, append([]string(nil), args...))
		index := len(calls) - 1
		if index >= len(responses) {
			t.Fatalf("unexpected call %d: %#v", index+1, args)
		}
		return []byte(responses[index]), nil
	}

	threads, err := NewClientWithRunner(runner).ListReviewThreads(context.Background(), "owner/repository", 42)
	if err != nil {
		t.Fatalf("ListReviewThreads() error = %v", err)
	}
	if len(threads) != 2 || len(threads[0].Comments.Nodes) != 2 || threads[1].ID != "THREAD_2" {
		t.Fatalf("threads = %#v", threads)
	}
	if !threads[1].IsResolved || !threads[1].IsOutdated {
		t.Fatalf("thread status = resolved:%v outdated:%v", threads[1].IsResolved, threads[1].IsOutdated)
	}
	if len(calls) != 3 {
		t.Fatalf("call count = %d, want 3", len(calls))
	}
	first := strings.Join(calls[0], " ")
	if !strings.Contains(first, "reviewThreads(first: 100") || !strings.Contains(first, "comments(first: 100)") {
		t.Fatalf("first query does not request paginated connections: %s", first)
	}
	if !strings.Contains(first, "fullDatabaseId") {
		t.Fatalf("query does not request the 64-bit comment ID: %s", first)
	}
	if !reflect.DeepEqual(calls[1][len(calls[1])-2:], []string{"-F", "after=comment-cursor"}) {
		t.Fatalf("comment pagination arguments = %#v", calls[1])
	}
	if !reflect.DeepEqual(calls[2][len(calls[2])-2:], []string{"-F", "after=thread-cursor"}) {
		t.Fatalf("thread pagination arguments = %#v", calls[2])
	}
	if !strings.Contains(strings.Join(calls[0], " "), "owner=owner") || !strings.Contains(strings.Join(calls[0], " "), "name=repository") {
		t.Fatalf("repository arguments = %#v", calls[0])
	}
}

func TestCurrentRepository(t *testing.T) {
	runner := func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "gh" {
			t.Fatalf("command = %q, want gh", name)
		}
		want := []string{"repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner"}
		if !reflect.DeepEqual(args, want) {
			t.Fatalf("arguments = %#v, want %#v", args, want)
		}
		return []byte("owner/repository\n"), nil
	}

	repository, err := NewClientWithRunner(runner).CurrentRepository(context.Background())
	if err != nil {
		t.Fatalf("CurrentRepository() error = %v", err)
	}
	if repository != "owner/repository" {
		t.Fatalf("repository = %q, want owner/repository", repository)
	}
}

func TestCurrentPullRequestUsesBaseRepositoryForFork(t *testing.T) {
	var gotArgs []string
	runner := func(_ context.Context, name string, args ...string) ([]byte, error) {
		gotArgs = args
		return []byte(`{"number":42,"state":"OPEN","url":"https://github.com/upstream/project/pull/42"}`), nil
	}
	pullRequest, err := NewClientWithRunner(runner).CurrentPullRequest(context.Background(), "fork/project")
	if err != nil {
		t.Fatalf("CurrentPullRequest() error = %v", err)
	}
	if pullRequest.Number != 42 || pullRequest.Repository != "upstream/project" {
		t.Fatalf("pull request = %#v", pullRequest)
	}
	wantArgs := []string{"pr", "view", "--json", "number,state,url", "--repo", "fork/project"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("arguments = %#v, want %#v", gotArgs, wantArgs)
	}
}

func TestCurrentPullRequestRequiresOpen(t *testing.T) {
	runner := func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return []byte(`{"number":42,"state":"CLOSED","url":"https://github.com/owner/repository/pull/42"}`), nil
	}
	_, err := NewClientWithRunner(runner).CurrentPullRequest(context.Background(), "")
	if err == nil || !strings.Contains(err.Error(), "not open") {
		t.Fatalf("CurrentPullRequest() error = %v, want closed PR error", err)
	}
}

func TestPullRequestForComment(t *testing.T) {
	runner := func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "gh" {
			t.Fatalf("command = %q, want gh", name)
		}
		want := []string{"api", "--jq", ".pull_request_url", "repos/owner/repository/pulls/comments/123"}
		if !reflect.DeepEqual(args, want) {
			t.Fatalf("arguments = %#v, want %#v", args, want)
		}
		return []byte("https://api.github.com/repos/owner/repository/pulls/42\n"), nil
	}
	number, err := NewClientWithRunner(runner).PullRequestForComment(context.Background(), "owner/repository", 123)
	if err != nil {
		t.Fatalf("PullRequestForComment() error = %v", err)
	}
	if number != 42 {
		t.Fatalf("pull request number = %d, want 42", number)
	}
}

func TestListReviewThreadsRejectsGraphQLErrorsAndMissingPR(t *testing.T) {
	tests := []struct {
		name     string
		response string
		wantErr  string
	}{
		{name: "graphql error", response: `{"errors":[{"message":"Resource not accessible by integration"}]}`, wantErr: "Resource not accessible"},
		{name: "missing pull request", response: `{"data":{"repository":{"pullRequest":null}}}`, wantErr: "not found or is inaccessible"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := func(_ context.Context, _ string, _ ...string) ([]byte, error) {
				return []byte(test.response), nil
			}
			_, err := NewClientWithRunner(runner).ListReviewThreads(context.Background(), "owner/repository", 42)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("ListReviewThreads() error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestValidateRepository(t *testing.T) {
	tests := map[string]struct {
		repository string
		wantErr    bool
	}{
		"owner and repository": {repository: "owner/repository"},
		"github URL":           {repository: "https://github.com/owner/repository"},
		"missing repository":   {repository: "owner", wantErr: true},
		"empty owner":          {repository: "/repository", wantErr: true},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := ValidateRepository(test.repository)
			if (err != nil) != test.wantErr {
				t.Fatalf("ValidateRepository() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestParseTarget(t *testing.T) {
	tests := []struct {
		input      string
		wantRepo   string
		wantPR     int
		wantID     int64
		wantAPIURL bool
		wantErr    bool
	}{
		{input: "42", wantPR: 42},
		{input: "https://github.com/owner/repository/pull/42", wantRepo: "owner/repository", wantPR: 42},
		{input: "https://github.com/owner/repository/pull/42#discussion_r123", wantRepo: "owner/repository", wantPR: 42, wantID: 123},
		{input: "https://github.com/owner/repository/pull/42#discussion-123", wantRepo: "owner/repository", wantPR: 42, wantID: 123},
		{input: "https://api.github.com/repos/owner/repository/pulls/42", wantRepo: "owner/repository", wantPR: 42},
		{input: "https://api.github.com/repos/owner/repository/pulls/comments/123", wantRepo: "owner/repository", wantID: 123, wantAPIURL: true},
		{input: "https://github.com/owner/repository/issues/42", wantErr: true},
		{input: "https://github.com/owner/repository/pull/42#issuecomment-123", wantErr: true},
		{input: "0", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			target, err := ParseTarget(test.input)
			if (err != nil) != test.wantErr {
				t.Fatalf("ParseTarget() error = %v, wantErr %v", err, test.wantErr)
			}
			if err != nil {
				return
			}
			if target.Repository != test.wantRepo || target.PullRequest != test.wantPR || target.CommentID != test.wantID || target.CommentAPIURL != test.wantAPIURL {
				t.Fatalf("ParseTarget() = %#v, want repo=%q PR=%d comment=%d apiURL=%v", target, test.wantRepo, test.wantPR, test.wantID, test.wantAPIURL)
			}
		})
	}
}

func TestGraphQLQueryPaginatesAfterCursor(t *testing.T) {
	runner := func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if !strings.Contains(strings.Join(args, " "), fmt.Sprintf("after=%s", "cursor-2")) {
			t.Fatalf("arguments = %#v, missing cursor", args)
		}
		return []byte(`{"data":{"repository":{"pullRequest":{"reviewThreads":{"nodes":[],"pageInfo":{"hasNextPage":false,"endCursor":null}}}}}}`), nil
	}
	_, err := NewClientWithRunner(runner).reviewThreadPage(context.Background(), "owner", "repository", 42, "cursor-2")
	if err != nil {
		t.Fatalf("reviewThreadPage() error = %v", err)
	}
}
