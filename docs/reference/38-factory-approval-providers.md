## Factory Approval Providers

Approval Providers let humans approve gated Factory phases in external systems
such as GitHub. The provider submits approval evidence into Bach; it never
decides workflow advancement directly. Bach validates every submitted record
against the waiting Work Item, attempt, phase, and workflow gates before
storing the same durable approval record produced by `bach factory approve`.

Configure approval providers inside a Factory:

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
  }

  approvals {
    provider "github" {
      command       = ["go", "run", "./approval-provider"]
      poll_interval = "1m"

      config = {
        repo         = "owner/repo"
        label_prefix = "approved/"
      }
    }
  }
}
```

Rules:

- A Factory may declare at most one `approvals` block with any number of named
  `provider` blocks.
- Provider names must be unique simple identifiers; `command` is required.
- `poll_interval` uses duration syntax, defaults to `5m`, and is clamped to at
  least `1s`.
- `config` passes string key/value pairs to the provider during handshake and
  poll.

Approval providers speak `bach.approval.v1` over stdio JSON-RPC with
`Content-Length` framing using `approval.handshake` and `approval.poll`. The
schema is `docs/schemas/approval-provider-v1.schema.json` and public Go types
live in `pkg/approvalprotocol`. Each poll returns records shaped as:

```json
{
  "source_type": "github_issue",
  "source_id": "42",
  "phase": "plan",
  "approver": "kris",
  "reason": "approved via issue label"
}
```

Addressing rules for each record:

- Match by intake source identity: set both `source_type` and `source_id` to
  match the Trigger Provider intake of the waiting Work Item, or
- match directly: set `work_item_id` to the Work Item UUID.
- Exactly one mode may be used per record; combining them is rejected.
- `phase` must be the canonical gated phase such as `plan` or
  `deploy.production`.

Recording semantics:

- Approval polling runs inside `bach factory start`; there is no standalone
  command.
- Records are applied only to Work Items currently in `waiting_approval` at
  the named phase whose workflow actually requires that approval. Everything
  else is skipped until the item waits.
- External approvals are idempotent and redelivery safe. Repeating an already
  recorded approval resolves to the existing record.
- Plan approvals capture the current Plan path and content hash exactly like
  CLI approvals, so editing the Plan after approval fails implementation with
  a stale-approval error.
- Recorded approvals store the approver reported by the provider with
  `approver_source = "provider"` plus optional reason and metadata.
