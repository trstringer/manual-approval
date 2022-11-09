# Manual Workflow Approval

[![ci](https://github.com/trstringer/manual-approval/actions/workflows/ci.yaml/badge.svg)](https://github.com/trstringer/manual-approval/actions/workflows/ci.yaml)

Pause a GitHub Actions workflow and require manual approval from one or more approvers before continuing.

This is a very common feature for a deployment or release pipeline, and while [this functionality is available from GitHub](https://docs.github.com/en/actions/managing-workflow-runs/reviewing-deployments), it requires the use of environments and if you want to use this for private repositories then you need GitHub Enterprise. This action provides manual approval without the use of environments, and is freely available to use on private repositories.

*Note: This approval duration is subject to the broader 72 hours timeout for a workflow. So keep that in mind when figuring out how quickly an approver must respond.*

The way this action works is the following:

1. Workflow comes to the `manual-approval` action.
1. `manual-approval` will create an issue in the containing repository and assign it to the `approvers`.
1. If and once all approvers respond with an approved keyword, the workflow will continue.
1. If any of the approvers responds with a denied keyword, then the workflow will exit with a failed status.

* Approval keywords - "approve", "approved", "lgtm", "yes"
* Denied keywords - "deny", "denied", "no"

These are case insensitive with optional punctuation either a period or an exclamation mark.

In all cases, `manual-approval` will close the initial GitHub issue.

## Usage

```yaml
steps:
  - uses: trstringer/manual-approval@v1
    with:
      secret: ${{ github.TOKEN }}
      approvers: user1,user2,org-team1
      minimum-approvals: 1
      issue-title: "Deploying v1.3.5 to prod from staging"
```

- `approvers` is a comma-delimited list of all required approvers. An approver can either be a user or an org team. (*Note: Required approvers must have the ability to be set as approvers in the repository. If you add an approver that doesn't have this permission then you would receive an HTTP/402 Validation Failed error when running this action*)
- `minimum-approvals` is an integer that sets the minimum number of approvals required to progress the workflow. Defaults to ALL approvers.
- `issue-title` is a string that will be appended to the title of the issue.

## Org team approver

If you want to have `approvers` set to an org team, then you need to take a different approach. The default [GitHub Actions automatic token](https://docs.github.com/en/actions/security-guides/automatic-token-authentication#permissions-for-the-github_token) does not have the necessary permissions to list out team members. If you would like to use this then you need to generate a token from a GitHub App with the correct set of permissions.

Create a GitHub App with **read-only access to organization members**. Once the app is created, add a repo secret with the app ID. In the GitHub App settings, generate a private key and add that as a secret in the repo as well. You can get the app token by using the [`tibdex/github-app-token`](https://github.com/tibdex/github-app-token) GitHub Action:

```yaml
jobs:
  myjob:
    runs-on: ubuntu-latest
    steps:
      - name: Generate token
        id: generate_token
        uses: tibdex/github-app-token@v1
        with:
          app_id: ${{ secrets.APP_ID }}
          private_key: ${{ secrets.APP_PRIVATE_KEY }}
      - name: Wait for approval
        uses: trstringer/manual-approval@v1
        with:
          secret: ${{ steps.generate_token.outputs.token }}
          approvers: myteam
          minimum-approvals: 1
```

## Timeout

If you'd like to force a timeout of your workflow pause, you can specify `timeout-minutes` at either the [step](https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions#jobsjob_idstepstimeout-minutes) level or the [job](https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions#jobsjob_idtimeout-minutes) level.

For instance, if you want your manual approval step to timeout after an hour you could do the following:

```yaml
steps:
  - uses: trstringer/manual-approval@v1
    timeout-minutes: 60
    ...
```

## Permissions

For the action to create a new issue in your project, please ensure that the action has write permissions on issues. You may have to add the following to your workflow:

```yaml
permissions:
  issues: write
```

For more information on permissions, please look at the [GitHub documentation](https://docs.github.com/en/actions/using-jobs/assigning-permissions-to-jobs).

## Limitations

* While the workflow is paused, it will still continue to consume a concurrent job allocation out of the [max concurrent jobs](https://docs.github.com/en/actions/learn-github-actions/usage-limits-billing-and-administration#usage-limits).
* A job (including a paused job) will be failed [after 6 hours](https://docs.github.com/en/actions/learn-github-actions/usage-limits-billing-and-administration#usage-limits).
* A paused job is still running compute/instance/virtual machine and will continue to incur costs.
