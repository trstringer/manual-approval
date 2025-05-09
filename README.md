
# Manual Workflow Approval

[![ci](https://github.com/trstringer/manual-approval/actions/workflows/ci.yaml/badge.svg)](https://github.com/trstringer/manual-approval/actions/workflows/ci.yaml)

Pause a GitHub Actions workflow and require manual approval from one or more approvers before continuing.

This is a very common feature for a deployment or release pipeline, and while [this functionality is available from GitHub](https://docs.github.com/en/actions/managing-workflow-runs/reviewing-deployments), it requires the use of environments and if you want to use this for private repositories, then you need GitHub Enterprise. This action provides manual approval without the use of environments, and is freely available to use on private repositories.

*Note: This approval duration is subject to the broader 35 day timeout for a workflow, as well as usage costs. So keep that in mind when figuring out how quickly an approver must respond. See [limitations](#limitations) for more information.*

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
      issue-body: "Please approve or deny the deployment of version v1.3.5."
      exclude-workflow-initiator-as-approver: false
      fail-on-denial: true
      additional-approved-words: ''
      additional-denied-words: ''
```

* `approvers` is a comma-delimited list of all required approvers. An approver can either be a user or an org team. (*Note: Required approvers must have the ability to be set as approvers in the repository. If you add an approver that doesn't have this permission then you would receive an HTTP/402 Validation Failed error when running this action*)
* `minimum-approvals` is an integer that sets the minimum number of approvals required to progress the workflow. Defaults to ALL approvers.
* `issue-title` is a string that will be appended to the title of the issue.
* `issue-body` is a string that will be prepended to the body of the issue.
* `exclude-workflow-initiator-as-approver` is a boolean that indicates if the workflow initiator (determined by the `GITHUB_ACTOR` environment variable) should be filtered from the final list of approvers. This is optional and defaults to `false`. Set this to `true` to prevent users in the `approvers` list from being able to self-approve workflows.
* `fail-on-denial` is a boolean that indicates if the workflow should fail if any approver denies the approval. This is optional and defaults to `true`. Set this to `false` to allow the workflow to continue if any approver denies the approval.
* `additional-approved-words` is a comma separated list of strings to expand the dictionary of words that indicate approval. This is optional and defaults to an empty string.
* `additional-denied-words` is a comma separated list of strings to expand the dictionary of words that indicate denial. This is optional and defaults to an empty string.

### Outputs

* `approval_status` is a string that indicates the final status of the approval. This will be either `approved` or `denied`.

### Creating Issues in a different repository

```yaml
steps:
  - uses: trstringer/manual-approval@v1
    with:
      secret: ${{ github.TOKEN }}
      approvers: user1,user2,org-team1
      minimum-approvals: 1
      issue-title: "Deploying v1.3.5 to prod from staging"
      issue-body: "Please approve or deny the deployment of version v1.3.5."
      exclude-workflow-initiator-as-approver: false
      additional-approved-words: ''
      additional-denied-words: ''
      target-repository: repository-name
      target-repository-owner: owner-id
```
- if either of `target-repository` or `target-repository-owner` is missing or is an empty string then the issue will be created in the same repository where this step is used.

### Using Custom Words

GitHub has a rich library of emojis, and these all work in additional approved words or denied words.  Some values GitHub will store in their text version - i.e. `:shipit:`. Other emojis, GitHub will store in their unicode emoji form, like ✅.
For a seamless experience, it is recommended that you add the custom words to a GitHub comment, and then copy it back out of the comment into your actions configuration yaml.

## Org team approver

If you want to have `approvers` set to an org team, then you need to take a different approach. The default [GitHub Actions automatic token](https://docs.github.com/en/actions/security-guides/automatic-token-authentication#permissions-for-the-github_token) does not have the necessary permissions to list out team members. If you would like to use this then you need to generate a token from a GitHub App with the correct set of permissions.

Create a GitHub App with **read-only access to organization members**. Once the app is created, add a repo secret with the app ID. In the GitHub App settings, generate a private key and add that as a secret in the repo as well. You can get the app token by using the [`tibdex/github-app-token`](https://github.com/tibdex/github-app-token) GitHub Action:

*Note: The GitHub App tokens expire after 1 hour which implies duration for the approval cannot exceed 60 minutes or the job will fail due to bad credentials. See [docs](https://docs.github.com/en/rest/apps/apps#create-an-installation-access-token-for-an-app).*

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

> **Note:** The `timeout-minutes` option has been removed from the `manual-approval` inputs, as it did nothing and incorrectly assured users that they were in fact
> getting timeout behavior.  Please use one of the below two approaches instead.
>
> If you are currently using `timeout-minutes` as a `manual-approval` input, you may see a warning, but this will not break your action.

For instance, if you want your manual approval step to timeout after an hour you could do the following:

```yaml
jobs:
  approval:
    steps:
      - uses: trstringer/manual-approval@v1
        timeout-minutes: 60
    ...
```
or 
```yaml
jobs:
  approval:
    timeout-minutes: 10
    steps:
      - uses: trstringer/manual-approval@v1

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
* A paused job is still running compute/instance/virtual machine and will continue to incur costs.
* Expirations (also mentioned elsewhere in this document):
  * A job (including a paused job) will be failed [after 6 hours, and a workflow will be failed after 35 days](https://docs.github.com/en/actions/learn-github-actions/usage-limits-billing-and-administration#usage-limits).
  * GitHub App tokens expire after 1 hour which implies duration for the approval cannot exceed 60 minutes or the job will fail due to bad credentials. See [docs](https://docs.github.com/en/rest/apps/apps#create-an-installation-access-token-for-an-app)

## Development

### Running test code

To test out your code in an action, you need to build the image and push it to a different container registry repository. For instance, if I want to test some code I won't build the image with the main image repository. Prior to this, comment out the label binding the image to a repo:

```dockerfile
# LABEL org.opencontainers.image.source https://github.com/trstringer/manual-approval
```

Build the image:

```shell
VERSION=1.7.1-rc.1 make IMAGE_REPO=ghcr.io/trstringer/manual-approval-test build
```

*Note: The image version can be whatever you want, as this image wouldn't be pushed to production. It is only for testing.*

Push the image to your container registry:

```shell
VERSION=1.7.1-rc.1 make IMAGE_REPO=ghcr.io/trstringer/manual-approval-test push
```

To test out the image you will need to modify `action.yaml` so that it points to your new image that you're testing:

```yaml
  image: docker://ghcr.io/trstringer/manual-approval-test:1.7.0-rc.1
```

Then to test out the image, run a workflow specifying your dev branch:

```yaml
- name: Wait for approval
  uses: your-github-user/manual-approval@your-dev-branch
  with:
    secret: ${{ secrets.GITHUB_TOKEN }}
    approvers: trstringer
```

For `uses`, this should point to your repo and dev branch.

*Note: To test out the action that uses an approver that is an org team, refer to the [org team approver](#org-team-approver) section for instructions.*

### Create a release

1. Build the new version's image: `$ VERSION=1.7.0 make build`
2. Push the new image: `$ VERSION=1.7.0 make push`
3. Create a release branch and modify `action.yaml` to point to the new image
4. Open and merge a PR to add these changes to the default branch
5. Make sure to fetch the new changes into your local repo: `$ git checkout main && git fetch origin && git merge origin main`
6. Delete the `v1` tag locally and remotely: `$ git tag -d v1 && git push --delete origin v1`
7. Create and push new tags: `$ git tag v1.7.0 && git tag v1 && git push origin --tags`
8. Create the GitHub project release
