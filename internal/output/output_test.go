package output

import (
	"bytes"
	"testing"

	"github.com/nurazon59/pull-request-review-context/internal/diff"
)

func TestLLM(t *testing.T) {
	var buffer bytes.Buffer
	items := []Item{{
		Path:    "main.go",
		Reviews: []string{"改善してください"},
		Target: diff.Target{
			StartLine: 12,
			EndLine:   13,
			Lines:     []string{"first", "second"},
		},
	}}

	if err := LLM(&buffer, items); err != nil {
		t.Fatalf("LLM() error = %v", err)
	}
	want := "file: main.go\nline: 12-13\n\ncode:\nfirst\nsecond\n\nreview:\n改善してください\n"
	if buffer.String() != want {
		t.Fatalf("LLM() = %q, want %q", buffer.String(), want)
	}
}

func TestJSON(t *testing.T) {
	var buffer bytes.Buffer
	items := []Item{{
		Path:    "main.go",
		Reviews: []string{"改善してください"},
		Target: diff.Target{
			StartLine: 12,
			EndLine:   12,
			Side:      "RIGHT",
			Lines:     []string{"first"},
		},
	}}

	if err := JSON(&buffer, items); err != nil {
		t.Fatalf("JSON() error = %v", err)
	}
	want := `[{"path":"main.go","start_line":12,"end_line":12,"side":"RIGHT","code":["first"],"reviews":["改善してください"]}]` + "\n"
	if buffer.String() != want {
		t.Fatalf("JSON() = %q, want %q", buffer.String(), want)
	}
}

func TestLLMIncludesResolvedAndOutdatedLabels(t *testing.T) {
	var buffer bytes.Buffer
	items := []Item{{
		Path:    "main.go",
		Reviews: []string{"修正済みの指摘"},
		Comments: []ReviewComment{{
			ID:       123,
			ThreadID: "THREAD_1",
			Resolved: true,
			Outdated: true,
		}},
		Target: diff.Target{StartLine: 12, EndLine: 12, Lines: []string{"line"}},
	}}
	if err := LLM(&buffer, items); err != nil {
		t.Fatalf("LLM() error = %v", err)
	}
	want := "file: main.go\nline: 12\n\ncode:\nline\n\nreview status: resolved, outdated\nreview:\n修正済みの指摘\n"
	if buffer.String() != want {
		t.Fatalf("LLM() = %q, want %q", buffer.String(), want)
	}
}

func TestJSONIncludesCommentMetadata(t *testing.T) {
	var buffer bytes.Buffer
	items := []Item{{
		Path:    "main.go",
		Reviews: []string{"review"},
		Comments: []ReviewComment{{
			ID:       123,
			ThreadID: "THREAD_1",
			URL:      "https://github.com/owner/repo/pull/1#discussion_r123",
			Path:     "main.go",
			Line:     12,
			Body:     "review",
			Resolved: true,
			Outdated: true,
		}},
		Target: diff.Target{StartLine: 12, EndLine: 12, Side: "RIGHT", Lines: []string{"line"}},
	}}
	if err := JSON(&buffer, items); err != nil {
		t.Fatalf("JSON() error = %v", err)
	}
	want := `[{"path":"main.go","start_line":12,"end_line":12,"side":"RIGHT","code":["line"],"reviews":["review"],"comments":[{"id":123,"thread_id":"THREAD_1","url":"https://github.com/owner/repo/pull/1#discussion_r123","path":"main.go","line":12,"body":"review","resolved":true,"outdated":true}]}]` + "\n"
	if buffer.String() != want {
		t.Fatalf("JSON() = %q, want %q", buffer.String(), want)
	}
}
