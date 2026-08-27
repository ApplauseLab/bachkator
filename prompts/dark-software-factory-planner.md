# Dark Software Factory Planner

You are planning a major Bachkator product slice: turn Bachkator into a local-first dark software factory that can watch external events, plan work, implement it with agents, merge it, and eventually deploy it from one inspectable HCL factory declaration.

Do not implement code in this pass. Produce a detailed implementation plan that another agent can execute in small, reviewable phases.

## Product Intent

The user wants Bachkator to support this Bachfile-level syntax:

```hcl
factory "sldc" {
  repo = "."

  triggers {
    discord {
      app_id     = "1234567890"
      app_secret = env("DISCORD_APP_SECRET")
      token      = env("DISCORD_TOKEN")
      channel    = "1234567890"
    }

    github {
      webhook_secret = env("GITHUB_WEBHOOK_SECRET")
    }
  }

  plan {
    prompt = file("prompt.txt")
  }

  implement {
    prompt = file("prompt.txt")
  }

  merge {
    prompt = file("prompt.txt")
  }

  deploy {
    # syntax still open
  }
}
```

Interpret "dark software factory" as an unattended local-first automation loop, not a cloud service. The factory should be inspectable, dry-runnable, evidence-producing, and governed by Bach targets, policies, quality gates, and state/evidence contracts.

## Repository Context To Preserve

Bachkator's current domain language and boundaries are important. Use the current terms from `CONTEXT.md` and do not invent a parallel execution universe.

Relevant current facts:

- `Project` means one repository workspace described by one Bachfile and executed from a resolved project root.
- `Bachfile` is the public HCL configuration surface.
- The supported product boundary is the CLI contract: commands, flags, Bachfile syntax, reference docs, JSON outputs, and terminal behavior.
- A `Target` is a named operation that Bach can inspect, plan, run, cache, and record.
- Existing target kinds include `shell`, `agent`, `image`, `pipeline`, `group`, and generated `policy` targets.
- `Agent Target` already owns provider execution, workspace clone behavior, git commit requirements, improvement attempts, policy attachment, and structured agent reports.
- `Policy` aggregates required targets, reviewers, quality gates, and findings thresholds.
- `State Store` is private local persistence. Do not expose its schema as a public integration point.
- `Evidence Store` protects Bach-owned evidence paths and provider/workspace boundaries.
- The managed control plane boundary already exists conceptually: external products may consume CLI/JSON evidence, but Bach remains the local OSS execution engine.

Relevant files and seams to inspect before writing the plan:

- `CONTEXT.md` for product vocabulary and package direction.
- `AGENTS.md` and child `AGENTS.md` files for DOX rules.
- `docs/designs/plan-first-agent-workflows.md` for existing plan-first/overnight workflow direction.
- `docs/adr/0019-agent-targets-and-managed-control-plane.md` for agent/control-plane boundary.
- `docs/adr/0001-unified-target-model.md` for target model constraints.
- `docs/adr/0006-cli-contract-over-public-go-api.md` for public boundary constraints.
- `internal/config/config_types.go`, `load.go`, `config_eval.go`, `validation.go`, `target_spec.go`, and `runtime_project.go` for Bachfile loading and model conversion.
- `internal/model/types.go` and `target_address.go` for shared domain shapes and target identity.
- `internal/target/agent.go` and adjacent files for target-kind handler behavior.
- `internal/runner/` for run planning, scheduling, state recording, logs, target execution, and policy fan-out.
- `internal/evidence/` for safe local artifact/workspace boundaries.
- `internal/state/` for private persistence and migrations.
- `internal/cli/` and `internal/app/` for CLI adapters and composition wiring.
- `docs/reference/*.md`, `docs/agents.md`, and `examples/opencode-provider/Bachfile` for current public syntax examples.

## External Product Context

The factory should eventually integrate with Atelier, and its design should learn from Fabro, Sandcastle, and fspec without copying their architectures wholesale.

Treat `docs/designs/plan-first-agent-workflows.md` as the product sequencing source:

- Implement plan-first agent workflows first: plan frontmatter, workstreams, reusable agent templates, generated targets, ledgers, evidence, and review queues.
- Add first-class factory syntax after plan-first execution exists. The factory attaches triggers to plan-first execution; it does not replace the plan model.
- Start factory triggers with manual intake and GitHub Issues before adding Discord. Browser automation may submit manual items later, but should not be a factory-domain trigger name.
- Add an Atelier state/evidence provider after local factory behavior exists. Bach must expose supported evidence/artifact exports rather than letting Atelier read private State Store tables.
- Add the Atelier bootstrap integration last, through Atelier Use-cases and OpenWorkflow orchestration.

Inspect these reference repositories before finalizing the plan:

- Atelier: `/Users/kris/GitHub/applause/atelier`
- Fabro: `/Users/kris/.local/share/opencode/repos/github.com/fabro-sh/fabro`
- Sandcastle: `/Users/kris/.local/share/opencode/repos/github.com/mattpocock/sandcastle`
- fspec: `/Users/kris/.local/share/opencode/repos/github.com/sengac/fspec`

Atelier integration constraints:

- Atelier is a managed OpenCode platform and the likely future managed-control-plane consumer for Bach factory evidence.
- Atelier state changes go through `Use-case -> commit() -> Event + Projection`; routes and workers must not bypass Use-cases.
- Atelier durable orchestration uses OpenWorkflow. Use-cases schedule workflows with `ctx.workflowClient.runWorkflow(...)`; routes and workflow activities do not directly own domain decisions.
- Atelier Projects group repositories, workspace context, and collaboration views. Factory work should map naturally to Project-scoped Tasks, ProjectConnections, WorkspaceSessionBindings, ActivityEvents, and AgentPacks where relevant.
- Atelier ProjectConnections store non-secret config and encrypted credentials for external systems. Bach factory credential design should be compatible with this pattern without requiring Atelier to read Bach's private State Store.
- Atelier should consume Bach factory evidence through CLI/JSON/events/artifacts, not by importing Bach internals or reading private `.bach/state.db` tables.
- The factory should eventually support an Atelier-managed runtime where credentials, prompt packs, policies, workspace provisioning, audit, and organization governance live in Atelier while Bach remains the repo-local execution and evidence engine.

Fabro design references to evaluate:

- Fabro explicitly positions itself as an open-source dark software factory.
- Fabro workflows are directed graphs that can include loops, human gates, commands, agent stages, parallel fan-out/fan-in, goal gates, retries, and conditionals.
- Fabro stores durable run event envelopes, emits SSE, supports exported JSONL, and uses checkpoint branches for work product plus metadata branches for structured run evidence.
- Fabro's GitHub App model separates public config from vault secrets and supports webhook delivery strategies. Its Slack Socket Mode integration is a useful contrast for Discord: outbound socket-style trigger delivery may avoid requiring a public webhook URL.
- Do not simply clone Fabro's Graphviz workflow model. Bach's factory must stay aligned with Bachfile syntax, Targets, Policies, Run evidence, and local-first Bach operations.

Sandcastle design references to evaluate:

- Sandcastle cleanly separates sandbox providers, agent providers, prompts, branch strategies, worktrees, issue trackers, completion signals, structured output, and run logs.
- Sandcastle's `Task` is selected from an issue tracker and worked through one agent iteration that produces at most one commit. Compare that with Bach factory work items and Agent Targets.
- Sandcastle's prompt template distinction is relevant to `file("prompt.txt")`: file-backed prompts may include template arguments or expansion, but prompt expansion can create security and reproducibility risks.
- Sandcastle treats multi-repo sandbox support as a major future refactor because single-repo assumptions affect lifecycle, sync, commits, cleanup, and result types. Bach factory `repo = "."` should avoid accidentally promising multi-repo semantics in the first slice.

fspec design references to evaluate:

- fspec provides acceptance-criteria-driven development with Gherkin scenarios, Example Mapping, work units, workflow states, checkpoints, hooks, quality gates, coverage links, and traceability from requirements through tests to implementation.
- Bach factory planning should decide whether a factory `plan` phase emits durable plan Markdown only, fspec-style acceptance criteria, or a neutral evidence format that Atelier can later project into Tasks/specs.
- Use fspec as inspiration for traceability and quality gates, not as a required dependency.

Use Bach operations for repo work. Start with:

```sh
go run ./cmd/bach list
```

Dry-run expensive or side-effecting targets before recommending them:

```sh
go run ./cmd/bach --dry-run run shell/test
go run ./cmd/bach --dry-run run shell/e2e
```

## Planning Questions To Answer

Your plan must answer these questions explicitly:

1. Is `factory` a new top-level Bachfile block, a new target kind, or a higher-level orchestration block that materializes existing targets? Choose one primary model and explain why.
2. How does `factory "sldc"` relate to the existing `project` block? Can a Bachfile contain multiple factories? Can a factory point at a repo different from `project.root`?
3. What is the exact public HCL shape for `factory`, `triggers`, `discord`, `github`, `plan`, `implement`, `merge`, and first deploy placeholder behavior?
4. Should `plan.prompt = file("prompt.txt")` store prompt contents or a path/reference? Existing `prompt` blocks use project-relative paths; reconcile that with the proposed `file()` syntax.
5. Does Bach already support `env()` and `file()` functions in HCL? If not, should the feature add them, or should it use existing expression patterns? Include security implications for secret values.
6. Where do Discord app secrets, bot tokens, GitHub webhook secrets, and future deploy credentials live? Define redaction, validation, logging, and state persistence rules.
7. What local process receives first-slice manual intake and GitHub Issue triggers? Is it a new CLI command such as `bach factory submit`, `bach factory intake`, `bach factory start`, a generated pipeline, or a long-running runner mode?
8. What events create factory work items? Define minimal event schemas for manual submissions and GitHub Issues first; leave Discord as a later trigger once the queue and credential model are proven.
9. Where are queued work items persisted? Decide whether this belongs in the private State Store, `.bach` evidence files, or a new local queue file under Bach ownership.
10. How does a work item flow through `plan`, `implement`, `merge`, and `deploy`? Map each phase to existing Agent Targets, policies, pipelines, generated targets, or new domain concepts.
11. How does the factory use the existing plan-first workflow? Decide whether the `plan` phase writes plan Markdown/frontmatter, generates transient targets, or both.
12. What evidence is produced at each phase? Include run IDs, prompts, provider sessions, agent reports, policy reports, commits, merge evidence, trigger event IDs, and deploy evidence.
13. What is dry-run behavior for a factory? A user must be able to inspect triggers, derived targets, prompt paths, risk, credentials present/missing, and planned actions without contacting external services.
14. What is list/explain/validate behavior? Decide how factory configuration appears in `bach list`, `bach explain`, `bach validate`, and JSON output.
15. What are concurrency rules? Include one factory loop per repo, trigger deduplication, work item locks, merge serialization, and safe shutdown/resume.
16. What are failure modes and retry/improvement semantics? Distinguish provider retry, agent improvement, trigger delivery retry, and merge/deploy retry.
17. What is the minimum deploy slice? If deploy syntax is still open, define a placeholder that validates cleanly but does not pretend to support production deployment yet.
18. Which docs, examples, tests, and AGENTS/DOX updates are required for each implementation phase?
19. What evidence and API surface should Atelier consume when it integrates factory runs? Define the boundary so Atelier does not read Bach private State Store tables.
20. How would Atelier represent factory-triggered work as Events, Projections, Tasks, ActivityEvents, ProjectConnections, WorkspaceSessionBindings, and workflows?
21. Which Fabro, Sandcastle, and fspec concepts should Bach deliberately adopt, adapt, or reject?

## Architecture Constraints

Follow these constraints in the plan:

- Preserve the unified Target model. Do not create a separate factory execution engine that bypasses target planning, logs, evidence, quality, policies, or State Store records.
- Keep public behavior in the CLI contract. Do not expose a public Go API for factories.
- Keep config loading side-effect free. Loading `factory` syntax must not probe hosts, contact Discord/GitHub, open listeners, mutate state, or execute tools.
- Keep secrets out of prompts, logs, JSON evidence, State Store rows, generated docs, and dry-run output. Report whether a secret is configured, not its value.
- Keep provider-specific HTTP/webhook handling behind clear internal seams. Avoid direct Discord/GitHub logic spread through config, runner, and CLI packages.
- Prefer materializing factory phases into existing targets or generated target specs where possible.
- Do not add backward-compatibility shims unless there is shipped behavior to preserve.
- Favor minimal first slices that can be validated and documented before adding full unattended runtime behavior.

## Expected Plan Structure

Produce a Markdown plan with these sections:

1. `Decision Summary`: the chosen model for `factory`, unresolved decisions, and sharp tradeoffs.
2. `Current System Map`: concise map of existing packages and docs that this work touches.
3. `Factory Domain Model`: proposed terms and relationships, using existing Bach vocabulary where possible.
4. `Comparative References`: what Atelier, Fabro, Sandcastle, and fspec imply for the design.
5. `Atelier Integration Path`: how Atelier can consume factory evidence later without owning Bach internals.
6. `Public Syntax`: proposed HCL syntax with a minimal complete example and validation rules.
7. `CLI Contract`: proposed commands, flags, dry-run behavior, JSON output, and examples.
8. `Execution Flow`: trigger ingestion through plan, implement, merge, deploy, evidence, and resume.
9. `Persistence And Evidence`: exact local artifacts, redaction rules, State Store responsibilities, and external evidence export contracts.
10. `Security Model`: threat model for webhooks, Discord commands, GitHub payloads, prompt injection, secret handling, workspace isolation, merge/deploy authority, and audit evidence.
11. `Implementation Phases`: small vertical slices with one clear owner area per phase where possible.
12. `Tests And Verification`: unit, e2e, docs, security, and dry-run test coverage per phase.
13. `Documentation And DOX`: reference fragments, examples, ADR/design updates, and AGENTS.md updates.
14. `Open Questions`: only real decisions that block implementation, not vague unknowns.

## Suggested Phase Boundaries

Use or refine these boundaries. Keep each phase independently reviewable.

1. Config-only factory schema: parse and validate `factory` blocks, triggers, phase prompt declarations, and deploy placeholder without runtime behavior.
2. Model and reference surface: add internal model structs, runtime conversion, validation JSON, reference docs, and a safe example Bachfile.
3. Factory explain/dry-run: expose inspectable factory plans through CLI without contacting external services.
4. Local work-item queue and event normalization: persist synthetic factory events locally and run them through a no-network command for tests.
5. Phase materialization: turn a work item into plan/implement/merge generated targets or pipelines using existing Agent Target behavior and policies.
6. Manual intake and GitHub Issues trigger adapters: normalize trigger events, dedupe them, and create factory work items without requiring Discord yet.
7. Atelier state/evidence provider: export ledgers, artifacts, review queues, and run evidence to an Atelier-owned backend/provider without exposing private State Store tables.
8. Factory serve loop: long-running local process with locks, bounded concurrency, graceful shutdown, resume, run/evidence linkage, and safe defaults.
9. Discord and webhook trigger expansion: add authenticated Discord/GitHub webhook adapters with redacted config, dedupe, tests, and no secret leakage after local/GitHub Issue intake works.
10. Atelier bootstrap integration: provision and configure factories from Atelier Projects through Atelier Use-cases and OpenWorkflow.
11. Deploy placeholder evolution: decide and implement the first real deploy contract only after the prior evidence path is stable.

## Deliverable Requirements

The final plan must be actionable enough that implementation agents can take one phase at a time without rediscovering the architecture.

Include:

- File-level change map for each phase.
- New or changed public commands and syntax.
- Validation diagnostics and examples.
- Test files to add or update.
- Bach targets to run for verification.
- Documentation files to update.
- Risks and mitigation for each phase.
- A future Atelier integration map that names the Bach evidence exports, Atelier domain concepts, and trust boundaries involved.

Do not include:

- Secret values.
- Unsupported syntax presented as currently shipped behavior.
- A cloud-control-plane design that requires Bach users to depend on a managed service.
- A separate job runner that bypasses Bach targets and run evidence.
