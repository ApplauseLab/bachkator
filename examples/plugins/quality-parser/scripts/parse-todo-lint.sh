#!/bin/sh
set -eu

report=$1

count=0
findings=""
while IFS= read -r line; do
  [ -n "$line" ] || continue
  count=$((count + 1))
  line_number=${line%%:*}
  text=${line#*:}
  message=$(printf '%s' "$text" | sed 's/^[[:space:]]*//')
  escaped_message=$(printf '%s' "$message" | sed 's/\\/\\\\/g; s/"/\\"/g')
  if [ -n "$findings" ]; then
    findings="$findings,"
  fi
  findings="$findings{\"kind\":\"issue\",\"file\":\"src/app.txt\",\"line\":$line_number,\"severity\":\"warning\",\"rule\":\"todo\",\"message\":\"$escaped_message\"}"
done <"$report"

printf '{"metrics":[{"name":"issues.total.count","value":%s,"unit":"count"},{"name":"issues.warning.count","value":%s,"unit":"count"}],"findings":[%s]}\n' "$count" "$count" "$findings"
