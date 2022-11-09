package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/google/go-github/v43/github"
)

func retrieveApprovers(client *github.Client, repoOwner string) ([]string, error) {
	workflowInitiator := os.Getenv(envVarWorkflowInitiator)
	shouldIncludeWorkflowInitiatorRaw := os.Getenv(envVarAllowWorkflowInitiatorAsApprover)
	shouldIncludeWorkflowInitiator, parseBoolErr := strconv.ParseBool(shouldIncludeWorkflowInitiatorRaw)
	if parseBoolErr != nil {
		return nil, fmt.Errorf("error parsing allow-workflow-initiator-as-approver flag: %w", parseBoolErr)
	}

	approvers := []string{}
	requiredApproversRaw := os.Getenv(envVarApprovers)
	requiredApprovers := strings.Split(requiredApproversRaw, ",")

	for i := range requiredApprovers {
		requiredApprovers[i] = strings.TrimSpace(requiredApprovers[i])
	}

	for _, approverUser := range requiredApprovers {
		expandedUsers := expandGroupFromUser(client, repoOwner, approverUser, workflowInitiator, shouldIncludeWorkflowInitiator)
		if expandedUsers != nil {
			approvers = append(approvers, expandedUsers...)
		} else if strings.EqualFold(workflowInitiator, approverUser) && !shouldIncludeWorkflowInitiator {
			fmt.Printf("Not adding user '%s' as an approver as they are the workflow initiator\n", approverUser)
		} else {
			approvers = append(approvers, approverUser)
		}
	}

	approvers = deduplicateUsers(approvers)

	minimumApprovalsRaw := os.Getenv(envVarMinimumApprovals)
	minimumApprovals := len(approvers)
	var err error
	if minimumApprovalsRaw != "" {
		minimumApprovals, err = strconv.Atoi(minimumApprovalsRaw)
		if err != nil {
			return nil, fmt.Errorf("error parsing minimum number of approvals: %w", err)
		}
	}

	if minimumApprovals > len(approvers) {
		return nil, fmt.Errorf("error: minimum required approvals (%d) is greater than the total number of approvers (%d)", minimumApprovals, len(approvers))
	}

	return approvers, nil
}

func expandGroupFromUser(client *github.Client, org, userOrTeam string, workflowInitiator string, shouldIncludeWorkflowInitiator bool) []string {
	fmt.Printf("Attempting to expand user %s/%s as a group (may not succeed)\n", org, userOrTeam)
	users, _, err := client.Teams.ListTeamMembersBySlug(context.Background(), org, userOrTeam, &github.TeamListTeamMembersOptions{})
	if err != nil {
		fmt.Printf("%v\n", err)
		return nil
	}

	userNames := make([]string, 0, len(users))
	for _, user := range users {
		userName := user.GetLogin()
		if strings.EqualFold(userName, workflowInitiator) && !shouldIncludeWorkflowInitiator {
			fmt.Printf("Not adding user '%s' from group '%s' as an approver as they are the workflow initiator\n", userName, userOrTeam)
		} else {
			userNames = append(userNames, userName)
		}
	}

	return userNames
}

func deduplicateUsers(users []string) []string {
	uniqValuesByKey := make(map[string]bool)
	uniqUsers := []string{}
	for _, user := range users {
		if _, ok := uniqValuesByKey[user]; !ok {
			uniqValuesByKey[user] = true
			uniqUsers = append(uniqUsers, user)
		}
	}
	return uniqUsers
}
