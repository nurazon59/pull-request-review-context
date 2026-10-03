package github

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var reviewCommentAnchor = regexp.MustCompile(`^(?:discussion_r|discussion-|discussion-diff-)([0-9]+)$`)

// Target identifies a pull request, optionally narrowed to one inline review comment.
type Target struct {
	Repository    string
	PullRequest   int
	CommentID     int64
	CommentAPIURL bool
}

func ParseTarget(value string) (Target, error) {
	if number, err := strconv.Atoi(value); err == nil {
		if number < 1 {
			return Target{}, fmt.Errorf("pull request number must be greater than zero")
		}
		return Target{PullRequest: number}, nil
	}
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
		return Target{}, fmt.Errorf("target must be a pull request number or GitHub pull request/comment URL: %q", value)
	}
	if parsed.User != nil || (!strings.EqualFold(parsed.Host, "github.com") && !strings.EqualFold(parsed.Host, "api.github.com")) {
		return Target{}, fmt.Errorf("target URL must use github.com or api.github.com: %q", value)
	}
	parts := splitPath(parsed.Path)
	apiParts := stripAPIPrefix(parts)
	if len(apiParts) == 5 && apiParts[0] == "repos" && apiParts[3] == "pulls" {
		repository := apiParts[1] + "/" + apiParts[2]
		if err := ValidateRepository(repository); err != nil {
			return Target{}, err
		}
		pullRequest, err := strconv.Atoi(apiParts[4])
		if err != nil || pullRequest < 1 {
			return Target{}, fmt.Errorf("URL contains an invalid pull request number: %q", value)
		}
		return Target{Repository: repository, PullRequest: pullRequest}, nil
	}
	if repository, commentID, ok := parseReviewCommentAPIPath(apiParts); ok {
		return Target{Repository: repository, CommentID: commentID, CommentAPIURL: true}, nil
	}
	if len(parts) < 4 || (parts[2] != "pull" && parts[2] != "pulls") {
		return Target{}, fmt.Errorf("URL must identify a pull request or inline review comment: %q", value)
	}
	repository := parts[0] + "/" + parts[1]
	if err := ValidateRepository(repository); err != nil {
		return Target{}, err
	}
	pullRequest, err := strconv.Atoi(parts[3])
	if err != nil || pullRequest < 1 {
		return Target{}, fmt.Errorf("URL contains an invalid pull request number: %q", value)
	}
	result := Target{Repository: repository, PullRequest: pullRequest}
	if parsed.Fragment == "" {
		return result, nil
	}
	anchor := reviewCommentAnchor.FindStringSubmatch(parsed.Fragment)
	if anchor == nil {
		return Target{}, fmt.Errorf("URL fragment must identify a review comment (for example #discussion_r123): %q", value)
	}
	commentID, err := strconv.ParseInt(anchor[1], 10, 64)
	if err != nil || commentID < 1 {
		return Target{}, fmt.Errorf("URL contains an invalid review comment ID: %q", value)
	}
	result.CommentID = commentID
	return result, nil
}

func parseReviewCommentAPIPath(parts []string) (string, int64, bool) {
	if len(parts) != 6 || parts[0] != "repos" || parts[3] != "pulls" || parts[4] != "comments" {
		return "", 0, false
	}
	commentID, err := strconv.ParseInt(parts[5], 10, 64)
	if err != nil || commentID < 1 {
		return "", 0, false
	}
	repository := parts[1] + "/" + parts[2]
	if ValidateRepository(repository) != nil {
		return "", 0, false
	}
	return repository, commentID, true
}

func stripAPIPrefix(parts []string) []string {
	if len(parts) > 0 && parts[0] == "api" {
		parts = parts[1:]
		if len(parts) > 0 && strings.HasPrefix(parts[0], "v") {
			parts = parts[1:]
		}
	}
	return parts
}

func splitPath(value string) []string {
	parts := strings.Split(strings.Trim(value, "/"), "/")
	if len(parts) == 1 && parts[0] == "" {
		return nil
	}
	return parts
}
