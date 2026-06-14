# GitHub Approvals Example

An example Bach **Approval Provider** that turns GitHub issue labels into
Factory approvals over the `bach.approval.v1` stdio JSON-RPC protocol.

## How it works

- An open issue labeled `approved/plan` approves the `plan` phase of every
  waiting Work Item whose intake source is `github_issue/<issue-number>`.
- Labels use a configurable prefix (default `approved/`); the suffix is the
  canonical phase, for example `approved/deploy.production`.
- The approver identity comes from the most recent labeled event on the issue.
- Bach still validates every record against the waiting Work Item, phase,
  attempt, and workflow gates before storing durable approval evidence.

Requires the `gh` CLI authenticated against the target repository
(`gh auth login` or `GH_TOKEN`).

## Provider command

```sh
go run github.com/applauselab/bachkator/examples/github-approvals@latest
```

Inside this repository:

```sh
go run ./examples/github-approvals
```

## Factory wiring

```hcl
factory "sldc" {
  workflow "ship" {
    plan {
      agent_template = agent_template.planner
      path           = "plans/factory/${work_item.id}.md"
    }

    implement {
      agent_template = agent_template.implementer
    }
  }

  approvals {
    provider "github" {
      command       = ["go", "run", "./examples/github-approvals"]
      poll_interval = "1m"

      config = {
        repo         = "owner/repo"
        label_prefix = "approved/"
      }
    }
  }
}
```

The config keys map to:

- `repo`: `owner/repo` scanned for open labeled issues (required).
- `label_prefix`: label prefix treated as approvals (default `approved/`).

Approvals match by source identity: create Work Items from a Trigger Provider
that reports the same source type and ID, for example
`source.type = "github_issue"` and `source.id = "42"` for issue #42.
