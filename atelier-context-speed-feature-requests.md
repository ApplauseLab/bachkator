# Bachkator Feature Requests For Atelier

Context: Atelier has a large Bun monorepo, remote staging deploy flows, Apple `container` image builds, local Postgres tests, and agents that need fast target discovery without rereading Makefiles/scripts. Bachkator is already useful as a named target graph, but the repo needs a few first-class features to make it materially better than the Makefile.

## Highest-Value Features

1. Computed variables

Support variables derived from commands or built-in Git/file hash functions. Atelier needs Makefile-style tags such as `$(git rev-parse --short HEAD)-dirty`, `deps-<hash>`, `runtime-<hash>`, and `source-$image_tag` without wrapper scripts or fragile shell interpolation.

2. Ordered pipelines

Aggregates can run dependencies concurrently, which is wrong for deploy flows. Add a sequential target type so `render -> apply -> rollout -> smoke` is encoded safely.

3. Environment profiles

Add first-class profiles such as `staging` and `staging-kristiyan` for namespace, public host, ACM cert, AWS profile, and OpenWorkflow namespace. This would avoid duplicating environment values across many targets.

4. Changed-file target suggestion

Add `bach affected` or `bach suggest <path>` to map dirty files to the smallest useful checks. Example: editing `packages/workspace-gateway/src/workspace/files.ts` should suggest `shell/typecheck-gateway`, `shell/test-gateway`, and relevant architecture/generated checks.

5. Target metadata and explain

Targets should expose when to use them, what they validate, expected cost, remote/destructive status, and related files. Add `bach explain <target>` and make `bach list` optionally show metadata.

6. Package graph plugin support

Add or standardize a Bun workspace/package dependency plugin that understands `@app/*` exports and can infer cross-package affected targets beyond direct TS imports.

7. Target-level quiet mode

`-log-only` is useful. Add target-level `quiet = true` for noisy tests/builds, with an override for verbose streaming.

8. Concurrency groups / locks

Atelier has shared resources that should not run concurrently: `.bach/state.db`, Postgres migrations/tests, image builds, and remote deploy/smoke operations. Add target locks such as `lock = "postgres"` or `lock = "container-builder"`.

9. Remote/destructive classifications

Targets that push images, apply Kubernetes manifests, restart deployments, or run live smoke tests should be marked `remote = true`, `destructive = true`, or `requires_confirmation = true` so agents can dry-run freely but execution is clearly risky.

10. Run outcome summaries

For `-log-only`, print a concise terminal summary with pass/fail, duration, log path, and a short failure excerpt. Avoid flooding the terminal with long build/test output.

## Priority Order

1. Computed variables.
2. Ordered pipelines.
3. `bach affected` / changed-file suggestions.
4. Environment profiles.
5. Concurrency locks.
6. Target metadata / explain.

These would make Bachkator a context and execution layer for agents, not just a structured wrapper around existing scripts.

## Additional Feature Requests

11. Target aliases and deprecation hints

Support aliases so teams can preserve old names while migrating from Makefile or scripts. Example: `alias "staging-kristiyan-deploy" = "pipeline/deploy-kristiyan"`. Add deprecation messages so agents learn the canonical target.

12. Required tool checks

Targets should declare required binaries and versions: `bun`, `container`, `kubectl`, `aws`, `gh`, `docker`, `opentofu`. Bach should fail early with a concise missing-tool report instead of failing deep inside a shell command.

13. Credential/session preflight targets

Add first-class preflight support for things like AWS SSO, ECR login, Kubernetes context, and GitHub auth. Bach could distinguish "credentials expired" from target failure and show the exact refresh command.

14. Kubernetes context guards

Remote targets should declare expected Kubernetes context and namespace. Bach should verify before `kubectl apply` or rollout, preventing accidental deploys to the wrong cluster/namespace.

15. Target output contracts

Allow targets to publish structured outputs, not just resources. Example: image build target returns `{ image_tag, app_image, worker_image }`, and render/deploy targets consume those values without recomputing tags.

16. Artifact and log indexing

Index run logs, generated manifests, image tags, and produced artifacts so agents can query prior runs: `bach runs --target image/all --since 24h --status failed` or `bach artifacts <run-id>`.

17. Failure pattern extraction

For failed targets, Bach should extract likely root cause lines using target-specific or repo-specific filters. This is more useful than dumping the last N lines for noisy Bun, Vite, Docker, and kubectl output.

18. Watch mode for target graph

Support `bach watch shell/test-gateway` or `bach watch affected`, rerunning only stale targets when relevant files change. This would speed local development and agent feedback loops.

19. Service readiness probes

Add reusable readiness primitives for TCP, HTTP, Kubernetes rollout, and command probes. This would replace hand-written wait loops for Postgres, API health, gateway health, and staging smoke preconditions.

20. Generated documentation from Bachfile

Generate a Markdown operations guide from the Bach graph, including targets, descriptions, profiles, remote/destructive flags, inputs, outputs, and common examples. This would keep AGENTS.md/README operational docs synced with the executable graph.

21. Target ownership and contacts

Allow targets to declare owning package/team/domain, for example `owner = "workspace-gateway"` or `domain = "deploy/staging"`. Agents can route questions and reviews to the right context and generated docs can group targets by domain.

22. Cost and duration estimates

Track historical runtime and resource cost per target. `bach list --cost` could show expected duration and whether a target is cheap, moderate, expensive, or remote. Agents can choose narrow checks first based on observed data.

23. Smart cache invalidation explanations

When a target is stale, Bach should explain why: changed input file, changed env var, changed command, dirty Git state, dependency fingerprint changed, missing output, or forced run. This helps agents trust the cache instead of rerunning everything.

24. Cache namespaces

Support separate cache namespaces for local dev, CI, staging deploys, and agent sessions. This prevents a deploy-specific env/profile from invalidating local test cache unexpectedly and makes concurrent agents safer.

25. Remote cache export/import

Allow selected target fingerprints and artifacts to be exported/imported from CI or shared storage. For large monorepos, agents could reuse CI-proven package test/build results locally when inputs match exactly.

26. Target graph visualization

Provide `bach graph --format dot|json|mermaid` so agents and humans can inspect dependencies, resources, locks, profiles, and remote/destructive flags. This would be especially useful before refactoring Makefile parity targets.

27. Transactional deploy checkpoints

For deploy pipelines, support checkpoints and rollback hooks. Example: render manifests, apply, rollout, smoke; if smoke fails, run a configured diagnostic or rollback target and record the failed checkpoint.

28. Secret redaction policy

Add configurable redaction for environment keys and output patterns in terminal output and logs. This matters because Bach now loads `.env` and deployment targets touch AWS/Kubernetes credentials.

29. Target parameter schemas

Variables should declare type, enum, default, description, and examples. Bach can validate `namespace`, `public_host`, `image_tag`, and profiles before running deploys, and agents can discover valid values programmatically.

30. Machine-readable plan output

Add `bach -dry-run -json <target>` to return the execution plan, cache state, commands, env diffs, resources, and risks. Agents could inspect and summarize the plan without scraping terminal text.

31. CI matrix generation

Generate GitHub Actions matrix entries from the Bach graph. CI could ask Bach which package tests, typechecks, image builds, and deploy checks exist instead of duplicating target lists in YAML.

32. PR check recommendation

Given a PR diff, Bach should output the minimum required CI checks plus optional broader checks. This is `bach affected` optimized for pull requests, including docs-only, package-only, and deploy-affecting changes.

33. Policy assertions

Add declarative assertions such as "image targets must use linux/amd64", "remote targets must have a profile", "deploy targets must be pipelines", or "destructive targets must not run without confirmation". Bach can validate the operations graph itself.

34. Workspace bootstrap target type

Add a first-class setup/bootstrap target that can install deps, verify tools, load env, start services, run migrations, and print next commands. This would make onboarding agents and humans faster than reading multiple docs.

35. Human approval checkpoints

Remote/destructive targets should support approval checkpoints that can be disabled in CI but required for local interactive runs. Example: before pushing images or applying manifests, print plan and require confirmation.

36. Target templates/macros

Many package targets follow the same pattern: typecheck, test, build, input plugin, workspace dependency. Add templates or macros to reduce Bachfile duplication and prevent target drift across packages.

37. Package autodiscovery

Scan Bun workspaces and automatically generate baseline `typecheck`, `test`, and `build` targets from package scripts. Hand-written targets can override or add metadata, but the common graph should not be manual.

38. Command sandbox modes

Targets should declare sandbox expectations: read-only, writes workspace, writes external system, network required, Kubernetes required, AWS required. Agents can use this to choose safe commands and request permission for risky ones.

39. Target-specific environment files

Allow targets or profiles to declare env overlays such as `.env.staging-kristiyan.live-e2e` without requiring users to pass `-env-file` manually. This would cleanly replace Makefile `source .env.$(ENV).live-e2e` recipes.

40. Structured smoke test protocol

Support smoke targets that emit structured health results for URLs, Kubernetes deployments, pods, and API endpoints. Bach can render a concise deploy health report rather than a raw script transcript.
