package main

import (
	"context"
	"fmt"
	"os"
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
	issueBody           string
	issueApprovers      []string
	minimumApprovals    int
	targetRepoOwner     string
	targetRepoName      string
	failOnDenial        bool
}

func newApprovalEnvironment(client *github.Client, repoFullName, repoOwner string, runID int, approvers []string, minimumApprovals int, issueTitle, issueBody string, targetRepoOwner string, targetRepoName string, failOnDenial bool) (*approvalEnvironment, error) {
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
		issueBody:        issueBody,
		targetRepoOwner:  targetRepoOwner,
		targetRepoName:   targetRepoName,
		failOnDenial:     failOnDenial,
	}, nil
}

func (a approvalEnvironment) runURL() string {
	baseUrl := a.client.BaseURL.String()
	if strings.Contains(baseUrl, "github.com") {
		baseUrl = "https://github.com/"
	}
	return fmt.Sprintf("%s%s/actions/runs/%d", baseUrl, a.repoFullName, a.runID)
}

func (a *approvalEnvironment) createApprovalIssue(ctx context.Context) error {
	issueTitle := fmt.Sprintf("Manual approval required for workflow run %d", a.runID)

	if a.issueTitle != "" {
		issueTitle = a.issueTitle
	}

	approversBody := ""
	for _, approver := range a.issueApprovers {
		approversBody = fmt.Sprintf("%s> * @%s\n", approversBody, approver)
	}

	issueBody := fmt.Sprintf(`> Workflow is pending manual review.
> URL: %s

> [!IMPORTANT]
> Required approvers: 
%s

> [!TIP]
> Respond %s to continue workflow or %s to cancel.`,
		a.runURL(),
		approversBody,
		formatAcceptedWords(approvedWords),
		formatAcceptedWords(deniedWords),
	)

	// if a.issueBody != "" {
	// 	issueBody = fmt.Sprintf(">%s\n>\n%s", a.issueBody, issueBody)
	// }
	issueBody = fmt.Sprintf(">[!NOTE]\n%s", issueBody)

	var err error
	fmt.Printf(
		"Creating issue in repo %s/%s with the following content:\nTitle: %s\nApprovers: %s\nBody:\n%s\n",
		a.targetRepoOwner,
		a.targetRepoName,
		issueTitle,
		a.issueApprovers,
		issueBody,
	)
	a.approvalIssue, _, err = a.client.Issues.Create(ctx, a.targetRepoOwner, a.targetRepoName, &github.IssueRequest{
		Title:     &issueTitle,
		Body:      &issueBody,
		Assignees: &a.issueApprovers,
	})
	if err != nil {
		return err
	}
	a.approvalIssueNumber = a.approvalIssue.GetNumber()

  bodyChunks := splitLongString(a.issueBody, 65536)
  for _, chunk := range bodyChunks {
      _, _, err = a.client.Issues.CreateComment(ctx, a.targetRepoOwner, a.targetRepoName, *a.approvalIssue.Number, &github.IssueComment{
          Body: &chunk,
      })
      if err != nil {
          return fmt.Errorf("failed to add comment chunk to issue: %w", err)
      }
  }

	fmt.Printf("Issue created: %s\n", a.approvalIssue.GetHTMLURL())
	return nil
}

func (a *approvalEnvironment) SetActionOutputs(outputs map[string]string) (bool, error) {
	outputFile := os.Getenv("GITHUB_OUTPUT")
	if outputFile == "" {
		return false, nil
	}

	f, err := os.OpenFile(outputFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	if err != nil {
		return false, err
	}
	defer f.Close()

	var pairs []string

	for key, value := range outputs {
		pairs = append(pairs, fmt.Sprintf("%s=%s", key, value))
	}

	// Add a newline before writing the new outputs if the file is not empty. This prevents
	// two outputs from being written on the same line.
	fileInfo, err := f.Stat()
	if err != nil {
			return false, err
	}
	if fileInfo.Size() > 0 {
			if _, err := f.WriteString("\n"); err != nil {
					return false, err
			}
	}

	if _, err := f.WriteString(strings.Join(pairs, "\n")); err != nil {
		return false, err
	}

	return true, nil
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
		re, err := regexp.Compile(fmt.Sprintf("(?i)^%s[.!]*\n*\\s*$", approvedWord))
		if err != nil {
			fmt.Printf("Error parsing. %v", err)
			return false, err
		}

		matched := re.MatchString(commentBody)

		if matched {
			return true, nil
		}
	}

	return false, nil
}

func isDenied(commentBody string) (bool, error) {
	for _, deniedWord := range deniedWords {
		re, err := regexp.Compile(fmt.Sprintf("(?i)^%s[.!]*\n*\\s*$", deniedWord))
		if err != nil {
			fmt.Printf("Error parsing. %v", err)
			return false, err
		}
		matched := re.MatchString(commentBody)
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

func splitLongLine(line string, maxL int) ([]string, bool) {
	if len(line) <= maxL {
		return []string{line}, false
	}

	words := strings.Fields(line)
	var result []string
	var currentLine string

	for _, word := range words {
		if len(currentLine)+len(word)+1 > maxL {
			result = append(result, currentLine)
			currentLine = word
		} else {
			if currentLine != "" {
				currentLine += " "
			}
			currentLine += word
		}
	}
	if currentLine != "" {
		result = append(result, currentLine)
	}
	return result, true
}

func splitLongString(input string, maxLength int) []string {
	var result []string

	lines := strings.Split(input, "\n")
	currentChunk := strings.Builder{}
	currentLength := 0

	for i, line := range lines {
    lineLength := len(line)
		if i < len(lines)-1 {
			lineLength++
    }

		if currentLength+lineLength > maxLength {
			if currentChunk.Len() > 0 {
				result = append(result, currentChunk.String())
				currentChunk.Reset()
				currentLength = 0
			}
		}

		lineSplit, isLongLine := splitLongLine(line, maxLength)
		if isLongLine {
			if currentChunk.Len() > 0 {
				result = append(result, currentChunk.String())
				currentChunk.Reset()
			}
			result = append(result, lineSplit[:len(lineSplit)-1]...)
			currentChunk.WriteString(lineSplit[len(lineSplit)-1])
			currentLength = len(lineSplit[len(lineSplit)-1])
		} else {
			currentChunk.WriteString(line)
			currentLength += lineLength
		}

		if i < len(lines)-1 {
			currentChunk.WriteString("\n")
		}
	}
	if currentChunk.Len() > 0 {
		result = append(result, currentChunk.String())
	}
	return result
}

