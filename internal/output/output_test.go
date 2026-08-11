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
	want := "main.go:12-13\nfirst\nsecond\nreview: 改善してください\n"
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
