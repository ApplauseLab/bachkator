# Bachkator

<div align="center">
  <img src="./assets/bachkator.png" alt="Bachkator logo" width="260" />

  <p><strong>Terraform for your builds and agent loops.</strong></p>
  <p><strong>Бачкатор</strong> is Bulgarian slang for a hard worker: the one who gets the job done.</p>
</div>

Bachkator is a build-system control plane for repositories where humans and coding agents need the same explicit, inspectable project operations.

Instead of asking every agent to rediscover commands from README fragments, CI YAML, package scripts, shell history, and vibes, you declare a **Bachfile**. Bachkator turns that Bachfile into a Dependency Graph with dry-runs, cache evidence, logs, risk metadata, quality gates, and embedded reference docs.

## ✨ Why Bachkator?

Modern agents can move fast. They also waste tokens, run the wrong command, skip preflight checks, lose logs, and accidentally treat deploy flows like parallel test jobs.

Bachkator gives them an operational contract:

- 🧭 **Discover** supported Targets with `bach list`.
- 🔎 **Explain** a Target before running it with `bach explain <target>`.
- 🧪 **Dry-run** the Run Plan with `bach --dry-run run <target>`.
- 🎯 **Suggest affected Targets** from changed files with `bach affected`.
- ⚡ **Skip fresh work** with input/output/dependency fingerprints.
- 🧵 **Run safely in parallel** while preserving ordered Pipeline Targets.
- 🧰 **Check required tools and preflights** before execution.
- 🚦 **Mark risk** with remote, destructive, and confirmation-required metadata.
- 📊 **Parse quality reports** and enforce quality gates through `bach quality`.
- 🧾 **Keep durable logs** under `.bach/runs/<run-id>/`.
- 📚 **Ship docs in the binary** with `bach reference`.

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
go build -o bach ./cmd/bach
```

Then, inside a project with a `Bachfile`:

```sh
bach list
bach explain shell/test
bach affected
bach --dry-run run shell/lint
bach run shell/lint
bach --dry-run run shell/test
bach -j 8 run shell/test
bach quality summary
bach runs
```

## 🧱 Bachfile in one screen

```hcl
# vim: set ft=hcl :

project "example" {
  root    = "."
  default = "shell/test"
  state   = ".bach/state.db"
}

input "file" "go_sources" {
  srcs = ["go.mod", "go.sum", "cmd", "internal"]
}

shell "test" {
  command = ["sh", "-c", "go test -coverprofile=$(RUN_DIRECTORY)/coverage.out ./..."]
  inputs  = [input.file.go_sources]
  outputs = {
    coverage = "$(RUN_DIRECTORY)/coverage.out"
  }
}

shell "lint" {
  command = [
    "golangci-lint",
    "run",
    "--issues-exit-code=0",
    "--output.checkstyle.path=$(RUN_DIRECTORY)/checkstyle.xml",
    "--output.text.path=stdout",
  ]
  tools = [{
    name    = "golangci-lint"
    command = ["sh", "-c", "golangci-lint version | grep -q 'version 2\\.'"]
    version = "v2"
  }]
  inputs = [input.file.go_sources]
  outputs = {
    checkstyle = "$(RUN_DIRECTORY)/checkstyle.xml"
  }
}

quality "test" {
  cov {
    format = "go-cover"
    path   = shell.test.outputs.coverage
  }

  quality_gate {
    metric = "coverage.line.percent"
    min    = 50
  }
}

quality "lint" {
  lint {
    format = "checkstyle-xml"
    path   = shell.lint.outputs.checkstyle
  }

  quality_gate {
    metric = "issues.total.count"
    max    = 0
  }
}

shell "build" {
  depends_on = ["shell/test"]
  command    = ["go", "build", "-o", "dist/app", "./cmd/app"]
  inputs     = [input.file.go_sources]
  outputs    = ["dist/app"]
}

pipeline "ci" {
  steps = ["shell/lint", "shell/test", "shell/build"]
}
```

## 🧠 The mental model

Think Terraform, but for repository operations:

| Terraform | Bachkator |
| --- | --- |
| configuration declares infrastructure | `Bachfile` declares project operations |
| plan before apply | dry-run before run |
| state tracks what exists | State Store tracks runs, fingerprints, reports, gates |
| providers expose capabilities | Targets, Inputs, Resources, Plugins, Quality Handlers |
| apply changes intentionally | run named Targets intentionally |

Bachkator does not replace your language tooling. It wraps it in a shared CLI Contract so every human, CI job, and agent uses the same Targets.

## 🧬 Claude dynamic workflows vs Bachkator

Bachkator works with any agent because it is just a CLI plus files in your repo. Use it from Claude Code, OpenCode, Codex, Cursor, CI, a shell script, or a human terminal.

[Claude Code dynamic workflows](https://code.claude.com/docs/en/workflows) are powerful: Claude writes a JavaScript script, the workflow runtime runs it in the background, and that script can orchestrate many subagents for audits, migrations, and cross-checked research. Bachkator complements that model by giving every workflow a durable repository contract to call.

| Need | Claude dynamic workflows | Bachkator |
| --- | --- | --- |
| Multi-agent orchestration | Excellent for Claude-written scripts that fan out subagents | Provides stable Targets those agents can invoke |
| Project operation contract | Saved workflows can live under `.claude/workflows/` | Explicit `Bachfile` checked into the repo |
| Plan before side effects | Workflow launch shows planned phases for approval | Built in for every Target with `bach --dry-run run <target>` |
| Cross-agent portability | Claude Code feature model | Works from Claude Code, OpenCode, Codex, Cursor, CI, and humans |
| Run evidence | Workflow progress/results live in Claude's workflow UI/session | Durable logs and State Store under `.bach/` |
| Cache and affected checks | Workflow script decides what to re-run | Inputs, Outputs, Resources, fingerprints, and `bach affected` |
| Risk controls | Claude permissions and workflow approval | Target metadata plus `requires_confirmation` / `-yes` |
| Quality gates | Workflow can ask agents to review or run tools | Parsed reports, metrics, findings, and gates via `bach quality` |
| Long-running loops | Great for background Claude subagent fan-out | Great as the stable loop contract, checkpoint target, and gate runner |

Use dynamic workflows for Claude-native reasoning, fan-out, and UI. Use Bachkator for the repository's executable truth.

## 🤖 Built for agent loops

Agents should not guess. They should:

1. `bach list`
2. `bach explain <target>` when unfamiliar or risky
3. `bach --dry-run run <target>` before expensive or side-effecting work
4. `bach affected` after edits
5. `bach run <target>` for the smallest useful gate
6. `bach runs` and `.bach/runs/.../*.log` when something fails

See the [Agent Guide](docs/agents.md) for the full workflow.

## 🧪 Quality gates as first-class Targets

Bachkator lets Targets publish report files and lets `quality` blocks parse and gate them. This repo dogfoods that with:

- `shell/test` → Go coverage profile → `coverage.line.percent` gate.
- `shell/lint` → golangci-lint Checkstyle XML → `issues.total.count == 0` gate.
- `.golangci.yml` → `golines` at 100 columns plus `dupl` duplication checks.

Because the linter exits zero, Bachkator owns the quality decision and records findings/gates in the State Store.

```sh
bach run shell/lint
bach quality findings
bach quality gates
```

## 🧩 Examples

- 🐝 [Agent swarm delivery](examples/plan-agents/README.md): OpenCode feature agents, merge agents, locks, checkpoints, and regression gates.
- 📦 [Bun package graph plugin](examples/plugins/): plugin-provided dependency closures for `bach affected`.
- 🌍 [Terraform delivery](examples/terraform-delivery/): risk metadata, ordered delivery flows, and infrastructure-shaped Targets.

## ✅ Supported today

- HCL `Bachfile` configuration.
- `project`, `var`, `env`, `profile`, `input`, `resource`, `plugin`, `alias`, `shell`, `pipeline`, `image`, and `quality` blocks.
- Named file Inputs with glob and directory hashing.
- Resources for logical dependency evidence without hashing large directories.
- Shell, Image, and Pipeline Targets.
- Parallel scheduling with deterministic single-thread order.
- Pipeline Targets for ordered deploy/release flows.
- Incremental cache fingerprints backed by SQLite.
- Per-Run and per-Target logs.
- Git environment injection for reproducible builds and releases.
- Computed variable defaults for Git SHAs, dirty suffixes, and file hashes.
- Environment profiles and `.env` / `-env-file` overlays.
- Target metadata for description, operator guidance, cost, risk, and confirmation guards.
- Required tool and preflight checks.
- Completion contracts with `success_when` and `fail_when`.
- Embedded reference docs.
- `bach explain` Target inspection.
- `bach affected` suggestions from changed files and plugin-provided Inputs.
- OCI image build command generation for Docker or Apple `container`.
- Quality report parsing, normalized metrics/findings, and quality gates.
- GitHub release Targets through `gh release create`.

## 📚 Docs

- 📖 [Reference](docs/reference.md): CLI flags, Bachfile syntax, Targets, Inputs, Resources, quality reports, plugins, state, logs, and Git environment.
- 🤖 [Agent Guide](docs/agents.md): how agents discover, dry-run, execute, and inspect project operations.
- 🧭 [Product Context](CONTEXT.md): domain language and architecture direction.
- 🤝 [Contributing](CONTRIBUTING.md): development workflow, docs rules, and release checklist.
- ⚖️ [License](LICENSE): MIT License, copyright 2026 ApplauseLab.

The binary embeds the reference docs:

```sh
bach reference
bach reference project
bach reference shell-targets
bach reference quality-reports
```

## 🛡️ Safety notes

- Dry-run before remote or destructive Targets.
- Use `requires_confirmation = true` for guarded operations; real execution then requires `-yes`.
- Use Pipeline Targets when order matters; plain dependency graph edges may run in parallel.
- Put generated reports under `$(RUN_DIRECTORY)` so every Run keeps its own evidence.
- Model huge produced directories as Resources instead of hashing them as Outputs.

## 🏁 Name

**Бачкатор** means the hard worker — the one who does the work, carries the team, and gets it done.

Bachkator is that worker for your repo: explicit Targets, inspectable Runs, durable evidence, fewer vibes.
