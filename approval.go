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
	approvalIssue       *github.Issue
	approvalIssueNumber int
	issueTitle          string
	issueApprovers      []string
	minimumApprovals    int
}

func newApprovalEnvironment(client *github.Client, repoFullName, repoOwner string, runID int, approvers []string, minimumApprovals int, issueTitle string) (*approvalEnvironment, error) {
	repoOwnerAndName := strings.Split(repoFullName, "/")
	if len(repoOwnerAndName) != 2 {
		return nil, fmt.Errorf("repo owner and name in unexpected format: %s", repoFullName)
	}
	repo := repoOwnerAndName[1]

	return &approvalEnvironment{
		client:           client,
		repoFullName:     repoFullName,
		repo:             repo,
		repoOwner:        repoOwner,
		runID:            runID,
		issueApprovers:   approvers,
		minimumApprovals: minimumApprovals,
		issueTitle:       issueTitle,
	}, nil
}

func (a approvalEnvironment) runURL() string {
	return fmt.Sprintf("https://github.com/%s/actions/runs/%d", a.repoFullName, a.runID)
}

func (a *approvalEnvironment) createApprovalIssue(ctx context.Context) error {
	issueTitle := fmt.Sprintf("Manual approval required for workflow run %d", a.runID)

	if a.issueTitle != "" {
		issueTitle = fmt.Sprintf("%s: %s", issueTitle, a.issueTitle)
	}

	issueBody := fmt.Sprintf(`Workflow is pending manual review.
URL: %s

Required approvers: %s

Respond %s to continue workflow or %s to cancel.`,
		a.runURL(),
		a.issueApprovers,
		formatAcceptedWords(approvedWords),
		formatAcceptedWords(deniedWords),
	)
	var err error
	fmt.Printf(
		"Creating issue in repo %s/%s with the following content:\nTitle: %s\nApprovers: %s\nBody:\n%s\n",
		a.repoOwner,
		a.repo,
		issueTitle,
		a.issueApprovers,
		issueBody,
	)
	a.approvalIssue, _, err = a.client.Issues.Create(ctx, a.repoOwner, a.repo, &github.IssueRequest{
		Title:     &issueTitle,
		Body:      &issueBody,
		Assignees: &a.issueApprovers,
	})
	if err != nil {
		return err
	}
	a.approvalIssueNumber = a.approvalIssue.GetNumber()

	fmt.Printf("Issue created: %s\n", a.approvalIssue.GetURL())
	return nil
}

func approvalFromComments(comments []*github.IssueComment, approvers []string, minimumApprovals int) (approvalStatus, error) {
	remainingApprovers := make([]string, len(approvers))
	copy(remainingApprovers, approvers)

	if minimumApprovals == 0 {
		minimumApprovals = len(approvers)
	}

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
			if len(remainingApprovers) == len(approvers)-minimumApprovals+1 {
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
		matched, err := regexp.MatchString(fmt.Sprintf("(?i)^%s[.!]*\n*$", approvedWord), commentBody)
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
		matched, err := regexp.MatchString(fmt.Sprintf("(?i)^%s[.!]*\n*$", deniedWord), commentBody)
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
