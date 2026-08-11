package github

import (
	"context"
	"reflect"
	"testing"
)

func TestListReviewComments(t *testing.T) {
	var gotName string
	var gotArgs []string
	runner := func(_ context.Context, name string, args ...string) ([]byte, error) {
		gotName = name
		gotArgs = args
		return []byte(`[[{"id":1,"path":"main.go","line":12,"side":"RIGHT"}]]`), nil
	}

	client := NewClientWithRunner(runner)
	comments, err := client.ListReviewComments(context.Background(), "owner/repository", 42)
	if err != nil {
		t.Fatalf("ListReviewComments() error = %v", err)
	}
	if gotName != "gh" {
		t.Fatalf("command = %q, want %q", gotName, "gh")
	}
	wantArgs := []string{
		"api", "--paginate", "--slurp", "-H", "Accept: application/vnd.github+json",
		"repos/owner/repository/pulls/42/comments?per_page=100",
	}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("arguments = %#v, want %#v", gotArgs, wantArgs)
	}
	if len(comments) != 1 || comments[0].Path != "main.go" {
		t.Fatalf("comments = %#v", comments)
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
		t.Fatalf("repository = %q, want %q", repository, "owner/repository")
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
