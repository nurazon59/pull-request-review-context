package github

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestListReviewCommentsPaginatesRESTThreadsAndLongThread(t *testing.T) {
	const baseID = 3000000000
	var ids []string
	for i := 0; i < 101; i++ {
		ids = append(ids, fmt.Sprintf(`{"fullDatabaseId":"%d"}`, baseID+i))
	}
	first100 := strings.Join(ids[:100], ",")
	responses := []string{
		fmt.Sprintf(`[[{"id":%d}],[{"id":%d},{"id":999}]]`, baseID, baseID+100),
		fmt.Sprintf(`[{"data":{"repository":{"pullRequest":{"reviewThreads":{"nodes":[{"id":"LONG","isResolved":true,"comments":{"totalCount":101,"nodes":[%s]}}]}}}}},{"data":{"repository":{"pullRequest":{"reviewThreads":{"nodes":[{"id":"NEXT","isOutdated":true,"comments":{"totalCount":1,"nodes":[{"fullDatabaseId":999}]}}]}}}}}]`, first100),
		fmt.Sprintf(`[{"data":{"node":{"comments":{"nodes":[%s]}}}},{"data":{"node":{"comments":{"nodes":[%s]}}}}]`, first100, ids[100]),
	}
	var calls [][]string
	client := NewClientWithRunner(func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "gh" {
			t.Fatalf("command %s", name)
		}
		calls = append(calls, args)
		if len(calls) > len(responses) {
			t.Fatal("unexpected request")
		}
		return []byte(responses[len(calls)-1]), nil
	})
	comments, err := client.ListReviewComments(context.Background(), "123/null", 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 3 || !comments[0].Resolved || !comments[1].Resolved || !comments[2].Outdated || comments[1].ID != baseID+100 {
		t.Fatalf("comments: %+v", comments)
	}
	for _, args := range calls {
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "--paginate --slurp") {
			t.Fatalf("pagination flags missing: %v", args)
		}
	}
	for _, args := range calls[1:] {
		query := strings.Join(args, " ")
		if !strings.Contains(query, "$endCursor: String") || strings.Count(query, "pageInfo") != 1 {
			t.Fatalf("query must expose one paginated connection: %s", query)
		}
	}
	if !strings.Contains(strings.Join(calls[1], " "), "-f owner=123 -f name=null -F number=42") {
		t.Fatalf("string variables coerced: %v", calls[1])
	}
	if !strings.Contains(strings.Join(calls[2], " "), "-f id=LONG") {
		t.Fatalf("long thread lookup: %v", calls[2])
	}
}

func TestListReviewCommentsFailures(t *testing.T) {
	for _, test := range []struct {
		name, rest, graphql, want string
		apiError                  bool
	}{
		{"invalid JSON", "broken", "", "decode", false},
		{"missing PR", `[[{"id":1}]]`, `[{"data":{"repository":{"pullRequest":null}}}]`, "not found", false},
		{"state missing", `[[{"id":1}]]`, `[{"data":{"repository":{"pullRequest":{"reviewThreads":{"nodes":[]}}}}}]`, "no thread state", false},
		{"API error", `[[{"id":1}]]`, "", "GitHub GraphQL error", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := NewClientWithRunner(func(_ context.Context, _ string, args ...string) ([]byte, error) {
				if args[1] == "graphql" {
					if test.apiError {
						return nil, errors.New(test.want)
					}
					return []byte(test.graphql), nil
				}
				return []byte(test.rest), nil
			})
			_, err := client.ListReviewComments(context.Background(), "owner/repository", 42)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v, want %s", err, test.want)
			}
		})
	}
}

func TestCurrentPullRequest(t *testing.T) {
	for _, repository := range []string{"", "upstream/project"} {
		t.Run(repository, func(t *testing.T) {
			client := NewClientWithRunner(func(_ context.Context, name string, args ...string) ([]byte, error) {
				if name == "git" {
					return []byte("123\n"), nil
				}
				joined := strings.Join(args, " ")
				if repository == "" && !strings.Contains(joined, `if .state == "OPEN"`) {
					t.Fatalf("must require open PR: %v", args)
				}
				if repository != "" && !strings.Contains(joined, "--repo upstream/project --head 123 --state open --limit 2") {
					t.Fatalf("current branch lookup: %v", args)
				}
				return []byte("https://github.com/upstream/project/pull/42\n"), nil
			})
			target, err := client.CurrentPullRequest(context.Background(), repository)
			if err != nil || target.Repository != "upstream/project" || target.PullRequest != 42 {
				t.Fatalf("target=%+v error=%v", target, err)
			}
		})
	}
}

func TestCurrentPullRequestFailure(t *testing.T) {
	for _, repository := range []string{"", "upstream/project"} {
		client := NewClientWithRunner(func(context.Context, string, ...string) ([]byte, error) {
			return nil, errors.New("no matching open PR or detached HEAD")
		})
		if _, err := client.CurrentPullRequest(context.Background(), repository); err == nil {
			t.Fatal("expected PR resolution error")
		}
	}
}

func TestCurrentRepository(t *testing.T) {
	runner := func(_ context.Context, name string, args ...string) ([]byte, error) {
		want := []string{"repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner"}
		if name != "gh" || !reflect.DeepEqual(args, want) {
			t.Fatalf("command=%s %v", name, args)
		}
		return []byte("owner/repository\n"), nil
	}
	repository, err := NewClientWithRunner(runner).CurrentRepository(context.Background())
	if err != nil || repository != "owner/repository" {
		t.Fatalf("repository=%s error=%v", repository, err)
	}
}

func TestParseTarget(t *testing.T) {
	for _, test := range []struct {
		input string
		want  Target
	}{
		{"42", Target{PullRequest: 42}},
		{"https://github.com/owner/repository/pull/42", Target{Repository: "owner/repository", PullRequest: 42}},
		{"https://github.com/owner/repository/pull/42/files#discussion_r123", Target{Repository: "owner/repository", PullRequest: 42, CommentID: 123}},
		{"https://github.com/owner/repository/pull/42#discussion-diff-123", Target{Repository: "owner/repository", PullRequest: 42, CommentID: 123}},
		{"https://api.github.com/repos/owner/repository/pulls/comments/123", Target{Repository: "owner/repository", CommentID: 123}},
		{"https://api.github.com/repos/owner/repository/pulls/42", Target{Repository: "owner/repository", PullRequest: 42}},
	} {
		target, err := ParseTarget(test.input)
		if err != nil || target != test.want {
			t.Fatalf("%s: target=%+v error=%v", test.input, target, err)
		}
	}
	for _, value := range []string{"0", "-1", "https://example.com/owner/repo/pull/42", "https://github.com/owner/repo/issues/42", "https://github.com/owner/repo/pull/42#issuecomment-123", "https://github.com/owner/repo/pull/42#discussion_r0"} {
		if _, err := ParseTarget(value); err == nil {
			t.Fatalf("expected invalid target: %s", value)
		}
	}
}

func TestValidateRepository(t *testing.T) {
	for _, value := range []string{"owner/repository", "https://github.com/owner/repository"} {
		if err := ValidateRepository(value); err != nil {
			t.Fatal(err)
		}
	}
	for _, value := range []string{"owner", "/repository", "owner/repository\n", "https://example.com/owner/repository"} {
		if err := ValidateRepository(value); err == nil {
			t.Fatalf("expected invalid repository: %s", value)
		}
	}
}
