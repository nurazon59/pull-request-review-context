package pullrequestreviewcontext

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/nurazon59/pull-request-review-context/internal/diff"
	gh "github.com/nurazon59/pull-request-review-context/internal/github"
	"github.com/nurazon59/pull-request-review-context/internal/output"
)

const appVersion = "v0.2.0"

type Options struct {
	Repository  string
	Format      string
	PullRequest int
}

var CLI struct {
	Repository  string           `name:"repository" short:"R" help:"GitHub repository in OWNER/REPOSITORY format." env:"GITHUB_REPOSITORY"`
	Format      string           `name:"format" help:"Output format." enum:"llm,json" default:"llm"`
	PullRequest int              `arg:"" name:"pull-request" help:"Pull request number."`
	Version     kong.VersionFlag `name:"version" help:"Print version information and quit."`
}

func Run() error {
	if len(os.Args) > 1 && os.Args[1] == "skills" {
		return runSkills(context.Background(), os.Args[2:], os.Stdout, os.Stderr)
	}

	kong.Parse(&CLI, kong.Name("pull-request-review-context"), kong.Vars{
		"version": appVersion,
	})
	return Execute(context.Background(), os.Stdout, gh.NewClient(), Options{
		Repository:  CLI.Repository,
		Format:      CLI.Format,
		PullRequest: CLI.PullRequest,
	})
}

func Execute(ctx context.Context, writer io.Writer, client gh.Client, options Options) error {
	repository := options.Repository
	if repository == "" {
		var err error
		repository, err = client.CurrentRepository(ctx)
		if err != nil {
			return fmt.Errorf("resolve current repository: %w", err)
		}
	}
	if err := gh.ValidateRepository(repository); err != nil {
		return err
	}

	comments, err := client.ListReviewComments(ctx, repository, options.PullRequest)
	if err != nil {
		return err
	}
	items := make([]output.Item, 0, len(comments))
	itemIndexes := make(map[string]int)
	for _, comment := range comments {
		target, err := diff.ExtractTargetLines(diff.Comment{
			DiffHunk:  comment.DiffHunk,
			StartLine: comment.StartLine,
			Line:      comment.Line,
			StartSide: comment.StartSide,
			Side:      comment.Side,
		})
		if errors.Is(err, diff.ErrNoTarget) {
			continue
		}
		if err != nil {
			return fmt.Errorf("extract comment %d: %w", comment.ID, err)
		}
		key := strings.Join([]string{
			comment.Path,
			fmt.Sprint(target.StartLine),
			fmt.Sprint(target.EndLine),
			target.Side,
			strings.Join(target.Lines, "\x00"),
		}, "\x00")
		if index, ok := itemIndexes[key]; ok {
			items[index].Reviews = append(items[index].Reviews, comment.Body)
			continue
		}
		itemIndexes[key] = len(items)
		items = append(items, output.Item{
			Path:    comment.Path,
			Reviews: []string{comment.Body},
			Target:  target,
		})
	}

	if options.Format == "json" {
		return output.JSON(writer, items)
	}
	return output.LLM(writer, items)
}
