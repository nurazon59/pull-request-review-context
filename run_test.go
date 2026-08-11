package pullrequestreviewcontext

import (
	"bytes"
	"context"
	"testing"

	gh "github.com/nurazon59/pull-request-review-context/internal/github"
)

func TestExecute(t *testing.T) {
	runner := func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return []byte(`[[{"id":1,"path":"main.go","diff_hunk":"@@ -11,1 +11,1 @@\n-old\n+new","line":11,"side":"RIGHT","body":"修正してください"},{"id":2,"path":"main.go","diff_hunk":"@@ -11,1 +11,1 @@\n-old\n+new","line":11,"side":"RIGHT","body":"別の指摘です"},{"id":3,"path":"main.go","body":"ファイル全体の指摘"}]]`), nil
	}
	var buffer bytes.Buffer

	err := Execute(context.Background(), &buffer, gh.NewClientWithRunner(runner), Options{
		Repository:  "owner/repository",
		Format:      "llm",
		PullRequest: 42,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := "main.go:11\nnew\nreview: 修正してください\nreview: 別の指摘です\n"
	if buffer.String() != want {
		t.Fatalf("Execute() = %q, want %q", buffer.String(), want)
	}
}
