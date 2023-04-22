package main

import (
	"testing"

	"github.com/google/go-github/v43/github"
)

func TestApprovalFromComments(t *testing.T) {
	login1 := "login1"
	login2 := "login2"
	login3 := "login3"
	bodyApproved := "Approved"
	bodyDenied := "Denied"
	bodyPending := "not approval or denial"

	testCases := []struct {
		name             string
		comments         []*github.IssueComment
		approvers        []string
		minimumApprovals int
		expectedStatus   approvalStatus
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
		{
			name: "multi_approver_minimum_one_approval",
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
			approvers:        []string{login1, login2},
			expectedStatus:   approvalStatusApproved,
			minimumApprovals: 1,
		},
		{
			name: "multi_approver_minimum_two_approvals",
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
			approvers:        []string{login1, login2, login3},
			expectedStatus:   approvalStatusApproved,
			minimumApprovals: 2,
		},
		{
			name: "multi_approver_approvals_less_than_minimum",
			comments: []*github.IssueComment{
				{
					User: &github.User{Login: &login1},
					Body: &bodyApproved,
				},
			},
			approvers:        []string{login1, login2, login3},
			expectedStatus:   approvalStatusPending,
			minimumApprovals: 2,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actual, err := approvalFromComments(testCase.comments, testCase.approvers, testCase.minimumApprovals)
			if err != nil {
				t.Fatalf("error getting approval from comments: %v", err)
			}

			if actual != testCase.expectedStatus {
				t.Fatalf("actual %s, expected %s", actual, testCase.expectedStatus)
			}
		})
	}
}

func TestApprovedCommentBody(t *testing.T) {
	testCases := []struct {
		name               string
		commentBody        string
		isSuccess          bool
		customApprovalWord string
	}{
		{
			name:               "approved_lowercase_no_punctuation",
			commentBody:        "approved",
			isSuccess:          true,
			customApprovalWord: "",
		},
		{
			name:               "approve_lowercase_no_punctuation",
			commentBody:        "approve",
			isSuccess:          true,
			customApprovalWord: "",
		},
		{
			name:               "lgtm_lowercase_no_punctuation",
			commentBody:        "lgtm",
			isSuccess:          true,
			customApprovalWord: "",
		},
		{
			name:               "yes_lowercase_no_punctuation",
			commentBody:        "yes",
			isSuccess:          true,
			customApprovalWord: "",
		},
		{
			name:               "approve_uppercase_no_punctuation",
			commentBody:        "APPROVE",
			isSuccess:          true,
			customApprovalWord: "",
		},
		{
			name:               "approved_titlecase_period",
			commentBody:        "Approved.",
			isSuccess:          true,
			customApprovalWord: "",
		},
		{
			name:               "approved_titlecase_exclamation",
			commentBody:        "Approved!",
			isSuccess:          true,
			customApprovalWord: "",
		},
		{
			name:               "approved_titlecase_multi_exclamation",
			commentBody:        "Approved!!",
			isSuccess:          true,
			customApprovalWord: "",
		},
		{
			name:               "approved_titlecase_question",
			commentBody:        "Approved?",
			isSuccess:          false,
			customApprovalWord: "",
		},
		{
			name:               "sentence_with_keyword",
			commentBody:        "should i approve this",
			isSuccess:          false,
			customApprovalWord: "",
		},
		{
			name:               "sentence_without_keyword",
			commentBody:        "this is just some random comment",
			isSuccess:          false,
			customApprovalWord: "",
		},
		{
			name:               "approved_with_newline",
			commentBody:        "approved\n",
			isSuccess:          true,
			customApprovalWord: "",
		},
		{
			name:               "approved_with_exclamation_newline",
			commentBody:        "approved!\n",
			isSuccess:          true,
			customApprovalWord: "",
		},
		{
			name:               "approved_with_multi_exclamation_multi_newline",
			commentBody:        "approved!!!\n\n\n",
			isSuccess:          true,
			customApprovalWord: "",
		},
		{
			name:               "approved_with_custom_approval_word",
			commentBody:        "shipit",
			isSuccess:          true,
			customApprovalWord: "shipit",
		},
		{
			name:               "approved_with_github_emoji_syntax",
			commentBody:        ":shipit:",
			isSuccess:          true,
			customApprovalWord: ":shipit:",
		},
		{
			name:               "approved_with_custom_hashtag",
			commentBody:        "#shipit",
			isSuccess:          true,
			customApprovalWord: "#shipit",
		},
		{
			name:               "approved_with_actual_emoji_✅",
			commentBody:        "✅ ",
			isSuccess:          true,
			customApprovalWord: "✅",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// before each
			word := testCase.customApprovalWord
			if len(word) > 0 {
				approvedWords = append(approvedWords, word)
			}

			// test
			actual, err := isApproved(testCase.commentBody)
			if err != nil {
				t.Fatalf("error getting approval: %v", err)
			}
			if actual != testCase.isSuccess {
				t.Fatalf("expected %v but got %v", testCase.isSuccess, actual)
			}

			// after each
			if len(word) > 0 {
				approvedWords = approvedWords[:len(approvedWords)-1]
			}
		})
	}
}

func TestDeniedCommentBody(t *testing.T) {
	testCases := []struct {
		name             string
		commentBody      string
		isSuccess        bool
		customDenialWord string
	}{
		{
			name:             "denied_lowercase_no_punctuation",
			commentBody:      "denied",
			isSuccess:        true,
			customDenialWord: "",
		},
		{
			name:             "deny_lowercase_no_punctuation",
			commentBody:      "deny",
			isSuccess:        true,
			customDenialWord: "",
		},
		{
			name:             "no_lowercase_no_punctuation",
			commentBody:      "no",
			isSuccess:        true,
			customDenialWord: "",
		},
		{
			name:             "deny_uppercase_no_punctuation",
			commentBody:      "DENY",
			isSuccess:        true,
			customDenialWord: "",
		},
		{
			name:             "denied_titlecase_period",
			commentBody:      "Denied.",
			isSuccess:        true,
			customDenialWord: "",
		},
		{
			name:             "denied_titlecase_exclamation",
			commentBody:      "Denied!",
			isSuccess:        true,
			customDenialWord: "",
		},
		{
			name:             "deny_titlecase_question",
			commentBody:      "Deny?",
			isSuccess:        false,
			customDenialWord: "",
		},
		{
			name:             "sentence_with_keyword",
			commentBody:      "should i deny this",
			isSuccess:        false,
			customDenialWord: "",
		},
		{
			name:             "sentence_without_keyword",
			commentBody:      "this is just some random comment",
			isSuccess:        false,
			customDenialWord: "",
		},
		{
			name:             "denied_with_newline",
			commentBody:      "denied\n",
			isSuccess:        true,
			customDenialWord: "",
		},
		{
			name:             "denied_with_custom_word",
			commentBody:      "naw",
			isSuccess:        true,
			customDenialWord: "naw",
		},
		{
			name:             "denied_with_github_emoji",
			commentBody:      ":no_entry_sign: ",
			isSuccess:        true,
			customDenialWord: ":no_entry_sign:",
		},
		{
			name:             "denied_with_hashtag",
			commentBody:      "#noway",
			isSuccess:        true,
			customDenialWord: "#noway",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// before each
			word := testCase.customDenialWord
			if len(word) > 0 {
				deniedWords = append(deniedWords, word)
			}

			// test
			actual, err := isDenied(testCase.commentBody)
			if err != nil {
				t.Fatalf("error getting approval: %v", err)
			}
			if actual != testCase.isSuccess {
				t.Fatalf("expected %v but got %v", testCase.isSuccess, actual)
			}

			// after each
			if len(word) > 0 {
				deniedWords = deniedWords[:len(deniedWords)-1]
			}
		})
	}
}
