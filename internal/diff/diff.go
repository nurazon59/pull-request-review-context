package diff

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var ErrNoTarget = errors.New("review comment has no target code line")

type Comment struct {
	DiffHunk  string
	StartLine *int
	Line      *int
	StartSide string
	Side      string
}

type Target struct {
	StartLine int
	EndLine   int
	Side      string
	Lines     []string
}

type hunkLine struct {
	oldLine *int
	newLine *int
	content string
}

var hunkHeaderPattern = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

func ExtractTargetLines(comment Comment) (Target, error) {
	if comment.Line == nil || comment.Side == "" {
		return Target{}, ErrNoTarget
	}

	startLine := *comment.Line
	startSide := comment.Side
	if comment.StartLine != nil {
		startLine = *comment.StartLine
		if comment.StartSide != "" {
			startSide = comment.StartSide
		}
	}
	if startLine > *comment.Line || startSide != comment.Side {
		return Target{}, fmt.Errorf("%w: mixed or invalid target range", ErrNoTarget)
	}

	lines, err := parseHunk(comment.DiffHunk)
	if err != nil {
		return Target{}, fmt.Errorf("%w: %v", ErrNoTarget, err)
	}

	target := Target{
		StartLine: startLine,
		EndLine:   *comment.Line,
		Side:      comment.Side,
	}
	for _, line := range lines {
		lineNumber := line.newLine
		if comment.Side == "LEFT" {
			lineNumber = line.oldLine
		}
		if lineNumber == nil || *lineNumber < target.StartLine || *lineNumber > target.EndLine {
			continue
		}
		target.Lines = append(target.Lines, line.content)
	}
	if len(target.Lines) == 0 {
		return Target{}, fmt.Errorf("%w: target line is not in diff hunk", ErrNoTarget)
	}
	return target, nil
}

func parseHunk(hunk string) ([]hunkLine, error) {
	parts := strings.Split(strings.ReplaceAll(hunk, "\r\n", "\n"), "\n")
	if len(parts) == 0 {
		return nil, errors.New("empty diff hunk")
	}
	header := hunkHeaderPattern.FindStringSubmatch(parts[0])
	if header == nil {
		return nil, errors.New("invalid diff hunk header")
	}
	oldLine, err := strconv.Atoi(header[1])
	if err != nil {
		return nil, err
	}
	newLine, err := strconv.Atoi(header[3])
	if err != nil {
		return nil, err
	}

	result := make([]hunkLine, 0, len(parts)-1)
	for _, part := range parts[1:] {
		if part == "" || strings.HasPrefix(part, "\\ No newline") {
			continue
		}
		line := hunkLine{}
		switch part[0] {
		case ' ':
			line.oldLine = intPointer(oldLine)
			line.newLine = intPointer(newLine)
			oldLine++
			newLine++
		case '-':
			line.oldLine = intPointer(oldLine)
			oldLine++
		case '+':
			line.newLine = intPointer(newLine)
			newLine++
		default:
			return nil, fmt.Errorf("invalid diff line prefix %q", part[:1])
		}
		line.content = part[1:]
		result = append(result, line)
	}
	return result, nil
}

func intPointer(value int) *int {
	return &value
}
