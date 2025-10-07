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
	_, _, err := client.Issues.CreateComment(ctx, apprv.targetRepoOwner, apprv.targetRepoName, apprv.approvalIssueNumber, &github.IssueComment{
		Body: &closeComment,
	})
	if err != nil {
		fmt.Printf("error commenting on issue: %v\n", err)
		return
	}
	_, _, err = client.Issues.Edit(ctx, apprv.targetRepoOwner, apprv.targetRepoName, apprv.approvalIssueNumber, &github.IssueRequest{State: &newState})
	if err != nil {
		fmt.Printf("error closing issue: %v\n", err)
		return
	}
}

func newCommentLoopChannel(ctx context.Context, apprv *approvalEnvironment, client *github.Client, pollingInterval time.Duration) chan int {
	channel := make(chan int)
	go func() {
		for {
			comments, _, err := client.Issues.ListComments(ctx, apprv.targetRepoOwner, apprv.targetRepoName, apprv.approvalIssueNumber, &github.IssueListCommentsOptions{})
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
				closeComment := fmt.Sprintf("The required number of approvals (%d) has been met; continuing workflow and closing this issue.", apprv.minimumApprovals)
				_, _, err := client.Issues.CreateComment(ctx, apprv.targetRepoOwner, apprv.targetRepoName, apprv.approvalIssueNumber, &github.IssueComment{
					Body: &closeComment,
				})
				if err != nil {
					fmt.Printf("error commenting on issue: %v\n", err)
					channel <- 1
					close(channel)
				}
				_, _, err = client.Issues.Edit(ctx, apprv.targetRepoOwner, apprv.targetRepoName, apprv.approvalIssueNumber, &github.IssueRequest{State: &newState})
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
				closeComment := "Request denied. Closing issue "
				if !apprv.failOnDenial {
					closeComment += "but continuing"
				} else {
					closeComment += "and failing"
				}
				closeComment += " workflow."

				_, _, err := client.Issues.CreateComment(ctx, apprv.targetRepoOwner, apprv.targetRepoName, apprv.approvalIssueNumber, &github.IssueComment{
					Body: &closeComment,
				})
				if err != nil {
					fmt.Printf("error commenting on issue: %v\n", err)
					channel <- 1
					close(channel)
				}
				_, _, err = client.Issues.Edit(ctx, apprv.targetRepoOwner, apprv.targetRepoName, apprv.approvalIssueNumber, &github.IssueRequest{State: &newState})
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

	targetRepoName := os.Getenv(envVarTargetRepo)
	targetRepoOwner := os.Getenv(envVarTargetRepoOwner)

	repoFullName := os.Getenv(envVarRepoFullName)
	runID, err := strconv.Atoi(os.Getenv(envVarRunID))
	if err != nil {
		fmt.Printf("error getting runID: %v\n", err)
		os.Exit(1)
	}
	repoOwner := os.Getenv(envVarRepoOwner)

	if targetRepoName == "" || targetRepoOwner == "" {
		parts := strings.SplitN(repoFullName, "/", 2)
		targetRepoOwner = parts[0]
		targetRepoName = parts[1]
	}

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

	failOnDenial := true
	failOnDenialRaw := os.Getenv(envVarFailOnDenial)
	if failOnDenialRaw != "" {
		failOnDenial, err = strconv.ParseBool(failOnDenialRaw)
		if err != nil {
			fmt.Printf("error parsing fail on denial: %v\n", err)
			os.Exit(1)
		}
	}

	pollingInterval := defaultPollingInterval
	pollingIntervalSecondsRaw := os.Getenv(envVarPollingIntervalSeconds)
	if pollingIntervalSecondsRaw != "" {
		pollingIntervalSeconds, err := strconv.Atoi(pollingIntervalSecondsRaw)
		if err != nil {
			fmt.Printf("error parsing polling interval: %v\n", err)
			os.Exit(1)
		}
		if pollingIntervalSeconds <= 0 {
			fmt.Printf("error: polling interval must be greater than 0\n")
			os.Exit(1)
		}
		pollingInterval = time.Duration(pollingIntervalSeconds) * time.Second
	}

	issueTitle := os.Getenv(envVarIssueTitle)
	var issueBody string
	if os.Getenv(envVarIssueBodyFilePath) != "" {
		fileContents, err := os.ReadFile(os.Getenv(envVarIssueBodyFilePath))
		if err != nil {
			fmt.Printf("error reading issue body file: %v\n", err)
			os.Exit(1)
		}
		issueBody = string(fileContents)
	} else {
		issueBody = os.Getenv(envVarIssueBody)
	}
	minimumApprovalsRaw := os.Getenv(envVarMinimumApprovals)
	minimumApprovals := 0
	if minimumApprovalsRaw != "" {
		minimumApprovals, err = strconv.Atoi(minimumApprovalsRaw)
		if err != nil {
			fmt.Printf("error parsing minimum approvals: %v\n", err)
			os.Exit(1)
		}
	}

	apprv, err := newApprovalEnvironment(client, repoFullName, repoOwner, runID, approvers, minimumApprovals, issueTitle, issueBody, targetRepoOwner, targetRepoName, failOnDenial)
	if err != nil {
		fmt.Printf("error creating approval environment: %v\n", err)
		os.Exit(1)
	}

	err = apprv.createApprovalIssue(ctx)
	if err != nil {
		fmt.Printf("error creating issue: %v", err)
		os.Exit(1)
	}

	outputs := map[string]string {
		"issue-number": fmt.Sprintf("%d", apprv.approvalIssueNumber),
		"issue-url": apprv.approvalIssue.GetHTMLURL(),
	}
	_, err = apprv.SetActionOutputs(outputs)
	if err != nil {
		fmt.Printf("error saving output: %v", err)
		os.Exit(1)
	}

	killSignalChannel := make(chan os.Signal, 1)
	signal.Notify(killSignalChannel, os.Interrupt)

	commentLoopChannel := newCommentLoopChannel(ctx, apprv, client, pollingInterval)

	select {
	case exitCode := <-commentLoopChannel:
		approvalStatus := ""

		if (!failOnDenial && exitCode == 1) {
			approvalStatus = "denied"
			exitCode = 0
		} else if (exitCode == 1) {
			approvalStatus = "denied"
		} else {
			approvalStatus = "approved"
		}
		outputs := map[string]string {
			"approval-status": approvalStatus,
		}
		if _, err := apprv.SetActionOutputs(outputs); err != nil {
			fmt.Printf("error setting action output: %v\n", err)
			exitCode = 1
		}
		os.Exit(exitCode)
	case <-killSignalChannel:
		handleInterrupt(ctx, client, apprv)
		os.Exit(1)
	}
}
