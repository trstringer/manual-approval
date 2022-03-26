package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/go-github/v43/github"
)

type approvalEnvironment struct {
	client              *github.Client
	repoFullName        string
	repo                string
	repoOwner           string
	runID               int
	approvers           []string
	approvalIssue       *github.Issue
	approvalIssueNumber int
}

func newApprovalEnvironment(client *github.Client, repoFullName, repoOwner string, runID int, approvers []string) (*approvalEnvironment, error) {
	repoOwnerAndName := strings.Split(repoFullName, "/")
	if len(repoOwnerAndName) != 2 {
		return nil, fmt.Errorf("repo owner and name in unexpected format: %s", repoFullName)
	}
	repo := repoOwnerAndName[1]

	return &approvalEnvironment{
		client:       client,
		repoFullName: repoFullName,
		repo:         repo,
		repoOwner:    repoOwner,
		runID:        runID,
		approvers:    approvers,
	}, nil
}

func (a approvalEnvironment) runURL() string {
	return fmt.Sprintf("https://github.com/%s/actions/runs/%d", a.repoFullName, a.runID)
}

func (a *approvalEnvironment) createApprovalIssue(ctx context.Context) error {
	issueTitle := fmt.Sprintf("Manual approval required for workflow run %d", a.runID)
	issueBody := fmt.Sprintf(`Workflow is pending manual review.
URL: %s

Required approvers: %s

Respond %s to continue workflow or %s to cancel.`,
		a.runURL(),
		a.approvers,
		formatAcceptedWords(approvedWords),
		formatAcceptedWords(deniedWords),
	)
	var err error
	a.approvalIssue, _, err = a.client.Issues.Create(ctx, a.repoOwner, a.repo, &github.IssueRequest{
		Title:     &issueTitle,
		Body:      &issueBody,
		Assignees: &a.approvers,
	})
	a.approvalIssueNumber = a.approvalIssue.GetNumber()
	return err
}

func approvalFromComments(comments []*github.IssueComment, approvers []string) (approvalStatus, error) {
	remainingApprovers := make([]string, len(approvers))
	copy(remainingApprovers, approvers)

	for _, comment := range comments {
		commentUser := comment.User.GetLogin()
		approverIdx := approversIndex(remainingApprovers, commentUser)
		if approverIdx < 0 {
			continue
		}

		commentBody := comment.GetBody()
		isApprovalComment, err := isApproved(commentBody)
		if err != nil {
			return approvalStatusPending, err
		}
		if isApprovalComment {
			if len(remainingApprovers) == 1 {
				return approvalStatusApproved, nil
			}
			remainingApprovers[approverIdx] = remainingApprovers[len(remainingApprovers)-1]
			remainingApprovers = remainingApprovers[:len(remainingApprovers)-1]
			continue
		}

		isDenialComment, err := isDenied(commentBody)
		if err != nil {
			return approvalStatusPending, err
		}
		if isDenialComment {
			return approvalStatusDenied, nil
		}
	}

	return approvalStatusPending, nil
}

func approversIndex(approvers []string, name string) int {
	for idx, approver := range approvers {
		if approver == name {
			return idx
		}
	}
	return -1
}

func isApproved(commentBody string) (bool, error) {
	for _, approvedWord := range approvedWords {
		matched, err := regexp.MatchString(fmt.Sprintf("(?i)^%s[.!]?$", approvedWord), commentBody)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}

	return false, nil
}

func isDenied(commentBody string) (bool, error) {
	for _, deniedWord := range deniedWords {
		matched, err := regexp.MatchString(fmt.Sprintf("(?i)^%s[.!]?$", deniedWord), commentBody)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}

	return false, nil
}

func formatAcceptedWords(words []string) string {
	var quotedWords []string

	for _, word := range words {
		quotedWords = append(quotedWords, fmt.Sprintf("\"%s\"", word))
	}

	return strings.Join(quotedWords, ", ")
}
