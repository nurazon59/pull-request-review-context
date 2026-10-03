package pullrequestreviewcontext

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	gh "github.com/nurazon59/pull-request-review-context/internal/github"
	"github.com/nurazon59/pull-request-review-context/internal/output"
)

func fixtureClient(t *testing.T) gh.Client {
	t.Helper()
	return gh.NewClientWithRunner(func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "gh" {
			t.Fatalf("unexpected command %s", name)
		}
		switch args[0] {
		case "pr":
			return []byte("https://github.com/upstream/project/pull/42\n"), nil
		case "repo":
			return []byte("upstream/project\n"), nil
		case "api":
			if args[1] == "graphql" {
				return []byte(threadStates), nil
			}
			if args[1] == "--jq" {
				return []byte("https://api.github.com/repos/upstream/project/pulls/42\n"), nil
			}
			return []byte(reviewComments), nil
		}
		t.Fatalf("unexpected command args %v", args)
		return nil, nil
	})
}

func TestExecuteTargetsAndStates(t *testing.T) {
	for _, test := range []struct {
		name    string
		options Options
		ids     []int64
	}{
		{"current branch", Options{}, []int64{101, 104}},
		{"PR number", Options{Target: "42"}, []int64{101, 104}},
		{"PR URL", Options{Target: "https://github.com/upstream/project/pull/42"}, []int64{101, 104}},
		{"all", Options{Target: "42", All: true}, []int64{101, 104, 102, 103}},
		{"resolved comment", Options{Target: "https://github.com/upstream/project/pull/42#discussion_r102"}, []int64{102}},
		{"API outdated comment", Options{Target: "https://api.github.com/repos/upstream/project/pulls/comments/103"}, []int64{103}},
		{"reply", Options{Target: "https://github.com/upstream/project/pull/42/files#discussion_r104"}, []int64{104}},
		{"legacy PR option", Options{Repository: "upstream/project", PullRequest: 42}, []int64{101, 104}},
	} {
		t.Run(test.name, func(t *testing.T) {
			test.options.Format = "json"
			var buffer bytes.Buffer
			if err := Execute(context.Background(), &buffer, fixtureClient(t), test.options); err != nil {
				t.Fatal(err)
			}
			var items []struct {
				Code     []string
				Comments []output.ReviewComment
			}
			if err := json.Unmarshal(buffer.Bytes(), &items); err != nil {
				t.Fatal(err)
			}
			var ids []int64
			for _, item := range items {
				// A non-outdated comment's line moved from 12 to 13. Its diff hunk is original.
				if !reflect.DeepEqual(item.Code, []string{"target"}) {
					t.Fatalf("wrong code: %v", item.Code)
				}
				for _, comment := range item.Comments {
					ids = append(ids, comment.ID)
					if comment.Path != "main.go" || comment.Line != 12 || comment.URL == "" || comment.ThreadID == "" || comment.Resolved != (comment.ID == 102) || comment.Outdated != (comment.ID == 103) {
						t.Fatalf("metadata: %+v", comment)
					}
				}
			}
			if !reflect.DeepEqual(ids, test.ids) {
				t.Fatalf("IDs = %v, want %v", ids, test.ids)
			}
		})
	}
}

func TestExecuteLabeledOutput(t *testing.T) {
	var buffer bytes.Buffer
	if err := Execute(context.Background(), &buffer, fixtureClient(t), Options{}); err != nil {
		t.Fatal(err)
	}
	want := "file: main.go\nline: 12\n\ncode:\ntarget\n\nreview:\nfirst\n\nreview:\nreply\n"
	if buffer.String() != want {
		t.Fatalf("output = %q, want %q", buffer.String(), want)
	}
}

func TestExecuteFailuresLeaveOutputEmpty(t *testing.T) {
	for _, test := range []struct {
		name    string
		options Options
		apiFail bool
	}{
		{"missing comment", Options{Target: "https://github.com/upstream/project/pull/42#discussion_r999"}, false},
		{"API failure", Options{Target: "42", Repository: "upstream/project"}, true},
		{"ambiguous arguments", Options{Target: "42", PullRequest: 43}, false},
		{"unknown format", Options{Format: "yaml"}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := fixtureClient(t)
			if test.apiFail {
				client = gh.NewClientWithRunner(func(_ context.Context, _ string, args ...string) ([]byte, error) {
					if args[1] == "graphql" {
						return nil, errors.New("GitHub API unavailable after first page")
					}
					return []byte(reviewComments), nil
				})
			}
			var buffer bytes.Buffer
			if err := Execute(context.Background(), &buffer, client, test.options); err == nil {
				t.Fatal("expected error")
			}
			if buffer.Len() != 0 {
				t.Fatalf("partial output: %s", buffer.String())
			}
		})
	}
}

func TestExecuteEmptyResult(t *testing.T) {
	client := gh.NewClientWithRunner(func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if args[1] == "graphql" {
			return []byte(threadStates), nil
		}
		return []byte(`[[{"id":102}]]`), nil
	})
	for _, format := range []string{"llm", "json"} {
		var buffer bytes.Buffer
		if err := Execute(context.Background(), &buffer, client, Options{Target: "42", Repository: "upstream/project", Format: format}); err != nil {
			t.Fatal(err)
		}
		want := ""
		if format == "json" {
			want = "[]\n"
		}
		if buffer.String() != want {
			t.Fatalf("%s output = %q", format, buffer.String())
		}
	}
}

const reviewComments = `[[
 {"id":101,"html_url":"https://github.com/upstream/project/pull/42#discussion_r101","path":"main.go","body":"first","line":13,"original_line":12,"side":"RIGHT","diff_hunk":"@@ -12,1 +12,2 @@\n-old\n+target\n+unrelated"},
 {"id":104,"html_url":"https://github.com/upstream/project/pull/42#discussion_r104","path":"main.go","body":"reply","line":13,"original_line":12,"side":"RIGHT","diff_hunk":"@@ -12,1 +12,2 @@\n-old\n+target\n+unrelated"},
 {"id":102,"html_url":"https://github.com/upstream/project/pull/42#discussion_r102","path":"main.go","body":"resolved","line":12,"side":"RIGHT","diff_hunk":"@@ -12,1 +12,1 @@\n-old\n+target"},
 {"id":103,"html_url":"https://github.com/upstream/project/pull/42#discussion_r103","path":"main.go","body":"outdated","original_line":12,"side":"RIGHT","diff_hunk":"@@ -12,1 +12,1 @@\n-old\n+target"}
]]`
const threadStates = `[{"data":{"repository":{"pullRequest":{"reviewThreads":{"nodes":[
 {"id":"UNRESOLVED","isResolved":false,"isOutdated":false,"comments":{"totalCount":2,"nodes":[{"fullDatabaseId":"101"},{"fullDatabaseId":"104"}]}},
 {"id":"RESOLVED","isResolved":true,"isOutdated":false,"comments":{"totalCount":1,"nodes":[{"fullDatabaseId":"102"}]}},
 {"id":"OUTDATED","isResolved":false,"isOutdated":true,"comments":{"totalCount":1,"nodes":[{"fullDatabaseId":"103"}]}}
]}}}}}]`
