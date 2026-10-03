package github

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var reviewCommentAnchor = regexp.MustCompile(`^(?:discussion_r|discussion-|discussion-diff-)([0-9]+)$`)

type Target struct {
	Repository  string
	PullRequest int
	CommentID   int64
}

func ParseTarget(value string) (Target, error) {
	if number, err := strconv.Atoi(value); err == nil {
		if number < 1 {
			return Target{}, fmt.Errorf("pull request number must be greater than zero")
		}
		return Target{PullRequest: number}, nil
	}
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || (u.Host != "github.com" && u.Host != "api.github.com") {
		return Target{}, fmt.Errorf("target must be a PR number or github.com/api.github.com PR or review comment URL: %q", value)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if u.Host == "api.github.com" && len(parts) > 0 && parts[0] == "repos" {
		parts = parts[1:]
	}
	if len(parts) < 4 || (parts[2] != "pull" && parts[2] != "pulls") {
		return Target{}, fmt.Errorf("URL must identify a pull request or inline review comment: %q", value)
	}
	target := Target{Repository: parts[0] + "/" + parts[1]}
	if err := ValidateRepository(target.Repository); err != nil {
		return Target{}, err
	}
	if u.Host == "api.github.com" && len(parts) == 5 && parts[3] == "comments" {
		target.CommentID, err = strconv.ParseInt(parts[4], 10, 64)
		if err != nil || target.CommentID < 1 {
			return Target{}, fmt.Errorf("invalid review comment ID in %q", value)
		}
		return target, nil
	}
	target.PullRequest, err = strconv.Atoi(parts[3])
	if err != nil || target.PullRequest < 1 {
		return Target{}, fmt.Errorf("invalid pull request number in %q", value)
	}
	if u.Fragment != "" {
		anchor := reviewCommentAnchor.FindStringSubmatch(u.Fragment)
		if anchor == nil {
			return Target{}, fmt.Errorf("URL fragment must identify a review comment: %q", value)
		}
		target.CommentID, err = strconv.ParseInt(anchor[1], 10, 64)
		if err != nil || target.CommentID < 1 {
			return Target{}, fmt.Errorf("invalid review comment ID in %q", value)
		}
	}
	return target, nil
}
