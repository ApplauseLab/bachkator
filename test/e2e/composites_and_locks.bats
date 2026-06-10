#!/usr/bin/env bats

load helpers

setup() {
  setup_bach
  make_project
}

@test "pipelines run sequential steps while groups run member DAGs" {
  write_bachfile <<'HCL'
project "e2e" {
  root    = "."
  default = "pipeline.deploy"
  state   = ".bach/state.db"
}

shell "render" {
  shell = "printf 'render\n' >> order.txt"
}

shell "apply" {
  shell = "test \"$(cat order.txt)\" = \"render\" && printf 'apply\n' >> order.txt"
}

shell "lint" {
  shell = "printf 'lint\n' >> group.txt"
}

shell "test" {
  shell = "printf 'test\n' >> group.txt"
}

group "ci" {
  targets = [shell.lint, shell.test]
}

pipeline "deploy" {
  steps = [group.ci, shell.render, shell.apply]
}
HCL

  run bach --jobs 2 run pipeline/deploy
  assert_success
  assert_output_contains "[group/ci] group: shell/lint, shell/test"
  assert_output_contains "[pipeline/deploy] pipeline: group/ci -> shell/render -> shell/apply"
  [[ "$(<"$E2E_PROJECT/order.txt")" = $'render\napply\n' ]]
  assert_file_contains "$E2E_PROJECT/group.txt" "lint"
  assert_file_contains "$E2E_PROJECT/group.txt" "test"
}

@test "same named locks serialize otherwise parallel targets" {
  write_bachfile <<'HCL'
project "e2e" {
  root    = "."
  default = "group.db"
  state   = ".bach/state.db"
}

shell "a" {
  lock  = "postgres"
  shell = "test ! -f postgres.lock; touch postgres.lock; sleep 0.1; rm postgres.lock; printf 'a\n' >> done.txt"
}

shell "b" {
  lock  = "postgres"
  shell = "test ! -f postgres.lock; touch postgres.lock; sleep 0.1; rm postgres.lock; printf 'b\n' >> done.txt"
}

group "db" {
  targets = [shell.a, shell.b]
}
HCL

  run bach --jobs 2 run group/db
  assert_success
  assert_file_contains "$E2E_PROJECT/done.txt" "a"
  assert_file_contains "$E2E_PROJECT/done.txt" "b"
}

@test "pipeline stops after a failed step" {
  write_bachfile <<'HCL'
project "e2e" {
  root  = "."
  state = ".bach/state.db"
}

shell "render" {
  shell = "printf render >> order.txt"
}

shell "apply" {
  shell = "printf apply >> order.txt; exit 1"
}

shell "smoke" {
  shell = "printf smoke >> order.txt"
}

pipeline "deploy" {
  steps = [shell.render, shell.apply, shell.smoke]
}
HCL

  run bach --jobs 2 run pipeline/deploy
  assert_failure
  [[ "$(<"$E2E_PROJECT/order.txt")" = "renderapply" ]]
}

@test "grouped pipelines can run together with cross-group dependencies inside a pipeline" {
  write_bachfile <<'HCL'
project "e2e" {
  root    = "."
  default = "group.release"
  state   = ".bach/state.db"
}

shell "api-schema" {
  shell = "sleep 0.1; printf 'api-schema\n' >> events.txt; touch api-schema.done"
}

shell "api-compile" {
  shell = "printf 'api-compile\n' >> events.txt; touch api-compile.done"
}

group "api-build" {
  targets = [shell.api-schema, shell.api-compile]
}

shell "api-package" {
  shell = "test -f api-schema.done && test -f api-compile.done && touch api-package.done && printf 'api-package\n' >> events.txt"
}

pipeline "api" {
  steps = [group.api-build, shell.api-package]
}

shell "web-assets" {
  shell = "printf 'web-assets\n' >> events.txt; touch web-assets.done"
}

shell "web-integration" {
  depends_on = [shell.api-schema]
  shell      = "test -f api-schema.done && printf 'web-integration\n' >> events.txt && touch web-integration.done"
}

group "web-build" {
  targets = [shell.web-assets, shell.web-integration]
}

shell "web-package" {
  shell = "test -f web-assets.done && test -f web-integration.done && touch web-package.done && printf 'web-package\n' >> events.txt"
}

pipeline "web" {
  steps = [group.web-build, shell.web-package]
}

group "release" {
  targets = [pipeline.api, pipeline.web]
}
HCL

  run bach --jobs 4 run group/release
  assert_success
  assert_output_contains "[group/release] group: pipeline/api, pipeline/web"
  assert_file_contains "$E2E_PROJECT/events.txt" "api-schema"
  assert_file_contains "$E2E_PROJECT/events.txt" "api-package"
  assert_file_contains "$E2E_PROJECT/events.txt" "web-integration"
  assert_file_contains "$E2E_PROJECT/events.txt" "web-package"
  assert_line_before "$E2E_PROJECT/events.txt" "api-schema" "web-integration"
  assert_line_before "$E2E_PROJECT/events.txt" "web-integration" "web-package"
  [[ -f "$E2E_PROJECT/api-package.done" ]]
  [[ -f "$E2E_PROJECT/web-package.done" ]]
}

@test "grouped pipelines run target groups with cross-pipeline target dependencies" {
  write_bachfile <<'HCL'
project "e2e" {
  root    = "."
  default = "group.release"
  state   = ".bach/state.db"
}

shell "api-contract" {
  shell = "sleep 0.1; printf 'api-contract\n' >> events.txt; touch api-contract.done"
}

shell "api-db" {
  shell = "printf 'api-db\n' >> events.txt; touch api-db.done"
}

group "api-foundation" {
  targets = [shell.api-contract, shell.api-db]
}

shell "api-service" {
  shell = "test -f api-contract.done && test -f api-db.done && printf 'api-service\n' >> events.txt; touch api-service.done"
}

shell "api-client" {
  depends_on = [shell.web-types]
  shell      = "test -f web-types.done && printf 'api-client\n' >> events.txt; touch api-client.done"
}

group "api-compile" {
  targets = [shell.api-service, shell.api-client]
}

shell "api-package" {
  shell = "test -f api-service.done && test -f api-client.done && printf 'api-package\n' >> events.txt; touch api-package.done"
}

pipeline "api" {
  steps = [group.api-foundation, group.api-compile, shell.api-package]
}

shell "web-types" {
  depends_on = [shell.api-contract]
  shell      = "test -f api-contract.done && printf 'web-types\n' >> events.txt; touch web-types.done"
}

shell "web-assets" {
  shell = "printf 'web-assets\n' >> events.txt; touch web-assets.done"
}

group "web-compile" {
  targets = [shell.web-types, shell.web-assets]
}

shell "web-bundle" {
  depends_on = [shell.api-service]
  shell      = "test -f api-service.done && test -f web-types.done && test -f web-assets.done && printf 'web-bundle\n' >> events.txt; touch web-bundle.done"
}

group "web-package-inputs" {
  targets = [shell.web-bundle]
}

shell "web-package" {
  shell = "test -f web-bundle.done && printf 'web-package\n' >> events.txt; touch web-package.done"
}

pipeline "web" {
  steps = [group.web-compile, group.web-package-inputs, shell.web-package]
}

group "release" {
  targets = [pipeline.api, pipeline.web]
}
HCL

  run bach --jobs 6 run group/release
  assert_success
  assert_output_contains "[group/release] group: pipeline/api, pipeline/web"
  assert_output_contains "[group/api-foundation] group: shell/api-contract, shell/api-db"
  assert_output_contains "[group/api-compile] group: shell/api-service, shell/api-client"
  assert_output_contains "[group/web-compile] group: shell/web-types, shell/web-assets"
  assert_line_before "$E2E_PROJECT/events.txt" "api-contract" "web-types"
  assert_line_before "$E2E_PROJECT/events.txt" "web-types" "api-client"
  assert_line_before "$E2E_PROJECT/events.txt" "api-service" "web-bundle"
  assert_line_before "$E2E_PROJECT/events.txt" "api-client" "api-package"
  assert_line_before "$E2E_PROJECT/events.txt" "web-bundle" "web-package"
  [[ -f "$E2E_PROJECT/api-package.done" ]]
  [[ -f "$E2E_PROJECT/web-package.done" ]]
}

@test "shared dependencies run once across nested group and pipeline closures" {
  write_bachfile <<'HCL'
project "e2e" {
  root    = "."
  default = "group.release"
  state   = ".bach/state.db"
}

shell "toolchain" {
  shell = "printf 'toolchain\n' >> events.txt; touch toolchain.done"
}

shell "api-a" {
  depends_on = [shell.toolchain]
  shell      = "test -f toolchain.done && printf 'api-a\n' >> events.txt"
}

shell "api-b" {
  depends_on = [shell.toolchain]
  shell      = "test -f toolchain.done && printf 'api-b\n' >> events.txt"
}

group "api-build" {
  targets = [shell.api-a, shell.api-b]
}

pipeline "api" {
  steps = [group.api-build]
}

shell "web-a" {
  depends_on = [shell.toolchain]
  shell      = "test -f toolchain.done && printf 'web-a\n' >> events.txt"
}

shell "web-b" {
  depends_on = [shell.toolchain]
  shell      = "test -f toolchain.done && printf 'web-b\n' >> events.txt"
}

group "web-build" {
  targets = [shell.web-a, shell.web-b]
}

pipeline "web" {
  steps = [group.web-build]
}

group "release" {
  targets = [pipeline.api, pipeline.web]
}
HCL

  run bach --jobs 4 run group/release
  assert_success
  assert_line_count "$E2E_PROJECT/events.txt" "toolchain" 1
  assert_line_before "$E2E_PROJECT/events.txt" "toolchain" "api-a"
  assert_line_before "$E2E_PROJECT/events.txt" "toolchain" "api-b"
  assert_line_before "$E2E_PROJECT/events.txt" "toolchain" "web-a"
  assert_line_before "$E2E_PROJECT/events.txt" "toolchain" "web-b"
}

@test "locks serialize across sibling groups and pipelines" {
  write_bachfile <<'HCL'
project "e2e" {
  root    = "."
  default = "group.release"
  state   = ".bach/state.db"
}

shell "api-migrate" {
  lock  = "database"
  shell = "test ! -f database.lock; touch database.lock; sleep 0.1; rm database.lock; printf 'api-migrate\n' >> events.txt"
}

group "api-db" {
  targets = [shell.api-migrate]
}

pipeline "api" {
  steps = [group.api-db]
}

shell "worker-migrate" {
  lock  = "database"
  shell = "test ! -f database.lock; touch database.lock; sleep 0.1; rm database.lock; printf 'worker-migrate\n' >> events.txt"
}

group "worker-db" {
  targets = [shell.worker-migrate]
}

pipeline "worker" {
  steps = [group.worker-db]
}

group "release" {
  targets = [pipeline.api, pipeline.worker]
}
HCL

  run bach --jobs 4 run group/release
  assert_success
  assert_file_contains "$E2E_PROJECT/events.txt" "api-migrate"
  assert_file_contains "$E2E_PROJECT/events.txt" "worker-migrate"
}

@test "failure in one nested pipeline preserves shared dependency and skips later steps" {
  write_bachfile <<'HCL'
project "e2e" {
  root    = "."
  default = "group.release"
  state   = ".bach/state.db"
}

shell "prepare" {
  shell = "printf 'prepare\n' >> events.txt; touch prepare.done"
}

shell "api-build" {
  depends_on = [shell.prepare]
  shell      = "test -f prepare.done && printf 'api-build\n' >> events.txt"
}

shell "api-fail" {
  shell = "printf 'api-fail\n' >> events.txt; exit 1"
}

shell "api-after" {
  shell = "printf 'api-after\n' >> events.txt"
}

pipeline "api" {
  steps = [shell.api-build, shell.api-fail, shell.api-after]
}

shell "web-build" {
  depends_on = [shell.prepare]
  shell      = "test -f prepare.done && printf 'web-build\n' >> events.txt"
}

pipeline "web" {
  steps = [shell.web-build]
}

group "release" {
  targets = [pipeline.api, pipeline.web]
}
HCL

  run bach --jobs 4 run group/release
  assert_failure
  assert_line_count "$E2E_PROJECT/events.txt" "prepare" 1
  assert_line_before "$E2E_PROJECT/events.txt" "prepare" "api-build"
  assert_line_before "$E2E_PROJECT/events.txt" "prepare" "web-build"
  assert_file_contains "$E2E_PROJECT/events.txt" "api-fail"
  if [[ -f "$E2E_PROJECT/events.txt" && "$(<"$E2E_PROJECT/events.txt")" == *"api-after"* ]]; then
    echo "api-after should not run after a failed pipeline step" >&2
    printf '%s\n' "$(<"$E2E_PROJECT/events.txt")" >&2
    return 1
  fi
}

@test "timeout in nested composite fails before later pipeline steps" {
  write_bachfile <<'HCL'
project "e2e" {
  root    = "."
  default = "pipeline.release"
  state   = ".bach/state.db"
}

shell "slow" {
  shell = "printf 'slow-start\n' >> events.txt; sleep 0.3; printf 'slow-end\n' >> events.txt"
}

group "slow-group" {
  timeout = "50ms"
  targets = [shell.slow]
}

shell "after" {
  shell = "printf 'after\n' >> events.txt"
}

pipeline "release" {
  steps = [group.slow-group, shell.after]
}
HCL

  run bach --jobs 2 run pipeline/release
  assert_failure
  assert_output_contains "timed out"
  assert_file_contains "$E2E_PROJECT/events.txt" "slow-start"
  if [[ "$(<"$E2E_PROJECT/events.txt")" == *"after"* ]]; then
    echo "after should not run after timed out composite step" >&2
    printf '%s\n' "$(<"$E2E_PROJECT/events.txt")" >&2
    return 1
  fi
}

@test "plain dependency graph cycles fail during run planning" {
  write_bachfile <<'HCL'
project "e2e" {
  root  = "."
  state = ".bach/state.db"
}

shell "a" {
  depends_on = [shell.b]
  command    = ["true"]
}

shell "b" {
  depends_on = [shell.a]
  command    = ["true"]
}
HCL

  run bach run shell/a
  assert_failure
  assert_output_contains "dependency cycle includes"
}

@test "pipeline step cycles fail during project load" {
  write_bachfile <<'HCL'
project "e2e" {
  root  = "."
  state = ".bach/state.db"
}

pipeline "inner" {
  steps = [pipeline.outer]
}

pipeline "outer" {
  steps = [pipeline.inner]
}
HCL

  run bach list
  assert_failure
  assert_output_contains "composite target cycle includes"
}

@test "mixed group and pipeline cycles fail during project load" {
  write_bachfile <<'HCL'
project "e2e" {
  root  = "."
  state = ".bach/state.db"
}

group "loop" {
  targets = [pipeline.loop]
}

pipeline "loop" {
  steps = [group.loop]
}
HCL

  run bach graph
  assert_failure
  assert_output_contains "composite target cycle includes"
}
