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
	Target      string
	All         bool
}

type cliOptions struct {
	Repository string           `name:"repository" short:"R" help:"GitHub repository in OWNER/REPOSITORY format." env:"GITHUB_REPOSITORY"`
	Format     string           `name:"format" help:"Output format." enum:"llm,json" default:"llm"`
	Target     string           `arg:"" optional:"" name:"pull-request-or-comment-url" help:"Pull request number, pull request URL, or inline review comment URL (any status). Defaults to the current branch's open pull request."`
	All        bool             `name:"all" help:"Include resolved and outdated review threads."`
	Version    kong.VersionFlag `name:"version" help:"Print version information and quit."`
}

func Run() error {
	if len(os.Args) > 1 && os.Args[1] == "skills" {
		return runSkills(context.Background(), os.Args[2:], os.Stdout, os.Stderr)
	}

	var cli cliOptions
	kong.Parse(&cli, kong.Name("pull-request-review-context"), kong.Vars{
		"version": appVersion,
	})
	return Execute(context.Background(), os.Stdout, gh.NewClient(), Options{
		Repository: cli.Repository,
		Format:     cli.Format,
		Target:     cli.Target,
		All:        cli.All,
	})
}

func Execute(ctx context.Context, writer io.Writer, client gh.Client, options Options) error {
	if options.Format == "" {
		options.Format = "llm"
	}
	if options.Format != "llm" && options.Format != "json" {
		return fmt.Errorf("unsupported output format %q", options.Format)
	}

	selector, err := resolveTarget(ctx, client, options)
	if err != nil {
		return err
	}
	comments, err := client.ListReviewComments(ctx, selector.Repository, selector.PullRequest)
	if err != nil {
		return err
	}
	items := make([]output.Item, 0)
	itemIndexes := make(map[string]int)
	selectedCommentFound := selector.CommentID == 0
	for _, comment := range comments {
		if selector.CommentID != 0 && comment.ID != selector.CommentID {
			continue
		}
		selectedCommentFound = true
		if selector.CommentID == 0 && !options.All && (comment.Resolved || comment.Outdated) {
			continue
		}
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
		metadata := output.ReviewComment{
			ID:       comment.ID,
			ThreadID: comment.ThreadID,
			URL:      comment.URL,
			Path:     comment.Path,
			Line:     target.StartLine,
			Body:     comment.Body,
			Resolved: comment.Resolved,
			Outdated: comment.Outdated,
		}
		index, ok := itemIndexes[key]
		if !ok {
			index = len(items)
			itemIndexes[key] = index
			items = append(items, output.Item{Path: comment.Path, Target: target})
		}
		items[index].Reviews = append(items[index].Reviews, comment.Body)
		items[index].Comments = append(items[index].Comments, metadata)
	}
	if !selectedCommentFound {
		return fmt.Errorf("review comment %d was not found in %s pull request #%d", selector.CommentID, selector.Repository, selector.PullRequest)
	}

	if options.Format == "json" {
		return output.JSON(writer, items)
	}
	return output.LLM(writer, items)
}

func resolveTarget(ctx context.Context, client gh.Client, options Options) (gh.Target, error) {
	value := options.Target
	if options.PullRequest != 0 {
		if value != "" {
			return gh.Target{}, errors.New("specify either a target or pull request number, not both")
		}
		value = fmt.Sprint(options.PullRequest)
	}
	if value == "" {
		return client.CurrentPullRequest(ctx, options.Repository)
	}
	target, err := gh.ParseTarget(value)
	if err != nil {
		return target, err
	}
	if target.Repository == "" {
		target.Repository = options.Repository
	}
	if target.Repository == "" {
		target.Repository, err = client.CurrentRepository(ctx)
		if err != nil {
			return target, err
		}
	}
	target.Repository, err = gh.NormalizeRepository(target.Repository)
	if err != nil {
		return target, err
	}
	if target.PullRequest == 0 {
		target.PullRequest, err = client.PullRequestForComment(ctx, target.Repository, target.CommentID)
	}
	return target, err
}
