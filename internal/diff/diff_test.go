package diff

import "testing"

func TestExtractTargetLines(t *testing.T) {
	tests := map[string]struct {
		comment Comment
		want    Target
		wantErr bool
	}{
		"right single line": {
			comment: Comment{
				DiffHunk: "@@ -10,3 +10,4 @@\n context\n-old\n+new\n+another",
				Line:     intPointer(12),
				Side:     "RIGHT",
			},
			want: Target{
				StartLine: 12,
				EndLine:   12,
				Side:      "RIGHT",
				Lines:     []string{"another"},
			},
		},
		"right multiple lines": {
			comment: Comment{
				DiffHunk:  "@@ -10,3 +10,4 @@\n context\n-old\n+new\n+another",
				StartLine: intPointer(11),
				Line:      intPointer(12),
				StartSide: "RIGHT",
				Side:      "RIGHT",
			},
			want: Target{
				StartLine: 11,
				EndLine:   12,
				Side:      "RIGHT",
				Lines:     []string{"new", "another"},
			},
		},
		"left deletion": {
			comment: Comment{
				DiffHunk: "@@ -10,3 +10,2 @@\n context\n-old\n+new",
				Line:     intPointer(11),
				Side:     "LEFT",
			},
			want: Target{
				StartLine: 11,
				EndLine:   11,
				Side:      "LEFT",
				Lines:     []string{"old"},
			},
		},
		"file comment has no target line": {
			comment: Comment{
				DiffHunk: "@@ -1 +1 @@\n-a\n+b",
				Side:     "RIGHT",
			},
			wantErr: true,
		},
		"target line is outside hunk": {
			comment: Comment{
				DiffHunk: "@@ -1 +1 @@\n-a\n+b",
				Line:     intPointer(9),
				Side:     "RIGHT",
			},
			wantErr: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := ExtractTargetLines(test.comment)
			if test.wantErr {
				if err == nil {
					t.Fatal("ExtractTargetLines() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ExtractTargetLines() error = %v", err)
			}
			if got.StartLine != test.want.StartLine || got.EndLine != test.want.EndLine || got.Side != test.want.Side {
				t.Fatalf("ExtractTargetLines() target = %#v, want %#v", got, test.want)
			}
			if len(got.Lines) != len(test.want.Lines) {
				t.Fatalf("ExtractTargetLines() lines = %#v, want %#v", got.Lines, test.want.Lines)
			}
			for i := range got.Lines {
				if got.Lines[i] != test.want.Lines[i] {
					t.Fatalf("ExtractTargetLines() lines = %#v, want %#v", got.Lines, test.want.Lines)
				}
			}
		})
	}
}
