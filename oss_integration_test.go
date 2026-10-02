//go:build integration

package pullrequestreviewcontext

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	gh "github.com/nurazon59/pull-request-review-context/internal/github"
)

func TestKubernetesPullRequestReviewComments(t *testing.T) {
	threads, err := gh.NewClient().ListReviewThreads(context.Background(), "kubernetes/kubernetes", 138132)
	if err != nil {
		t.Fatalf("ListReviewThreads() error = %v", err)
	}
	commentCount := 0
	lineComments := 0
	for _, thread := range threads {
		for _, comment := range thread.Comments.Nodes {
			commentCount++
			if comment.Line != nil || comment.OriginalLine != nil {
				lineComments++
			}
		}
	}
	if commentCount < 40 {
		t.Fatalf("review comments = %d, want at least 40", commentCount)
	}
	if lineComments < 10 {
		t.Fatalf("line-targeted review comments = %d, want at least 10", lineComments)
	}

	var output bytes.Buffer
	err = Execute(context.Background(), &output, gh.NewClient(), Options{
		Repository:  "kubernetes/kubernetes",
		Format:      "json",
		PullRequest: 138132,
		All:         true,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var items []struct {
		Code []string `json:"code"`
	}
	if err := json.Unmarshal(output.Bytes(), &items); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(items) < 10 {
		t.Fatalf("extracted items = %d, want at least 10", len(items))
	}
	for _, item := range items {
		if len(item.Code) == 0 {
			t.Fatal("extracted item has no code lines")
		}
	}
}
