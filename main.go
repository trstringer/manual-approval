package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v43/github"
	"golang.org/x/oauth2"
)

func newGithubClient(ctx context.Context) *github.Client {
	token := os.Getenv(envVarToken)
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	return github.NewClient(tc)
}

func main() {
	repoFullName := os.Getenv(envVarRepoFullName)
	runID, err := strconv.Atoi(os.Getenv(envVarRunID))
	if err != nil {
		fmt.Printf("error getting runID: %v\n", err)
		os.Exit(1)
	}
	repoOwner := os.Getenv(envVarRepoOwner)

	ctx := context.Background()
	client := newGithubClient(ctx)

	requiredApproversRaw := os.Getenv(envVarApprovers)
	fmt.Printf("Required approvers: %s\n", requiredApproversRaw)
	approvers := strings.Split(requiredApproversRaw, ",")

	minimumApprovalsRaw := os.Getenv(envVarMinimumApprovals)
	minimumApprovals := len(approvers)
	if minimumApprovalsRaw != "" {
		minimumApprovals, err = strconv.Atoi(minimumApprovalsRaw)
		if err != nil {
			fmt.Printf("error parsing minimum number of approvals: %v\n", err)
			os.Exit(1)
		}
	}

	if minimumApprovals > len(approvers) {
		fmt.Printf("error: minimum required approvals (%v) is greater than the total number of approvers (%v)\n", minimumApprovals, len(approvers))
		os.Exit(1)
	}

	apprv, err := newApprovalEnvironment(client, repoFullName, repoOwner, runID, approvers, minimumApprovals)
	if err != nil {
		fmt.Printf("error creating approval environment: %v\n", err)
		os.Exit(1)
	}

	err = apprv.createApprovalIssue(ctx)
	if err != nil {
		fmt.Printf("error creating issue: %v", err)
		os.Exit(1)
	}

commentLoop:
	for {
		comments, _, err := client.Issues.ListComments(ctx, apprv.repoOwner, apprv.repo, apprv.approvalIssueNumber, &github.IssueListCommentsOptions{})
		if err != nil {
			fmt.Printf("error getting comments: %v\n", err)
			os.Exit(1)
		}

		approved, err := approvalFromComments(comments, approvers, minimumApprovals)
		if err != nil {
			fmt.Printf("error getting approval from comments: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Workflow status: %s\n", approved)
		switch approved {
		case approvalStatusApproved:
			newState := "closed"
			closeComment := "All approvers have approved, continuing workflow and closing this issue."
			_, _, err := client.Issues.CreateComment(ctx, apprv.repoOwner, apprv.repo, apprv.approvalIssueNumber, &github.IssueComment{
				Body: &closeComment,
			})
			if err != nil {
				fmt.Printf("error commenting on issue: %v\n", err)
				os.Exit(1)
			}
			_, _, err = client.Issues.Edit(ctx, apprv.repoOwner, apprv.repo, apprv.approvalIssueNumber, &github.IssueRequest{State: &newState})
			if err != nil {
				fmt.Printf("error closing issue: %v\n", err)
				os.Exit(1)
			}
			break commentLoop
		case approvalStatusDenied:
			newState := "closed"
			closeComment := "Request denied. Closing issue and failing workflow."
			_, _, err := client.Issues.CreateComment(ctx, apprv.repoOwner, apprv.repo, apprv.approvalIssueNumber, &github.IssueComment{
				Body: &closeComment,
			})
			if err != nil {
				fmt.Printf("error commenting on issue: %v\n", err)
				os.Exit(1)
			}
			_, _, err = client.Issues.Edit(ctx, apprv.repoOwner, apprv.repo, apprv.approvalIssueNumber, &github.IssueRequest{State: &newState})
			if err != nil {
				fmt.Printf("error closing issue: %v\n", err)
				os.Exit(1)
			}
			os.Exit(1)
		}

		time.Sleep(pollingInterval)
	}

	fmt.Println("Workflow manual approval completed")
}
