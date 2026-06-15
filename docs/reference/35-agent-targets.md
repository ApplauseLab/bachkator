## Agent Targets

Agent Targets run implementation and merge providers through generated prompt and context artifacts.
Implementer agents can attach reviewer policies that run `mode = "review"` agents and publish
quality-gated policy evidence. Merge agents consume that passing policy evidence before invoking a
serialized provider.

```hcl
provider "opencode" {
  type = "opencode"
}

provider "generic_opencode" {
  type    = "agent"
  command = ["opencode", "run"]
}

prompt "implementer" {
  path        = "prompts/agents/implementer.md"
  description = "Default implementation instructions"
  version     = "v1"
}

prompt "architecture_review" {
  path = "prompts/agents/architecture-review.md"
}

prompt "merge" {
  path = "prompts/agents/merge.md"
}

agent_template "feature_implementer" {
  mode     = "implement"
  provider = provider.opencode
  role     = "implementer"
  prompt   = prompt.implementer

  workspace {
    path = ".bach/agents/feature-implementer"
  }

  git {
    branch = "bach/agents/feature-implementer"
  }
}

agent "checkout_refactor" {
  template = agent_template.feature_implementer
  plan     = "plans/checkout-refactor.md"

  workspace {
    mode = "clone"
    path = ".bach/agents/checkout_refactor"
  }

  git {
    branch = "bach/agents/checkout_refactor"
    commit = "required"
  }
}

agent "architecture_review" {
  mode     = "review"
  provider = provider.generic_opencode
  role     = "architecture-reviewer"
  prompt   = prompt.architecture_review
}

agent "merge_checkout_refactor" {
  mode     = "merge"
  provider = provider.generic_opencode
  prompt   = prompt.merge
  subject  = agent.checkout_refactor
}

policy "merge_review" {
  reviewers = [agent.architecture_review]

  quality_gate {
    metric = "findings.error.open.count"
    max    = 0
  }
}
```

Provider blocks:

- `type`: `opencode` for the first-class OpenCode provider, or `agent` for a generic command provider.
- `command`: required only for generic `type = "agent"` providers. Bach expands environment variables and appends the generated prompt path as the final argument.

First-class OpenCode providers do not accept `command` and are supported for `mode = "implement"` Agent Targets. Bach owns the argv and invokes `opencode run --format json "Follow the attached Bach agent prompt." --file <generated-prompt>` so it can capture JSONL provider evidence and session IDs while ensuring OpenCode receives the generated Bach contract. On improvement attempts after attempt 1, Bach resumes the captured session with `opencode run --format json --session <sessionID> "Follow the attached Bach agent prompt." --file <generated-prompt>`. Review and merge agents that should run OpenCode directly can use a generic `type = "agent"` provider with `command = ["opencode", "run"]`.

OpenCode JSONL is provider evidence, not target success evidence. Malformed JSONL or a successful OpenCode process that emits no `sessionID` fails the provider attempt. Bach mirrors OpenCode assistant text events and tool names to the target log/stdout as readable progress while preserving the complete provider event stream, including tool inputs, outputs, titles, and descriptions, in raw JSONL. Normal progress output does not include raw tool arguments.
Raw OpenCode JSONL capture is capped at 10 MiB per attempt, individual JSONL events are capped at 1 MiB, and mirrored provider text/tool call summaries are capped at 1 MiB. Exceeding a cap fails the provider attempt after Bach drains the provider process output.

Prompt blocks:

- `path`: project-relative Markdown instructions file. Agent Targets that reference the prompt treat the file as an implicit input.
- `description`: optional passive metadata.
- `version`: optional passive metadata.

Prompt files provide reusable task guidance only. Bach always appends generated context and the required
structured report schema to the prompt passed to the provider. If an agent omits `prompt`, Bach uses a
baked-in default task prompt and still injects the same report schema.

Agent Template blocks:

- `agent_template "<name>"` declares reusable Agent Target defaults; it is not a Target, cannot run, and is hidden from `bach list`.
- `provider`: required typed reference such as `provider.opencode`.
- `mode`: optional `implement`, `review`, or `merge`; defaults to `implement`.
- `role`: optional report/log identity metadata.
- `prompt`: optional typed reference such as `prompt.implementer`.
- `policy`: optional typed reference supported only when `mode = "implement"`.
- `workspace` and `git` use the same block shape as Agent Targets.
- `plan` and `subject` are not valid on templates because concrete agents or future Factory Work Items provide concrete context.
- Template placeholders are valid only in `workspace.path` and `git.branch`: `${work_item.id}`, `${work_item.slug}`, `${plan.id}`, `${workstream.id}`, `${factory.name}`, and `${workflow.name}`.
- Concrete `agent` blocks can set `template = agent_template.<name>`; explicit fields and blocks on the agent override inherited template defaults.
- Runnable Agent Targets must be concrete. `bach validate` rejects an agent that still has unresolved template placeholders after inheritance.

Provider processes are untrusted with respect to Bach-owned policy evidence. During provider execution,
Bach fails the attempt if `.bach/artifacts/policies`, target cache state, policy-target run state, Plan ledger/evidence rows, or Factory approval records change outside Bach's own writes.
Implementer and reviewer providers require the main checkout to be clean before invocation and fail if
the provider changes main checkout HEAD, branch, Git metadata, ignored files, or non-`.bach` status.

Agent fields:

- `template`: optional typed reference such as `agent_template.feature_implementer`; inherited values are applied before validation.
- `mode`: `implement`, `review`, or `merge`. Managed workspaces and git evidence apply to `implement` agents.
- `provider`: required typed reference such as `provider.opencode`.
- `role`: optional report/log identity metadata.
- `prompt`: optional typed reference such as `prompt.implementer`.
- `plan`: required project-relative implementation plan path for `mode = "implement"`; not used by review or merge agents. The plan file is an implicit input.
- `subject`: required for `mode = "merge"`; must be an explicit `agent.<name>` reference to the implementer being merged.
- `policy`: optional typed reference such as `policy.merge_review`; supported on implementer agents.
- `workspace.mode`: defaults to `clone` and must be `clone` in v1.
- `workspace.path`: defaults to `.bach/agents/<agent-name>` and must stay under `.bach/agents`.
- `git.branch`: defaults to `bach/agents/<agent-name>`.
- `git.commit`: defaults to `required`; allowed values are `required` and `optional`.
- `improve.max_attempts`: optional number of policy-informed attempts for implementer agents.
- `improve.until`: currently supports `policy.passed`.

Agent Targets always execute when requested; prompt and plan inputs still participate in `bach affected`.
Bach creates or reuses the workspace clone for each run. New workspaces are cloned from the project root and
checked out on the configured branch. Existing workspaces must be clean git clones at run start, and the
project HEAD must already be an ancestor of the workspace HEAD; Bach fails the target instead of resetting or
cleaning a dirty or stale workspace.

Bach writes implementation attempt artifacts under `.bach/runs/<run-id>/<agent-target>/attempt-N/`:

- `agent-prompt.md`: provider prompt combining the optional prompt file, required plan, workspace path, context path, report path, and commit instructions.
- `agent-context.json`: structured metadata including target, mode, attempt, provider, prompt, plan, workspace, branch, report path, and context path.
- `agent-report.json`: provider completion report path.
- `provider-events.raw.jsonl`: raw OpenCode JSONL events when `type = "opencode"`.
- `provider-session.json`: OpenCode session evidence including session ID, workspace path, dirty status, raw event path, and executed argv when `type = "opencode"` and a session ID was observed.
- `provider-summary.json`: normalized OpenCode provider telemetry when `type = "opencode"`. `finish_reason`, `tokens`, and `cost` are present only when OpenCode emits them; `tokens` preserves provider-shaped token payloads.

Provider session and summary artifacts use the machine-readable contracts in `docs/schemas/provider-session.schema.json` and `docs/schemas/provider-summary.schema.json`.

Merge artifacts are written directly under `.bach/runs/<run-id>/<merge-agent-target>/` as
`merge-prompt.md`, `merge-context.json`, and `merge-report.json`.

Implementation and reviewer provider commands run in the managed workspace. Merge provider commands run in the main project checkout. Agent providers should report the provider base command, such as `["opencode", "run"]`; Bach-managed OpenCode flags such as `--format json`, `--session`, and the generated prompt path are captured in provider session evidence instead.

Bach also exposes convenience environment variables:

- `BACH_AGENT_PROMPT_PATH`
- `BACH_AGENT_CONTEXT_PATH`
- `BACH_AGENT_REPORT_PATH`
- `BACH_AGENT_WORKSPACE`
- `BACH_AGENT_TARGET`
- `BACH_AGENT_ATTEMPT`
- `BACH_AGENT_MAX_ATTEMPTS`
- `BACH_AGENT_FEEDBACK_BUNDLE`
- `BACH_AGENT_ATTEMPT_DIRECTORY`
- `BACH_AGENT_MODE`
- `BACH_AGENT_ROLE`
- `BACH_PROJECT_ROOT`

The prompt file remains the primary invocation contract; environment variables are convenience helpers.

Reviewer policies:

- `policy` blocks declare `reviewers`, a list of `agent.<name>` references whose targets must use `mode = "review"`.
- `quality_gate` blocks in a policy are enforced by the generated policy target for implementer agents that reference the policy.
- After implementation evidence passes, Bach invokes a generated policy target named `policy/<name>@agent.<subject>` so policy work has its own target run, log, quality report, and applied-policy artifact.
- Attached policy `required_targets` run in the subject workspace after implementation evidence passes and before reviewer fan-out.
- Required-target failures are converted into policy findings, and Bach validates the subject workspace commit, branch, and cleanliness before reviewers run.
- After an implementer creates valid git evidence, reviewers run in parallel inside the managed workspace.
- Reviewers receive `BACH_AGENT_MODE=review`, `BACH_AGENT_ROLE`, `BACH_AGENT_REPORT_PATH`, and `BACH_AGENT_SUBJECT_*` environment variables.
- Reviewer providers receive a generated reviewer prompt that combines the optional reviewer prompt file,
  subject metadata, and the required reviewer quality-evidence JSON schema.
- Bach aggregates reviewer findings into the generated policy target's `policy-report.json` using the `agent-report-json` quality format.
- Policy reports provide `findings.open.count` and `findings.error.open.count` metrics for quality gates.

Default reviewer prompt files are provided under `prompts/agents/architecture-review.md`,
`prompts/agents/docs-sweeper.md`, and `prompts/agents/security-review.md`. Docs-sweeper and security
reviewers should emit blocking findings when user-visible/agent-visible docs or security expectations are
not met.

Improvement loops:

- `improve { max_attempts = N, until = "policy.passed" }` starts another provider attempt after failed policy evidence.
- Failed attempts write `feedback-bundle.json` under the attempt directory with verdict, findings, failed gates, reviewer summaries, and evidence paths.
- OpenCode improvement attempts resume the previous captured session by default. Before resuming, Bach verifies the previous session evidence, target, workspace path, feedback bundle, and current workspace branch. Bach records whether the workspace is dirty before invoking OpenCode and never cleans or resets dirty workspace contents.
- `attempt-history.json` records each attempt, provider session ID, and provider artifact paths while the final policy verdict is based on the latest attempt.
- `retry` remains separate from `improve`: retry repeats the same failed target execution, while improve starts a new policy-informed agent attempt.

Merge agents:

- `mode = "merge"` targets default to `lock = "merge-lane"` unless a lock is set explicitly.
- Bach refuses to invoke the merge provider until the subject's latest matching applied policy artifact under `.bach/artifacts/policies/<run-id>/<sanitized-subject>.json` has a passing verdict, whose `subject_workspace` matches the merge subject workspace, whose `subject_commit` matches the subject workspace HEAD, and whose `policy_target` names the generated policy target that succeeded in the recorded run. For example, `agent/checkout_refactor` is written as `agent-checkout_refactor.json`.
- Before provider invocation, Bach verifies the subject workspace is on the configured subject branch and is clean.
- Merge providers run in the main project checkout, receive `BACH_AGENT_SUBJECT_TARGET`, `BACH_AGENT_SUBJECT_BRANCH`, `BACH_AGENT_SUBJECT_COMMIT`, `BACH_AGENT_SUBJECT_WORKSPACE`, and `BACH_AGENT_POLICY_EVIDENCE`, and get the generated merge prompt as the final argv.
- The generated merge context includes subject branch, commit, workspace, plan, provider metadata, and policy evidence.
- A successful merge report must include `pr_url`, `target_branch_commit`, or `merge_commit` evidence. PR URLs must be valid absolute URLs and must be paired with `target_branch_commit` or `merge_commit`; commit evidence must name a commit reachable from the main checkout and must have the reviewed subject commit as an ancestor.
- Merge providers receive a generated merge prompt that combines the optional merge prompt file, subject
  metadata, policy evidence, and the required merge completion report JSON schema.
- `bach runs inspect --json <run-id>` exports all target executions in `targets`, including log paths, artifact paths, quality report summaries, agent reports, applied policy summaries, provider metadata, subject metadata, and merge evidence for control-plane ingestion.

Providers must write an Agent Report JSON file to `BACH_AGENT_REPORT_PATH`. The minimal v1 envelope is:

```json
{
  "target": "agent/checkout_refactor",
  "provider_name": "opencode",
  "provider_type": "opencode",
  "provider_command": ["opencode", "run"],
  "mode": "implement",
  "status": "passed",
  "attempt": 1,
  "workspace": "/absolute/project/.bach/agents/checkout_refactor",
  "branch": "bach/agents/checkout_refactor",
  "commit": "abc123",
  "changed_files": ["src/checkout.go"],
  "summary": "Refactored checkout validation."
}
```

Valid statuses are `passed`, `failed`, `blocked`, and `partial`; only `passed` succeeds the Agent Target.
Bach validates the report target, provider evidence, mode, attempt, workspace, branch, commit, and summary.
Missing or malformed reports fail the Agent Target before later policy or reviewer phases. When
`git.commit = "required"`, the provider must create a new descendant commit in the configured branch during
the attempt or the target fails. The workspace must also be clean after provider execution.

Merge providers write a merge Agent Report JSON file to `BACH_AGENT_REPORT_PATH`:

```json
{
  "target": "agent/merge_checkout_refactor",
  "provider_name": "opencode",
  "provider_type": "agent",
  "provider_command": ["opencode", "run"],
  "mode": "merge",
  "status": "passed",
  "subject": {
    "target": "agent/checkout_refactor",
    "workspace": "/absolute/project/.bach/agents/checkout_refactor",
    "commit": "abc123"
  },
  "pr_url": "https://github.com/example/repo/pull/123",
  "summary": "Opened merge PR for checkout refactor."
}
```
