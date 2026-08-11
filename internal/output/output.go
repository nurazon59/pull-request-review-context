package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/nurazon59/pull-request-review-context/internal/diff"
)

type Item struct {
	Path    string
	Reviews []string
	Target  diff.Target
}

type jsonItem struct {
	Path      string   `json:"path"`
	StartLine int      `json:"start_line"`
	EndLine   int      `json:"end_line"`
	Side      string   `json:"side"`
	Code      []string `json:"code"`
	Reviews   []string `json:"reviews"`
}

func LLM(writer io.Writer, items []Item) error {
	for i, item := range items {
		if i > 0 {
			if _, err := fmt.Fprintln(writer); err != nil {
				return err
			}
		}
		line := fmt.Sprintf("%s:%d", item.Path, item.Target.StartLine)
		if item.Target.StartLine != item.Target.EndLine {
			line = fmt.Sprintf("%s-%d", line, item.Target.EndLine)
		}
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return err
		}
		for _, codeLine := range item.Target.Lines {
			if _, err := fmt.Fprintln(writer, codeLine); err != nil {
				return err
			}
		}
		for _, review := range item.Reviews {
			if strings.TrimSpace(review) == "" {
				continue
			}
			if _, err := fmt.Fprintln(writer, "review: "+review); err != nil {
				return err
			}
		}
	}
	return nil
}

func JSON(writer io.Writer, items []Item) error {
	result := make([]jsonItem, 0, len(items))
	for _, item := range items {
		result = append(result, jsonItem{
			Path:      item.Path,
			StartLine: item.Target.StartLine,
			EndLine:   item.Target.EndLine,
			Side:      item.Target.Side,
			Code:      item.Target.Lines,
			Reviews:   item.Reviews,
		})
	}
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(result)
}
