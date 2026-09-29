#!/bin/sh
set -eu

REPO="${TASK_MECCA_REPO:-silverkhan/TaskMecca}"
TAG="${TASK_MECCA_RELEASE_TAG:-go-main}"
BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"

say() { printf '%s\n' "$*"; }
fail() { printf 'Task Mecca install: %s\n' "$*" >&2; exit 1; }

command -v curl >/dev/null 2>&1 || fail "curl is required."

os="$(uname -s)"
arch="$(uname -m)"

case "$os" in
  Darwin)
    platform="macos"
    case "$arch" in
      arm64|aarch64) machine="arm64" ;;
      x86_64|amd64) machine="amd64" ;;
      *) fail "unsupported macOS architecture: $arch" ;;
    esac
    ;;
  Linux)
    platform="linux"
    case "$arch" in
      x86_64|amd64) machine="amd64" ;;
      *) fail "unsupported Linux architecture: $arch" ;;
    esac
    ;;
  *)
    fail "unsupported operating system: $os (Windows uses the PowerShell installer when added)."
    ;;
esac

asset="task-mecca-${platform}-${machine}"

tmp="$(mktemp -d 2>/dev/null || mktemp -d -t task-mecca)"
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

say "Downloading Task Mecca (${platform}/${machine})..."
curl -fsSL "${BASE_URL}/${asset}" -o "${tmp}/${asset}" || fail "release asset not found: ${TAG}/${asset}"
curl -fsSL "${BASE_URL}/SHA256SUMS.txt" -o "${tmp}/SHA256SUMS.txt" || fail "checksum file not found."

expected="$(awk -v f="$asset" '$2 == f || $2 == "dist/" f {print $1}' "${tmp}/SHA256SUMS.txt")"
[ -n "$expected" ] || fail "checksum entry missing for $asset"

if command -v shasum >/dev/null 2>&1; then
  actual="$(shasum -a 256 "${tmp}/${asset}" | awk '{print $1}')"
elif command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "${tmp}/${asset}" | awk '{print $1}')"
else
  fail "SHA-256 tool not found (shasum or sha256sum required)."
fi

[ "$expected" = "$actual" ] || fail "SHA-256 verification failed."
chmod +x "${tmp}/${asset}"

install_path=""
if command -v task-mecca >/dev/null 2>&1; then
  current="$(command -v task-mecca)"
  current_dir="$(dirname "$current")"
  if [ -w "$current" ] || [ -w "$current_dir" ]; then
    install_path="$current"
  fi
fi

if [ -z "$install_path" ]; then
  for dir in "/opt/homebrew/bin" "/usr/local/bin" "${HOME}/.local/bin"; do
    if [ -d "$dir" ] && [ -w "$dir" ]; then
      install_path="$dir/task-mecca"
      break
    fi
  done
fi

if [ -z "$install_path" ]; then
  mkdir -p "${HOME}/.local/bin"
  install_path="${HOME}/.local/bin/task-mecca"
fi

install_dir="$(dirname "$install_path")"
mkdir -p "$install_dir"
cp "${tmp}/${asset}" "${install_path}.tmp"
chmod +x "${install_path}.tmp"
mv "${install_path}.tmp" "$install_path"

say "Installed: $install_path"
"$install_path" --version

case ":$PATH:" in
  *":$install_dir:"*) ;;
  *)
    say ""
    say "Add this directory to PATH:"
    say "  export PATH=\"$install_dir:\$PATH\""
    ;;
esac

say ""
say "Project setup:"
say "  task-mecca init"
say "Existing project framework update:"
say "  task-mecca update"
