## Factory Work Items

Factories declare durable queues for plan-first work. A Factory is not a Target and is hidden from
`bach list`.

```hcl
factory "sldc" {
  workflow "ship" {
    plan {
      agent_template = agent_template.planner
      path = "plans/factory/${work_item.id}.md"
    }

    implement {
      agent_template = agent_template.implementer
    }

    merge {
      target = pipeline/merge_ready
    }

    deploy "staging" {
      target = shell/deploy_staging
    }

    verify "staging" {
      target = shell/verify_staging
    }

    deploy "production" {
      target            = pipeline/deploy_production
      requires_approval = true
    }

    verify "production" {
      target = group/production_gate
    }
  }

  triggers {
    manual {}

    provider "github" {
      command = ["bach-trigger-fixture"]
      poll_interval = "5m"
      config = {
        items_path = ".bach/fixtures/trigger-items.json"
      }

      route {
        label    = "urgent"
        workflow = "hotfix"
      }
    }
  }
}
```

Factory fields:

- `factory "<name>"`: declares a queue namespace.
- `workflow "<name>"`: declares a route for submitted Work Items. A single workflow is selected by default; multiple workflows require `--workflow` on submit.
- `plan { agent_template = agent_template.<name>, path = "...", requires_approval = true }`: runs a planning Agent Target and copies exactly one Plan file from the agent workspace to the configured project-relative path. `${work_item.id}`, `${factory.name}`, and `${workflow.name}` are supported in `path`. `requires_approval` defaults to `true`; set it to `false` to allow unattended planning.
- `implement { agent_template = agent_template.<name> }`: runs `bach plan implement` internally with this implementation template override. The daemon does not mutate the planner-authored Plan file.
- `merge { target = <target> }`: runs one normal Bach target after implementation succeeds.
- `deploy "<name>" { target = <target>, requires_approval = true }` and `verify "<name>" { target = <target> }`: run named deployment and verification targets in declaration order. Deploy approvals default to `false`; set `requires_approval = true` to pause before the deploy target runs.
- `triggers { manual {} }`: enables `bach factory submit` for that Factory.
- `triggers { provider "<name>" { ... } }`: enables an external trigger provider process that the Factory daemon polls for new Work Items.

Provider trigger fields:

- `command`: required argv array for the provider process. The first element may be `bach` to resolve to the current Bach executable.
- `poll_interval`: optional duration string; defaults to `5m` and is clamped to at least `1s`.
- `config`: optional map of string keys to string values passed to the provider during handshake and poll.
- `route { label = "...", workflow = "..." }`: optional routing rule. Items with the matching label are routed to the named workflow. When a Factory has multiple workflows, at least one route is required; with a single workflow, omitted routes default to that workflow.

Validation rules:

- Factory and workflow names must start with an ASCII letter, digit, or `_`, and may then contain ASCII letters, digits, `_`, `.`, or `-`.
- Factory names must be unique within a Bachfile.
- A Factory must declare at least one workflow.
- Workflow names must be unique within a Factory.
- Daemon-executable workflows require exactly one `plan` block and exactly one `implement` block.
- `merge`, named `deploy`, and named `verify` phases are optional. Each phase block accepts singular `target`.
- `requires_approval` is accepted only on `plan` and `deploy` blocks. `implement`, `merge`, and `verify` reject the field.
- A Factory may declare at most one `triggers` block, at most one `manual` trigger block, and any number of named `provider` trigger blocks.
- Provider trigger names must be unique within a Factory and must be simple identifiers.
- Provider triggers require a non-empty `command` array.
- Provider triggers require at least one `route` block when the Factory declares multiple workflows.
- Unknown Factory fields are rejected by Bachfile validation.

Submit a Work Item:

```sh
bach factory submit sldc \
  --title "Ship billing webhook" \
  --body "Implement the webhook and tests." \
  --label billing \
  --dedupe-key billing-webhook
```

Submission creates a UUIDv7 Work Item with lifecycle `pending`, `current_phase = "plan"`, one pending
attempt, and an immutable intake snapshot at `.bach/artifacts/factory/<work-item-id>/intake.json`.
The Work Item is persisted through the configured Backend Provider using the `factory_queue` capability.
The submitted `--plan` value is stored as an opaque reference and is not parsed or used for file I/O in
this slice.

If `--dedupe-key` matches an existing pending item for the same factory and workflow, submit returns that
existing item. JSON output reports `"created": false` for this case.

Inspect and manage the queue:

```sh
bach factory list sldc
bach factory inspect sldc <work-item-id>
bach factory cancel sldc <work-item-id> --reason "no longer needed"
bach factory approve sldc <work-item-id> --phase plan
bach factory approve sldc <work-item-id> --phase deploy.production --reason "change approved"
bach factory status sldc
```

`bach factory approve` records durable approval evidence for a Work Item that is currently waiting at the
specified phase. The command accepts `--phase <phase>` and an optional `--reason <text>`. It returns the
existing approval idempotently when the same Work Item, attempt, and phase were already approved. Approval
phase strings use dot form such as `deploy.production`. The Backend Provider DTO schema is
`docs/schemas/backend-factory-approval-v1.schema.json`.

Start a long-running Factory daemon:

```sh
bach factory start sldc --yes
bach factory start sldc --yes --poll-interval 10s --renew-interval 1m --lease-ttl 2m
```

The daemon acquires a Backend lease, polls for pending Work Items, claims one item at a time, and executes
the workflow spine `plan -> implement -> merge -> deploy[*] -> verify[*]`. Empty queues do not stop the
daemon; use SIGINT or SIGTERM to stop it and release the lease. `--poll-interval` controls how often the
daemon checks the queue and defaults to `5s`. `--renew-interval` controls how often the daemon renews its
lease and defaults to `10s`. `--lease-ttl` controls how long a lease remains valid without renewal and
defaults to `30s`.

When a `plan` or `deploy` phase requires approval and no matching approval exists, the daemon sets the Work
Item lifecycle to `waiting_approval`, keeps `current_phase` at the gated phase, clears active daemon
ownership, and continues polling other eligible work. After an operator records approval with
`bach factory approve`, the daemon resumes the Work Item on a later poll or after a restart. Plan approval
evidence stores the Plan path and hash; if the Plan file changes after approval but before implementation
resumes, the Work Item fails with a stale-approval message instead of silently implementing different text.

When a Factory declares provider triggers, `bach factory start` also starts a long-running JSON-RPC session with each provider process. The daemon polls each provider on its configured interval, routes returned items to workflows using labels, and enqueues or updates pending Work Items. If any item in a polled batch fails intake validation, the entire batch is nacked so the provider can redeliver; successfully processed batches are acked and the trigger cursor is advanced. Provider trigger protocol messages conform to `docs/schemas/trigger-provider-v1.schema.json`. Provider intake failures do not fail Work Items that are already queued or active.

Use `--json` with any Factory command for machine-readable output. `factory submit` returns
`{"item": <work-item>, "created": true|false}`. `factory list` returns `{"items": [<work-item>, ...]}`.
`factory inspect` and `factory cancel` return a single Work Item object. `factory inspect` includes an
`approvals` array. `factory status` returns the active daemon lease, optional active Work Item ID, and
lifecycle counts. `factory start --json` returns the daemon ID and acquired lease record after the command
stops. CLI Work Item JSON omits raw body text; use the intake evidence URI for private submission details.
Failed Work Items include `failure_phase` and `failure_message`. The Backend Provider Work Item DTO schema is
`docs/schemas/backend-factory-work-item-v1.schema.json`.

`factory list` defaults to `pending` and `waiting_approval` items; pass `--status all` to include every
lifecycle. `--status` accepts `pending`, `claimed`, `running`, `waiting_approval`, `completed`, `failed`,
`cancelled`, or `all`. Pass `--workflow` to filter list output by workflow.

Current lifecycle values are:

- `pending`: queued for future planning/execution phases.
- `claimed`: claimed by a daemon lease before phase execution starts.
- `running`: executing the current workflow phase.
- `waiting_approval`: paused at a gated phase until an approval is recorded.
- `completed`: all configured workflow phases succeeded.
- `failed`: a workflow phase failed; the Work Item records the failed phase and message.
- `cancelled`: manually cancelled before execution.

Deferred Factory behavior includes retries, review queues, and replan loops.
