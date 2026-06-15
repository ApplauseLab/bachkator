#!/usr/bin/env bats

load helpers

setup() {
  setup_bach
  make_project
}

write_quality_plugin() {
  mkdir -p "$E2E_PROJECT/scripts"
  cat >"$E2E_PROJECT/scripts/parse-quality.sh" <<'SH'
#!/bin/sh
set -eu
report=$1
errors=$(awk -F= '/^errors=/ { print $2 }' "$report")
case "${errors:-}" in
  ''|*[!0-9]*) printf 'invalid report\n' >&2; exit 2 ;;
esac
cat <<JSON
{"metrics":[{"name":"issues.total.count","value":$errors,"unit":"count"},{"name":"issues.error.count","value":$errors,"unit":"count"}],"findings":[]}
JSON
SH
  chmod +x "$E2E_PROJECT/scripts/parse-quality.sh"
}

@test "quality plugin parses metrics and gates reports" {
  write_quality_plugin
  write_bachfile <<'HCL'
project "e2e" {
  root  = "."
}

plugin "lint_parser" {
  type    = "quality"
  command = ["sh", "scripts/parse-quality.sh"]
}

shell "lint" {
  shell = "mkdir -p .bach/artifacts && printf 'errors=0\n' > .bach/artifacts/lint.txt"
}

quality "shell.lint" {
  lint {
    path   = ".bach/artifacts/lint.txt"
    parser = plugin.lint_parser
  }

  quality_gate {
    metric = "issues.total.count"
    max    = 0
  }
}
HCL

  run bach run --log-only shell/lint
  assert_success
  assert_output_contains "quality report .bach/artifacts/lint.txt parsed: metrics=2 findings=0"

  run bach quality metrics
  assert_success
  assert_output_contains "issues.total.count"
  assert_output_contains "issues.error.count"
}

@test "quality plugin gate failure is quality-failed" {
  write_quality_plugin
  write_bachfile <<'HCL'
project "e2e" {
  root  = "."
}

plugin "lint_parser" {
  type    = "quality"
  command = ["sh", "scripts/parse-quality.sh"]
}

shell "lint" {
  shell = "mkdir -p .bach/artifacts && printf 'errors=1\n' > .bach/artifacts/lint.txt"
}

quality "shell.lint" {
  lint {
    path   = ".bach/artifacts/lint.txt"
    parser = plugin.lint_parser
  }

  quality_gate {
    metric = "issues.total.count"
    max    = 0
  }
}
HCL

  run bach run --log-only shell/lint
  assert_failure
  assert_output_contains "quality gate failed"
  assert_output_contains "quality-failed=1"
}

@test "quality plugin parser failure does not retry" {
  write_quality_plugin
  cat >"$E2E_PROJECT/scripts/write-broken-report.sh" <<'SH'
#!/bin/sh
set -eu
n=$(cat attempts 2>/dev/null || printf 0)
n=$((n + 1))
printf '%s' "$n" > attempts
mkdir -p .bach/artifacts
printf 'broken\n' > .bach/artifacts/lint.txt
SH
  chmod +x "$E2E_PROJECT/scripts/write-broken-report.sh"
  write_bachfile <<'HCL'
project "e2e" {
  root  = "."
}

plugin "lint_parser" {
  type    = "quality"
  command = ["sh", "scripts/parse-quality.sh"]
}

shell "lint" {
  command = ["sh", "scripts/write-broken-report.sh"]
  retry {
    attempts                      = 3
    retry_on_quality_gate_failure = true
  }
}

quality "shell.lint" {
  lint {
    path   = ".bach/artifacts/lint.txt"
    parser = plugin.lint_parser
  }
}
HCL

  run bach run --log-only shell/lint
  assert_failure
  assert_output_contains "quality report .bach/artifacts/lint.txt failed"
  assert_file_contains "$E2E_PROJECT/attempts" "1"
}

@test "retry_on_quality_gate_failure retries only gate failures" {
  write_quality_plugin
  cat >"$E2E_PROJECT/scripts/write-flaky-report.sh" <<'SH'
#!/bin/sh
set -eu
n=$(cat attempts 2>/dev/null || printf 0)
n=$((n + 1))
printf '%s' "$n" > attempts
mkdir -p .bach/artifacts
if [ "$n" -lt 2 ]; then
  printf 'errors=1\n' > .bach/artifacts/lint.txt
else
  printf 'errors=0\n' > .bach/artifacts/lint.txt
fi
SH
  chmod +x "$E2E_PROJECT/scripts/write-flaky-report.sh"
  write_bachfile <<'HCL'
project "e2e" {
  root  = "."
}

plugin "lint_parser" {
  type    = "quality"
  command = ["sh", "scripts/parse-quality.sh"]
}

shell "lint" {
  command = ["sh", "scripts/write-flaky-report.sh"]
  retry {
    attempts                      = 3
    retry_on_quality_gate_failure = true
  }
}

quality "shell.lint" {
  lint {
    path   = ".bach/artifacts/lint.txt"
    parser = plugin.lint_parser
  }

  quality_gate {
    metric = "issues.total.count"
    max    = 0
  }
}
HCL

  run bach run --log-only shell/lint
  assert_success
  assert_file_contains "$E2E_PROJECT/attempts" "2"
}

@test "cached quality plugin target does not rerun but force does" {
  write_quality_plugin
  cat >"$E2E_PROJECT/scripts/write-cacheable-report.sh" <<'SH'
#!/bin/sh
set -eu
n=$(cat attempts 2>/dev/null || printf 0)
n=$((n + 1))
printf '%s' "$n" > attempts
mkdir -p .bach/artifacts
printf 'errors=0\n' > .bach/artifacts/lint.txt
SH
  chmod +x "$E2E_PROJECT/scripts/write-cacheable-report.sh"
  write_bachfile <<'HCL'
project "e2e" {
  root  = "."
}

plugin "lint_parser" {
  type    = "quality"
  command = ["sh", "scripts/parse-quality.sh"]
}

shell "lint" {
  command = ["sh", "scripts/write-cacheable-report.sh"]
  inputs  = ["input.txt"]
  outputs = [".bach/artifacts/lint.txt"]
}

quality "shell.lint" {
  lint {
    path   = ".bach/artifacts/lint.txt"
    parser = plugin.lint_parser
  }
}
HCL
  printf 'one\n' >"$E2E_PROJECT/input.txt"

  run bach run --log-only shell/lint
  assert_success
  assert_file_contains "$E2E_PROJECT/attempts" "1"

  run bach run --log-only shell/lint
  assert_success
  assert_output_contains "[shell/lint] (cached)"
  assert_file_contains "$E2E_PROJECT/attempts" "1"

  run bach run --log-only --force shell/lint
  assert_success
  assert_file_contains "$E2E_PROJECT/attempts" "2"
}
