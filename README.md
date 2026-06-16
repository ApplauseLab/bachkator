# Bachkator

<div align="center">
  <img src="./assets/bachkator.png" alt="Bachkator logo" width="260" />

  <p><strong>Lights-off automation. Lights-on evidence.</strong></p>
  <p>The local-first dark factory and build system for agent-era repositories.</p>
  <p><strong>Бачкатор</strong> is Bulgarian slang for a hard worker: the one who gets the job done, even when nobody is watching.</p>
</div>

Bachkator turns a checked-in `Bachfile` into a repository's executable contract. Humans, CI, and coding agents use the same named Targets, dry-run plans, cache fingerprints, quality gates, logs, policy evidence, Plan ledgers, and Factory queues.

Use it as a **build system** when you need explicit, inspectable project operations. Use it as a **dark factory** when work should move through intake, planning, implementation, policy evaluation, merge, deploy, and verification without relying on a human or agent to rediscover the process from README fragments, CI YAML, package scripts, shell history, and vibes.

> **Dark factory** means lights off for the busywork, lights on for the evidence: unattended delivery queues where every phase is declared, gated, logged, and inspectable.

## ✨ Why Bachkator?

Modern agents can move fast. They also waste tokens, run the wrong command, skip preflight checks, lose logs, and accidentally treat deploy flows like parallel test jobs.

Bachkator gives humans, CI, and agents one operational contract:

- 🏭 **Run a dark factory** with durable Work Items, daemon leases, provider triggers, phase approvals, and plan-first delivery workflows.
- 📝 **Plan before implementation** with Markdown Plans, Plan hashes, dependency status, batch execution, and immutable Backend ledgers.
- 🤖 **Execute agent work safely** through Agent Targets, managed workspaces, generated prompts, required reports, reviewer policies, improvement loops, and merge evidence.
- 🧭 **Discover repo operations** with `bach list`, target aliases, embedded reference docs, and `bach explain <target>`.
- 🧪 **Dry-run before side effects** with `bach run --dry-run <target>` and machine-readable plan output.
- 🎯 **Choose focused checks** with `bach affected` and file provenance lookups.
- ⚡ **Skip fresh work** with input, output, environment, dependency, and operation fingerprints.
- 🧵 **Run safely in parallel** while preserving ordered Pipeline Targets for release and deploy lanes.
- 🚦 **Mark risk explicitly** with cost, remote, destructive, confirmation-required, tool, preflight, timeout, retry, and lock metadata.
- 📊 **Turn reports into gates** with normalized metrics, findings, quality plugins, Rego policies, and `bach quality` queries.
- 🧾 **Keep durable evidence** in Backend records, `.bach/runs/<run-id>/` logs, run artifacts, policy artifacts, and JSON exports.
- 🔌 **Extend by contract** with typed executable plugins, Backend Providers, and Trigger Providers instead of private in-process hooks.

## 🚀 Quickstart

Install the latest release:

```sh
curl -fsSL https://raw.githubusercontent.com/ApplauseLab/bachkator/main/install.sh | bash
```

Install somewhere else:

```sh
curl -fsSL https://raw.githubusercontent.com/ApplauseLab/bachkator/main/install.sh | \
  BACH_INSTALL_DIR="$HOME/bin" bash
```

Or build from source:

```sh
go run ./cmd/bach run shell/build
```

Then, inside a project with a `Bachfile`:

```sh
bach list
bach explain shell/test
bach run --dry-run shell/test
bach run shell/test
bach affected
bach runs list
bach reference
```

Contributor setup for this repository:

```sh
go run ./cmd/bach run --dry-run shell/install-git-hooks
go run ./cmd/bach run shell/install-git-hooks
```

Commits in this repository use semantic subject lines such as `feat(cli): add dry-run output` or `docs: update agent workflow`.

## 🧱 Build graph in one screen

A Bachfile declares project operations as typed Targets. Bachkator plans, fingerprints, runs, gates, records, and explains those Targets.

```hcl
# vim: set ft=hcl :

project "example" {
  root    = "."
  default = "group.gate"
}

input "file" "go_sources" {
  srcs = ["go.mod", "go.sum", "cmd", "internal"]
}

shell "test" {
  command = ["go", "test", "./..."]
  inputs  = [input.file.go_sources]
}

shell "lint" {
  command = ["golangci-lint", "run"]
  inputs  = [input.file.go_sources]
}

shell "build" {
  depends_on = [shell.test]
  command    = ["go", "build", "-o", "dist/app", "./cmd/app"]
  inputs     = [input.file.go_sources]
  outputs    = ["dist/app"]
}

group "gate" {
  targets = [shell.lint, shell.test, shell.build]
}
```

Run the graph through the same CLI contract an agent would use:

```sh
bach list
bach explain group/gate
bach run --dry-run group/gate
bach run --log-only --force group/gate
bach provenance dist/app
bach runs inspect <run-id>
```

## 🏭 Dark factory in one screen

A Factory is not a Target. It is a durable delivery queue and daemon policy layered on top of the same Bachfile contract.

```text
intake -> plan -> approval -> implement (+ policy evaluation) -> merge -> deploy -> verify
```

This Factory sketch assumes the referenced Agent Templates and Targets are declared elsewhere in the same Bachfile:

```hcl
factory "delivery" {
  workflow "ship" {
    plan {
      agent_template    = agent_template.planner
      path              = "plans/factory/${work_item.id}.md"
      requires_approval = true
    }

    implement {
      agent_template = agent_template.implementer
    }

    merge {
      target = "pipeline.merge_ready"
    }

    deploy "staging" {
      target = "shell.deploy_staging"
    }

    verify "staging" {
      target = "group.staging_gate"
    }

    deploy "production" {
      target            = "pipeline.deploy_production"
      requires_approval = true
    }

    verify "production" {
      target = "group.production_gate"
    }
  }

  triggers {
    manual {}

    provider "github_issues" {
      command       = ["bach-github-issue-trigger"]
      poll_interval = "5m"
      config = {
        repo      = "ApplauseLab/bachkator"
        token_env = "GITHUB_TOKEN"
        labels    = "factory:ship"
      }

      route {
        label    = "factory:ship"
        workflow = "ship"
      }
    }
  }
}
```

Submit work, run the daemon, and approve gated phases:

```sh
bach factory submit delivery --workflow ship --title "Ship billing webhook" --body "Implement webhook and tests"
bach factory list delivery
bach factory start delivery --yes
bach factory approve delivery <work-item-id> --phase plan
bach factory approve delivery <work-item-id> --phase deploy.production --reason "change approved"
bach factory inspect delivery <work-item-id>
```

The daemon claims one Work Item at a time and executes the workflow spine `plan -> implement -> merge -> deploy[*] -> verify[*]`. If a plan or deploy phase needs approval, Bachkator records the pause, releases the active claim, and resumes after approval without silently changing the approved Plan hash.

## 🔌 Extensible by contract

Bachkator is designed to grow without turning your repository into a pile of bespoke shell glue or requiring external systems to import Bach internals. Its extension points are executable contracts and versioned JSON schemas.

- **Graph plugins** are typed executables in any language. They run while loading the Project and can contribute dependency/input evidence before validation, fingerprinting, scheduling, and `bach affected` matching.
- **Quality plugins** are typed executables that parse project-specific report files into normalized metrics and findings after a target succeeds. Bachkator then owns quality gates, Rego policy evaluation, run evidence, and CLI inspection.
- **Backend Providers** speak `bach.backend.v1` and advertise capabilities such as `runs`, `evidence_refs`, `quality_reports`, `findings`, `factory_queue`, `plan_ledger`, and `approvals`. The bundled SQLite provider is local-first; other providers can implement the same state and evidence protocol.
- **Trigger Providers** speak `bach.trigger.v1` and feed normalized Work Items into Factory queues through handshake, poll, ack, and nack messages. GitHub Issues, Discord intake, ticket systems, scheduled maintenance, or private control planes can all become Factory intake without becoming Bach core.

The key rule is that Bach owns the execution semantics and evidence boundary. Extensions provide graph evidence, parsed quality evidence, durable Backend capabilities, or normalized intake; Bachkator still validates the Bachfile, plans the Run, enforces gates, records evidence, and drives Factory phase transitions.

Start with the contracts:

- [Plugin reference](docs/reference/28-plugins.md)
- [Backend Provider schema](docs/schemas/backend-provider-v1.schema.json)
- [Trigger Provider schema](docs/schemas/trigger-provider-v1.schema.json)

## 🧠 The mental model

Think Terraform, but for repository operations and unattended delivery:

| Terraform | Bachkator |
| --- | --- |
| configuration declares infrastructure | `Bachfile` declares project operations and factory lanes |
| plan before apply | dry-run before run, Plan before implementation |
| state tracks what exists | Backend tracks runs, fingerprints, reports, gates, Work Items, approvals, and Plan ledgers |
| providers expose capabilities | Targets, Inputs, Resources, Plugins, Quality Handlers, Providers, and Trigger Providers |
| apply changes intentionally | run named Targets and approve Factory phases intentionally |
| policy and review guard changes | quality gates, Rego policies, reviewer agents, policy fan-out, and merge evidence guard delivery |

Bachkator does not replace your language tooling, CI, or coding agents. It wraps them in a shared CLI Contract so every human, CI job, agent, and Factory daemon uses the same executable truth.

Inside this repository, that contract is mandatory for routine project work: use Bach targets instead of running tools such as `gofmt`, `go test`, `go build`, `golangci-lint`, Bats, docs generators, or release scripts directly. If a repeated operation is missing, add or update a Bach target first, then run that target.

## 🤖 Built for agent loops and unattended delivery

Agents and factory daemons should not guess. They should:

1. `bach list`
2. `bach explain <target>` when unfamiliar or risky
3. `bach run --dry-run <target>` before expensive or side-effecting work
4. `bach affected` after edits
5. `bach run <target>` for the smallest useful gate
6. `bach run --log-only --force group/gate` before handoff or commit so quality reports and gates execute instead of relying on cached status
7. `bach factory submit <factory>` when work should enter an unattended lane
8. `bach factory approve <factory> <item> --phase <phase>` for gated plan or deploy phases
9. `bach factory start <factory>` when a daemon should drain the queue
10. `bach runs list`, `bach runs inspect <run-id>`, and `.bach/runs/.../*.log` when something fails

See the [Agent Guide](docs/agent-guide.md) for target, Plan, and Factory operating guidance.

## 🧾 Evidence and safety

Bachkator is built for high-agency automation, but it treats side effects and external providers as evidence-bound operations.

- Dry-runs are read-only and show the Run Plan before execution.
- Target metadata carries operator guidance, cost, remote/destructive risk, and confirmation requirements.
- Pipeline Targets preserve order when deploy or release sequence matters.
- Locks coordinate shared local or remote resources within a run.
- Required tools and preflights fail early with operator-facing fix guidance.
- Cache fingerprints explain why work is fresh or stale.
- Quality blocks parse report files into normalized metrics and findings, then enforce gates.
- Rego policies evaluate normalized evidence with network and runtime-introspection builtins disabled.
- Agent Targets run providers in managed workspaces and require structured reports, git evidence, and clean workspace boundaries.
- Policy fan-out runs required targets and reviewer agents against the subject workspace before merge.
- Factory approvals are durable, phase-scoped, and tied to Plan evidence where relevant.
- Run exports intentionally link to local evidence paths instead of dumping every raw provider log into broad JSON output.

## 🧩 Examples

- 🐝 [Agent swarm delivery](examples/plan-agents/README.md): OpenCode feature agents, merge agents, locks, checkpoints, and regression gates.
- 📦 [Bun package graph plugin](examples/plugins/): plugin-provided dependency closures for `bach affected`.
- 📊 [Quality parser plugin](examples/plugins/quality-parser/README.md): project-local report parser emitting Bach metrics and findings.
- 🌍 [Terraform delivery](examples/terraform-delivery/): risk metadata, ordered delivery flows, and infrastructure-shaped Targets.

## ✅ Supported today

- HCL `Bachfile` configuration with `project`, `backend`, `var`, `env`, `profile`, `input`, `resource`, `plugin`, `provider`, `prompt`, `agent_template`, `alias`, `policy`, `shell`, `group`, `pipeline`, `image`, `agent`, `quality`, and `factory` blocks.
- Factory Work Items with manual and provider triggers, route rules, durable intake evidence, dedupe keys, lifecycle management, approvals, daemon leases, and machine-readable queue output.
- Factory workflows with plan, implement, optional merge, named deploy, and named verify phases.
- Markdown Plan status, implementation, review grouping, dependency waves, batch execution, Plan hashes, and Backend ledger records.
- Shell, Image, Group, Pipeline, and implementation/review/merge Agent Targets.
- Agent provider prompt/context/report artifacts, OpenCode provider evidence capture, clone workspaces, required commit/report enforcement, reviewer policies, improvement loops, applied-policy verdicts, and merge evidence.
- Subject-scoped policy fan-out with generated policy nodes, required target execution, reviewer aggregation, quality gates, and evaluation JSON.
- Named file Inputs, Resources, graph plugins, quality parser plugins, target aliases, target explanations, file provenance, and affected-target suggestions.
- Backend Provider and Trigger Provider protocols for replacing durable state/evidence storage or feeding external intake into Factory queues without expanding Bach core.
- Incremental cache fingerprints backed by the configured Backend Provider.
- Per-Run and per-Target logs, run inspection, concise log slicing, and JSON exports for failures, quality evidence, agent reports, policy evidence, and merge evidence.
- Environment profiles, `.env` and `--env-file` overlays, Git environment injection, computed variables, tool checks, preflights, completion contracts, retries, locks, timeouts, and target risk metadata.
- OCI image build command generation for Docker-compatible builders or Apple `container`.
- GitHub release Targets through `gh release create`.
- Embedded reference docs through `bach reference`.

Deferred Factory behavior includes retries, review queues, and replan loops.

## 📚 Docs

- 📖 [Reference](docs/reference.md): CLI flags, Bachfile syntax, Targets, Agent Targets, Plans, Factory Work Items, Inputs, Resources, quality reports, plugins, Backend configuration, logs, and Git environment.
- 🔌 [Plugin reference](docs/reference/28-plugins.md): graph and quality executable plugin contracts.
- 🧬 [Backend Provider schema](docs/schemas/backend-provider-v1.schema.json) and [Trigger Provider schema](docs/schemas/trigger-provider-v1.schema.json): versioned integration contracts for durable evidence and Factory intake.
- 🤖 [Agent Guide](docs/agent-guide.md): how agents discover, dry-run, execute, and inspect project operations.
- 🧭 [Product Context](CONTEXT.md): domain language and architecture direction.
- 🤝 [Contributing](CONTRIBUTING.md): development workflow, docs rules, and release checklist.
- ⚖️ [License](LICENSE): MIT License, copyright 2026 ApplauseLab.

The binary embeds the reference docs:

```sh
bach reference
bach reference factory-work-items
bach reference agent-targets
bach reference quality-reports
bach reference plans
```

## 🛡️ Safety notes

- Dry-run before remote, destructive, expensive, or unfamiliar Targets.
- Use `requires_confirmation = true` for guarded operations; real execution then requires `--yes`.
- Use Pipeline Targets when order matters; plain dependency graph edges may run in parallel.
- Put generated reports under `$(RUN_DIRECTORY)` so every Run keeps its own evidence.
- Model huge produced directories as Resources instead of hashing them as Outputs.
- Treat provider output as untrusted unless Bachkator has captured it as structured evidence and passed the configured policy.

## 🏁 Name

**Бачкатор** means the hard worker: the one who does the work, carries the team, and gets it done.

Bachkator is that worker for your repo: explicit Targets, unattended Factory lanes, inspectable Runs, durable evidence, fewer vibes.
