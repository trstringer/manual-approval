package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-github/v43/github"
)

func TestApprovalFromCommentsAllowCommentReasons(t *testing.T) {
	comment := func(user, body string) *github.IssueComment {
		return &github.IssueComment{User: &github.User{Login: github.String(user)}, Body: github.String(body)}
	}
	testCases := []struct {
		name     string
		comments []*github.IssueComment
		minimum  int
		enabled  approvalStatus
		disabled approvalStatus
	}{
		{"approval_with_reason", []*github.IssueComment{comment("login1", "Approved.\nDenied is explanatory text only.")}, 1, approvalStatusApproved, approvalStatusPending},
		{"denial_with_crlf_reason", []*github.IssueComment{comment("login1", "Denied!\r\nApproved is explanatory text only.")}, 1, approvalStatusDenied, approvalStatusPending},
		{"exact_approval", []*github.IssueComment{comment("LOGIN1", "APPROVED!\n")}, 1, approvalStatusApproved, approvalStatusApproved},
		{"exact_denial", []*github.IssueComment{comment("login1", "Denied.\n")}, 1, approvalStatusDenied, approvalStatusDenied},
		{"decision_on_later_line", []*github.IssueComment{comment("login1", "Context first.\nApproved.")}, 1, approvalStatusPending, approvalStatusPending},
		{"empty_first_line", []*github.IssueComment{comment("login1", "\nApproved.")}, 1, approvalStatusPending, approvalStatusPending},
		{"same_line_explanation", []*github.IssueComment{comment("login1", "Approved. Checks passed.")}, 1, approvalStatusPending, approvalStatusPending},
		{"unauthorized_approval", []*github.IssueComment{comment("outsider", "Approved.\nChecks passed.")}, 1, approvalStatusPending, approvalStatusPending},
		{"unauthorized_denial", []*github.IssueComment{comment("outsider", "Denied.\nChecks failed.")}, 1, approvalStatusPending, approvalStatusPending},
		{"distinct_approvers", []*github.IssueComment{comment("login1", "Approved.\nFirst review."), comment("login2", "Approved.\nSecond review.")}, 2, approvalStatusApproved, approvalStatusPending},
		{"duplicate_approver", []*github.IssueComment{comment("login1", "Approved.\nFirst review."), comment("login1", "Approved.\nRepeated review.")}, 2, approvalStatusPending, approvalStatusPending},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			for _, allowReasons := range []bool{false, true} {
				t.Run(fmt.Sprintf("allow_reasons=%t", allowReasons), func(t *testing.T) {
					expected := testCase.disabled
					if allowReasons {
						expected = testCase.enabled
					}
					actual, err := approvalFromComments(testCase.comments, []string{"login1", "login2"}, testCase.minimum, allowReasons)
					if err != nil || actual != expected {
						t.Fatalf("got status %s, error %v; want %s", actual, err, expected)
					}
				})
			}
		})
	}
}

func TestCommentLoopAllowCommentReasons(t *testing.T) {
	for _, allowReasons := range []bool{false, true} {
		for _, decision := range []string{"Approved", "Denied"} {
			t.Run(fmt.Sprintf("allow_reasons=%t/%s", allowReasons, decision), func(t *testing.T) {
				var mu sync.Mutex
				polls, closingComments, closes := 0, 0, 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					mu.Lock()
					defer mu.Unlock()
					w.Header().Set("Content-Type", "application/json")
					var response any
					switch {
					case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/issues/1/comments":
						polls++
						comments := []*github.IssueComment{{
							User: &github.User{Login: github.String("login1")},
							Body: github.String(decision + ".\nThe deployment checks explain this decision."),
						}}
						if polls > 1 {
							comments = append(comments, &github.IssueComment{
								User: &github.User{Login: github.String("login1")}, Body: github.String(decision),
							})
						}
						response = comments
					case r.Method == http.MethodPost && r.URL.Path == "/repos/owner/repo/issues/1/comments":
						var comment github.IssueComment
						wantBody := "The required number of approvals (1) has been met; continuing workflow and closing this issue."
						if decision == "Denied" {
							wantBody = "Request denied. Closing issue and failing workflow."
						}
						if err := json.NewDecoder(r.Body).Decode(&comment); err != nil || comment.GetBody() != wantBody {
							t.Errorf("got closing comment %q, error %v; want %q", comment.GetBody(), err, wantBody)
						}
						closingComments++
						response = &github.IssueComment{}
					case r.Method == http.MethodPatch && r.URL.Path == "/repos/owner/repo/issues/1":
						var issue github.IssueRequest
						if err := json.NewDecoder(r.Body).Decode(&issue); err != nil || issue.GetState() != "closed" {
							t.Errorf("got close request %v, error %v", issue, err)
						}
						closes++
						response = &github.Issue{}
					default:
						t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
						http.Error(w, "unexpected request", http.StatusNotFound)
						return
					}
					if err := json.NewEncoder(w).Encode(response); err != nil {
						t.Errorf("encoding response: %v", err)
					}
				}))
				defer server.Close()
				client := github.NewClient(server.Client())
				baseURL, err := url.Parse(server.URL + "/")
				if err != nil {
					t.Fatal(err)
				}
				client.BaseURL = baseURL
				apprv := &approvalEnvironment{
					targetRepoOwner: "owner", targetRepoName: "repo", approvalIssueNumber: 1,
					issueApprovers: []string{"login1"}, minimumApprovals: 1,
					failOnDenial: true, allowCommentReasons: allowReasons,
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				result := newCommentLoopChannel(ctx, apprv, client, time.Millisecond)
				select {
				case exit := <-result:
					wantExit := 0
					if decision == "Denied" {
						wantExit = 1
					}
					if exit != wantExit {
						t.Fatalf("got exit %d, want %d", exit, wantExit)
					}
				case <-ctx.Done():
					t.Fatal("comment loop did not complete")
				}
				mu.Lock()
				defer mu.Unlock()
				wantPolls := 2
				if allowReasons {
					wantPolls = 1
				}
				if polls != wantPolls || closingComments != 1 || closes != 1 {
					t.Fatalf("got polls/comments/closes %d/%d/%d; want %d/1/1", polls, closingComments, closes, wantPolls)
				}
			})
		}
	}
}

func TestApprovalFromComments(t *testing.T) {
	login1 := "login1"
	login2 := "login2"
	login3 := "login3"
	bodyApproved := "Approved"
	bodyDenied := "Denied"
	bodyPending := "not approval or denial"

	login1u := strings.ToUpper(login1)

	testCases := []struct {
		name             string
		comments         []*github.IssueComment
		approvers        []string
		minimumApprovals int
		expectedStatus   approvalStatus
		allowReasons     bool
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
			name: "single_approver_approval_with_reason_enabled",
			comments: []*github.IssueComment{
				{
					User: &github.User{Login: &login1},
					Body: github.String("Approved.\nA later denied keyword is explanatory text only."),
				},
			},
			approvers:      []string{login1},
			expectedStatus: approvalStatusApproved,
			allowReasons:   true,
		},
		{
			name: "single_approver_denial_with_reason_enabled",
			comments: []*github.IssueComment{
				{
					User: &github.User{Login: &login1},
					Body: github.String("Denied!\r\nA later approved keyword is explanatory text only."),
				},
			},
			approvers:      []string{login1},
			expectedStatus: approvalStatusDenied,
			allowReasons:   true,
		},
		{
			name: "single_approver_reason_stays_pending_by_default",
			comments: []*github.IssueComment{
				{
					User: &github.User{Login: &login1},
					Body: github.String("Approved.\nThe deployment checks passed."),
				},
			},
			approvers:      []string{login1},
			expectedStatus: approvalStatusPending,
		},
		{
			name: "decision_keyword_after_first_line_stays_pending",
			comments: []*github.IssueComment{
				{
					User: &github.User{Login: &login1},
					Body: github.String("Context first.\nApproved."),
				},
			},
			approvers:      []string{login1},
			expectedStatus: approvalStatusPending,
			allowReasons:   true,
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
		{
			name: "single_approver_single_comment_approved_case_insensitive",
			comments: []*github.IssueComment{
				{
					User: &github.User{Login: &login1u},
					Body: &bodyApproved,
				},
			},
			approvers:      []string{login1},
			expectedStatus: approvalStatusApproved,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actual, err := approvalFromComments(testCase.comments, testCase.approvers, testCase.minimumApprovals, testCase.allowReasons)
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

func TestSaveOutput(t *testing.T) {
	testCases := []struct {
		name                string
		approvalIssueNumber int
		env_github_output   string
		isSuccess           bool
	}{
		{
			name:                "save_output_with_env",
			approvalIssueNumber: 123,
			env_github_output:   "./output.txt",
			isSuccess:           true,
		},
		{
			name:                "fail_save_output_without_env",
			approvalIssueNumber: 123,
			env_github_output:   "",
			isSuccess:           false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("GITHUB_OUTPUT", testCase.env_github_output)
			a := approvalEnvironment{
				client:              nil,
				repoFullName:        "",
				repo:                "",
				repoOwner:           "",
				runID:               -1,
				approvalIssueNumber: testCase.approvalIssueNumber,
				issueTitle:          "",
				issueBody:           "",
				issueApprovers:      nil,
				minimumApprovals:    0,
			}

			if err := os.Remove(testCase.env_github_output); err != nil && !os.IsNotExist(err) {
				t.Fatalf("failed to remove file: %v", err)
			}

			actual, err := a.SetActionOutputs(nil)

			if err != nil {
				t.Fatalf("error creating output file: %v: %v", testCase.env_github_output, err)
			}

			if actual != testCase.isSuccess {
				t.Fatalf("expected %v but got %v", testCase.isSuccess, actual)
			}

			if actual == true {
				if _, err := os.Stat(testCase.env_github_output); errors.Is(err, os.ErrNotExist) {
					t.Fatalf("expected create output file %v but it was not", testCase.env_github_output)
				}
			}
		})
	}
}
