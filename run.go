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
	Target     string           `arg:"" optional:"" name:"pull-request-or-comment-url" help:"Pull request number, pull request URL, or inline review comment URL. Defaults to the current branch's open pull request."`
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

	repository, pullRequest, selectedCommentID, err := resolveTarget(ctx, client, options)
	if err != nil {
		return err
	}
	threads, err := client.ListReviewThreads(ctx, repository, pullRequest)
	if err != nil {
		return err
	}
	items := make([]output.Item, 0)
	itemIndexes := make(map[string]int)
	selectedCommentFound := selectedCommentID == 0
	for _, thread := range threads {
		for _, comment := range thread.Comments.Nodes {
			commentID, err := comment.NumericID()
			if err != nil {
				return fmt.Errorf("read review comment ID: %w", err)
			}
			if selectedCommentID != 0 && commentID != selectedCommentID {
				continue
			}
			selectedCommentFound = true
			if !options.All && (thread.IsResolved || thread.IsOutdated) {
				continue
			}
			commentTarget, ok := commentTargetFor(thread, comment)
			if !ok {
				continue
			}
			target, err := diff.ExtractTargetLines(commentTarget)
			if errors.Is(err, diff.ErrNoTarget) {
				continue
			}
			if err != nil {
				return fmt.Errorf("extract review comment %d: %w", commentID, err)
			}
			path := comment.Path
			if path == "" {
				path = thread.Path
			}
			key := strings.Join([]string{
				path,
				fmt.Sprint(target.StartLine),
				fmt.Sprint(target.EndLine),
				target.Side,
				strings.Join(target.Lines, "\x00"),
			}, "\x00")
			metadata := output.ReviewComment{
				ID:       commentID,
				ThreadID: thread.ID,
				URL:      comment.URL,
				Path:     path,
				Line:     target.StartLine,
				Body:     comment.Body,
				Resolved: thread.IsResolved,
				Outdated: thread.IsOutdated,
			}
			if index, ok := itemIndexes[key]; ok {
				items[index].Reviews = append(items[index].Reviews, comment.Body)
				items[index].Comments = append(items[index].Comments, metadata)
				continue
			}
			itemIndexes[key] = len(items)
			items = append(items, output.Item{
				Path:     path,
				Reviews:  []string{comment.Body},
				Comments: []output.ReviewComment{metadata},
				Target:   target,
			})
		}
	}
	if !selectedCommentFound {
		return fmt.Errorf("review comment %d was not found in %s pull request #%d", selectedCommentID, repository, pullRequest)
	}

	if options.Format == "json" {
		return output.JSON(writer, items)
	}
	return output.LLM(writer, items)
}

func resolveTarget(ctx context.Context, client gh.Client, options Options) (string, int, int64, error) {
	if options.Target != "" && options.PullRequest != 0 {
		return "", 0, 0, errors.New("specify either a target or pull request number, not both")
	}
	if options.Target == "" && options.PullRequest == 0 {
		pullRequest, err := client.CurrentPullRequest(ctx, options.Repository)
		if err != nil {
			return "", 0, 0, err
		}
		return pullRequest.Repository, pullRequest.Number, 0, nil
	}

	selector := gh.Target{PullRequest: options.PullRequest}
	if options.Target != "" {
		var err error
		selector, err = gh.ParseTarget(options.Target)
		if err != nil {
			return "", 0, 0, err
		}
	}
	repository := selector.Repository
	if repository == "" {
		repository = options.Repository
	}
	if repository == "" {
		var err error
		repository, err = client.CurrentRepository(ctx)
		if err != nil {
			return "", 0, 0, fmt.Errorf("resolve current repository: %w", err)
		}
	}
	normalizedRepository, err := gh.NormalizeRepository(repository)
	if err != nil {
		return "", 0, 0, err
	}
	if selector.CommentAPIURL {
		selector.PullRequest, err = client.PullRequestForComment(ctx, normalizedRepository, selector.CommentID)
		if err != nil {
			return "", 0, 0, err
		}
	}
	if selector.PullRequest < 1 {
		return "", 0, 0, errors.New("pull request number must be greater than zero")
	}
	return normalizedRepository, selector.PullRequest, selector.CommentID, nil
}

func commentTargetFor(thread gh.ReviewThread, comment gh.ReviewComment) (diff.Comment, bool) {
	line := firstInt(comment.Line, thread.Line, comment.OriginalLine, thread.OriginalLine)
	startLine := firstInt(comment.StartLine, thread.StartLine, comment.OriginalStartLine, thread.OriginalStartLine)
	if thread.IsOutdated {
		line = firstInt(comment.OriginalLine, thread.OriginalLine, comment.Line, thread.Line)
		startLine = firstInt(comment.OriginalStartLine, thread.OriginalStartLine, comment.StartLine, thread.StartLine)
	}
	if line == nil {
		return diff.Comment{}, false
	}
	side := thread.DiffSide
	if side == "" {
		return diff.Comment{}, false
	}
	startSide := firstString(thread.StartDiffSide, side)
	hunk := comment.DiffHunk
	if hunk == "" {
		for _, candidate := range thread.Comments.Nodes {
			if candidate.DiffHunk != "" {
				hunk = candidate.DiffHunk
				break
			}
		}
	}
	return diff.Comment{
		DiffHunk:  hunk,
		StartLine: startLine,
		Line:      line,
		StartSide: startSide,
		Side:      side,
	}, true
}

func firstInt(values ...*int) *int {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func firstString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
