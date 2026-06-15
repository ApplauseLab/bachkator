#!/bin/sh
set -eu

if [ -z "${GOOS:-}" ] || [ -z "${GOARCH:-}" ]; then
  printf '%s\n' 'GOOS and GOARCH must be set' >&2
  exit 1
fi

archive="dist/bachkator_${GOOS}_${GOARCH}.tar.gz"
release_dir="dist/release/bachkator_${GOOS}_${GOARCH}"

rm -rf "$release_dir" "$archive"
mkdir -p "$release_dir"

go build \
  -ldflags "-X main.version=${BACH_VERSION}" \
  -o "$release_dir/bach" \
  ./cmd/bach

tar -C "$release_dir" -czf "$archive" bach
