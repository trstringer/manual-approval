package main

import (
	"context"
	"fmt"

	"github.com/google/go-github/v43/github"
)

// createLabelIfNotExists creates a label if it does not exist.
// It returns an error if the label does not exist and it fails to create it.
// It returns nil if the label already exists or if it was successfully created.
func createLabelIfNotExists(client *github.Client, repoOwner string, repoFullName string, label string) error {
	_, resp, err := client.Issues.GetLabel(context.Background(), repoOwner, repoFullName, label)
	if err != nil {
		if resp.StatusCode != 404 {
			return fmt.Errorf("error getting label: %w", err)
		}
	}

	if resp.StatusCode == 200 {
		return nil
	}

	_, _, err = client.Issues.CreateLabel(context.Background(), repoOwner, repoFullName, &github.Label{
		Name: &label,
	})
	if err != nil {
		return fmt.Errorf("error creating label: %w", err)
	}

	return nil
}
