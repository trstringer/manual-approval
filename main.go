package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v43/github"
	"golang.org/x/oauth2"
)

func handleInterrupt(ctx context.Context, client *github.Client, apprv *approvalEnvironment) {
	newState := "closed"
	closeComment := "Workflow cancelled, closing issue."
	fmt.Println(closeComment)
	_, _, err := client.Issues.CreateComment(ctx, apprv.repoOwner, apprv.repo, apprv.approvalIssueNumber, &github.IssueComment{
		Body: &closeComment,
	})
	if err != nil {
		fmt.Printf("error commenting on issue: %v\n", err)
		return
	}
	_, _, err = client.Issues.Edit(ctx, apprv.repoOwner, apprv.repo, apprv.approvalIssueNumber, &github.IssueRequest{State: &newState})
	if err != nil {
		fmt.Printf("error closing issue: %v\n", err)
		return
	}
}

func newCommentLoopChannel(ctx context.Context, apprv *approvalEnvironment, client *github.Client) chan int {
	channel := make(chan int)
	go func() {
		for {
			comments, _, err := client.Issues.ListComments(ctx, apprv.repoOwner, apprv.repo, apprv.approvalIssueNumber, &github.IssueListCommentsOptions{})
			if err != nil {
				fmt.Printf("error getting comments: %v\n", err)
				channel <- 1
				close(channel)
			}

			approved, err := approvalFromComments(comments, apprv.issueApprovers, apprv.minimumApprovals)
			if err != nil {
				fmt.Printf("error getting approval from comments: %v\n", err)
				channel <- 1
				close(channel)
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
					channel <- 1
					close(channel)
				}
				_, _, err = client.Issues.Edit(ctx, apprv.repoOwner, apprv.repo, apprv.approvalIssueNumber, &github.IssueRequest{State: &newState})
				if err != nil {
					fmt.Printf("error closing issue: %v\n", err)
					channel <- 1
					close(channel)
				}
				channel <- 0
				fmt.Println("Workflow manual approval completed")
				close(channel)
			case approvalStatusDenied:
				newState := "closed"
				closeComment := "Request denied. Closing issue and failing workflow."
				_, _, err := client.Issues.CreateComment(ctx, apprv.repoOwner, apprv.repo, apprv.approvalIssueNumber, &github.IssueComment{
					Body: &closeComment,
				})
				if err != nil {
					fmt.Printf("error commenting on issue: %v\n", err)
					channel <- 1
					close(channel)
				}
				_, _, err = client.Issues.Edit(ctx, apprv.repoOwner, apprv.repo, apprv.approvalIssueNumber, &github.IssueRequest{State: &newState})
				if err != nil {
					fmt.Printf("error closing issue: %v\n", err)
					channel <- 1
					close(channel)
				}
				channel <- 1
				close(channel)
			}

			time.Sleep(pollingInterval)
		}
	}()
	return channel
}

func newGithubClient(ctx context.Context) (*github.Client, error) {
	token := os.Getenv(envVarToken)
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	serverUrl, serverUrlPresent := os.LookupEnv("GITHUB_SERVER_URL")
	apiUrl, apiUrlPresent := os.LookupEnv("GITHUB_API_URL")

	if serverUrlPresent {
		if !apiUrlPresent {
			apiUrl = serverUrl
		}
		return github.NewEnterpriseClient(apiUrl, serverUrl, tc)
	}
	return github.NewClient(tc), nil
}

func validateInput() error {
	missingEnvVars := []string{}
	if os.Getenv(envVarRepoFullName) == "" {
		missingEnvVars = append(missingEnvVars, envVarRepoFullName)
	}

	if os.Getenv(envVarRunID) == "" {
		missingEnvVars = append(missingEnvVars, envVarRunID)
	}

	if os.Getenv(envVarRepoOwner) == "" {
		missingEnvVars = append(missingEnvVars, envVarRepoOwner)
	}

	if os.Getenv(envVarToken) == "" {
		missingEnvVars = append(missingEnvVars, envVarToken)
	}

	if os.Getenv(envVarApprovers) == "" {
		missingEnvVars = append(missingEnvVars, envVarApprovers)
	}

	if len(missingEnvVars) > 0 {
		return fmt.Errorf("missing env vars: %v", missingEnvVars)
	}
	return nil
}

func main() {
	if err := validateInput(); err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}

	repoFullName := os.Getenv(envVarRepoFullName)
	runID, err := strconv.Atoi(os.Getenv(envVarRunID))
	if err != nil {
		fmt.Printf("error getting runID: %v\n", err)
		os.Exit(1)
	}
	repoOwner := os.Getenv(envVarRepoOwner)

	ctx := context.Background()
	client, err := newGithubClient(ctx)
	if err != nil {
		fmt.Printf("error connecting to server: %v\n", err)
		os.Exit(1)
	}

	approvers, err := retrieveApprovers(client, repoOwner)
	if err != nil {
		fmt.Printf("error retrieving approvers: %v\n", err)
		os.Exit(1)
	}

	issueTitle := os.Getenv(envVarIssueTitle)
	issueBody := os.Getenv(envVarIssueBody)
	minimumApprovalsRaw := os.Getenv(envVarMinimumApprovals)
	minimumApprovals := 0
	if minimumApprovalsRaw != "" {
		minimumApprovals, err = strconv.Atoi(minimumApprovalsRaw)
		if err != nil {
			fmt.Printf("error parsing minimum approvals: %v\n", err)
			os.Exit(1)
		}
	}
	issueLabels := strings.Split(os.Getenv(envVarIssueLabels), ",")
	apprv, err := newApprovalEnvironment(
		client,
		repoFullName,
		repoOwner,
		runID,
		approvers,
		minimumApprovals,
		issueTitle,
		issueBody,
		issueLabels,
	)
	if err != nil {
		fmt.Printf("error creating approval environment: %v\n", err)
		os.Exit(1)
	}

	err = apprv.createApprovalIssue(ctx)
	if err != nil {
		fmt.Printf("error creating issue: %v", err)
		os.Exit(1)
	}

	killSignalChannel := make(chan os.Signal, 1)
	signal.Notify(killSignalChannel, os.Interrupt)

	commentLoopChannel := newCommentLoopChannel(ctx, apprv, client)

	select {
	case exitCode := <-commentLoopChannel:
		os.Exit(exitCode)
	case <-killSignalChannel:
		handleInterrupt(ctx, client, apprv)
		os.Exit(1)
	}
}
