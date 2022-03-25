package main

import (
	"testing"

	"github.com/google/go-github/v43/github"
)

func TestApprovalFromComments(t *testing.T) {
	login1 := "login1"
	login2 := "login2"
	bodyApproved := "Approved"
	bodyDenied := "Denied"
	bodyPending := "not approval or denial"

	testCases := []struct {
		name           string
		comments       []*github.IssueComment
		approvers      []string
		expectedStatus approvalStatus
	}{
		{
			name: "single_approver_single_comment_approved",
			comments: []*github.IssueComment{
				{
					User: &github.User{Login: &login1},
					Body: &bodyApproved,
				},
			},
			approvers:      []string{login1},
			expectedStatus: approvalStatusApproved,
		},
		{
			name: "single_approver_single_comment_denied",
			comments: []*github.IssueComment{
				{
					User: &github.User{Login: &login1},
					Body: &bodyDenied,
				},
			},
			approvers:      []string{login1},
			expectedStatus: approvalStatusDenied,
		},
		{
			name: "single_approver_single_comment_pending",
			comments: []*github.IssueComment{
				{
					User: &github.User{Login: &login1},
					Body: &bodyPending,
				},
			},
			approvers:      []string{login1},
			expectedStatus: approvalStatusPending,
		},
		{
			name: "single_approver_multi_comment_approved",
			comments: []*github.IssueComment{
				{
					User: &github.User{Login: &login1},
					Body: &bodyPending,
				},
				{
					User: &github.User{Login: &login1},
					Body: &bodyApproved,
				},
			},
			approvers:      []string{login1},
			expectedStatus: approvalStatusApproved,
		},
		{
			name: "multi_approver_approved",
			comments: []*github.IssueComment{
				{
					User: &github.User{Login: &login1},
					Body: &bodyApproved,
				},
				{
					User: &github.User{Login: &login2},
					Body: &bodyApproved,
				},
			},
			approvers:      []string{login1, login2},
			expectedStatus: approvalStatusApproved,
		},
		{
			name: "multi_approver_mixed",
			comments: []*github.IssueComment{
				{
					User: &github.User{Login: &login1},
					Body: &bodyPending,
				},
				{
					User: &github.User{Login: &login2},
					Body: &bodyApproved,
				},
			},
			approvers:      []string{login1, login2},
			expectedStatus: approvalStatusPending,
		},
		{
			name: "multi_approver_denied",
			comments: []*github.IssueComment{
				{
					User: &github.User{Login: &login1},
					Body: &bodyDenied,
				},
				{
					User: &github.User{Login: &login2},
					Body: &bodyApproved,
				},
			},
			approvers:      []string{login1, login2},
			expectedStatus: approvalStatusDenied,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actual := approvalFromComments(testCase.comments, testCase.approvers)
			if actual != testCase.expectedStatus {
				t.Fatalf("actual %s, expected %s", actual, testCase.expectedStatus)
			}
		})
	}
}
