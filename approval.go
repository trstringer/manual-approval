package main

import (
	"context"
	"fmt"
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

Respond '%s' to continue workflow or '%s' to cancel.
	`, a.runURL(), a.approvers, approvalStatusApproved, approvalStatusDenied)
	var err error
	a.approvalIssue, _, err = a.client.Issues.Create(ctx, a.repoOwner, a.repo, &github.IssueRequest{
		Title:     &issueTitle,
		Body:      &issueBody,
		Assignees: &a.approvers,
	})
	a.approvalIssueNumber = a.approvalIssue.GetNumber()
	return err
}

func approvalFromComments(comments []*github.IssueComment, approvers []string) approvalStatus {
	remainingApprovers := make([]string, len(approvers))
	copy(remainingApprovers, approvers)

	for _, comment := range comments {
		commentUser := comment.User.GetLogin()
		approverIdx := approversIndex(remainingApprovers, commentUser)
		if approverIdx < 0 {
			continue
		}

		commentBody := comment.GetBody()
		if commentBody == string(approvalStatusApproved) {
			if len(remainingApprovers) == 1 {
				return approvalStatusApproved
			}
			remainingApprovers[approverIdx] = remainingApprovers[len(remainingApprovers)-1]
			remainingApprovers = remainingApprovers[:len(remainingApprovers)-1]
			continue
		} else if commentBody == string(approvalStatusDenied) {
			return approvalStatusDenied
		}
	}

	return approvalStatusPending
}

func approversIndex(approvers []string, name string) int {
	for idx, approver := range approvers {
		if approver == name {
			return idx
		}
	}
	return -1
}
