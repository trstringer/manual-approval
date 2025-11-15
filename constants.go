package main

import (
	"os"
	"strings"
	"time"
)

const (
	defaultPollingInterval time.Duration = 10 * time.Second

	envVarRepoFullName                       string = "GITHUB_REPOSITORY"
	envVarRunID                              string = "GITHUB_RUN_ID"
	envVarRepoOwner                          string = "GITHUB_REPOSITORY_OWNER"
	envVarWorkflowInitiator                  string = "GITHUB_ACTOR"
	envVarToken                              string = "INPUT_SECRET"
	envVarApprovers                          string = "INPUT_APPROVERS"
	envVarMinimumApprovals                   string = "INPUT_MINIMUM-APPROVALS"
	envVarIssueTitle                         string = "INPUT_ISSUE-TITLE"
	envVarIssueBody                          string = "INPUT_ISSUE-BODY"
	envVarIssueBodyFilePath                  string = "INPUT_ISSUE-BODY-FILE-PATH"
	envVarExcludeWorkflowInitiatorAsApprover string = "INPUT_EXCLUDE-WORKFLOW-INITIATOR-AS-APPROVER"
	envVarAdditionalApprovedWords            string = "INPUT_ADDITIONAL-APPROVED-WORDS"
	envVarAdditionalDeniedWords              string = "INPUT_ADDITIONAL-DENIED-WORDS"
	envVarFailOnDenial                       string = "INPUT_FAIL-ON-DENIAL"
	envVarTargetRepoOwner                    string = "INPUT_TARGET-REPOSITORY-OWNER"
	envVarTargetRepo                         string = "INPUT_TARGET-REPOSITORY"
	envVarPollingIntervalSeconds             string = "INPUT_POLLING-INTERVAL-SECONDS"
)

var (
	additionalApprovedWords = readAdditionalWords(envVarAdditionalApprovedWords)
	additionalDeniedWords   = readAdditionalWords(envVarAdditionalDeniedWords)

	approvedWords = append([]string{"approved", "approve", "lgtm", "yes"}, additionalApprovedWords...)
	deniedWords   = append([]string{"denied", "deny", "no"}, additionalDeniedWords...)
)

func readAdditionalWords(envVar string) []string {
	rawValue := strings.TrimSpace(os.Getenv(envVar))
	if len(rawValue) == 0 {
		// Nothing else to do here.
		return []string{}
	}
	slicedWords := strings.Split(rawValue, ",")
	for i := range slicedWords {
		// no leading or trailing spaces in user provided words.
		slicedWords[i] = strings.TrimSpace(slicedWords[i])
	}
	return slicedWords
}
