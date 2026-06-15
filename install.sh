#!/usr/bin/env sh
set -eu

repo="${BACH_REPO:-ApplauseLab/bachkator}"
install_dir="${BACH_INSTALL_DIR:-/usr/local/bin}"

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "bach installer: missing required command: $1" >&2
    exit 1
  fi
}

need curl
need tar
need uname

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"

case "$os" in
  darwin|linux) ;;
  *)
    echo "bach installer: unsupported OS: $os" >&2
    exit 1
    ;;
esac

case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *)
    echo "bach installer: unsupported architecture: $arch" >&2
    exit 1
    ;;
esac

archive="bachkator_${os}_${arch}.tar.gz"
url="https://github.com/${repo}/releases/latest/download/${archive}"
tmp_dir="$(mktemp -d)"

cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT INT TERM

echo "Downloading $url"
curl -fsSL "$url" -o "$tmp_dir/$archive"
tar -xzf "$tmp_dir/$archive" -C "$tmp_dir" bach

if [ ! -d "$install_dir" ]; then
  mkdir -p "$install_dir" 2>/dev/null || {
    need sudo
    sudo mkdir -p "$install_dir"
  }
fi

if [ -w "$install_dir" ]; then
  mv "$tmp_dir/bach" "$install_dir/bach"
else
  need sudo
  sudo mv "$tmp_dir/bach" "$install_dir/bach"
fi
chmod +x "$install_dir/bach" 2>/dev/null || sudo chmod +x "$install_dir/bach"

echo "Installed bach to $install_dir/bach"
