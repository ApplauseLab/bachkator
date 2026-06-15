#!/bin/sh
set -eu

BATS_BIN=${BATS_BIN:-bats}
mkdir -p .bach/artifacts
"$BATS_BIN" test/todo.bats
printf passed > .bach/artifacts/todo-bats.passed
