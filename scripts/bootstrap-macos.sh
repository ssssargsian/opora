#!/bin/zsh
set -euo pipefail

if [[ "$(uname -s)" != "Darwin" || "$(uname -m)" != "arm64" ]]; then
  echo "This bootstrap targets macOS on Apple Silicon." >&2
  exit 1
fi

if [[ ! -x /opt/homebrew/bin/brew ]]; then
  echo "Homebrew is required at /opt/homebrew. Install it from https://brew.sh and run this command again." >&2
  exit 1
fi

required=(go node pnpm docker)
for command_name in $required; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "Missing required command: $command_name" >&2
    exit 1
  fi
done

go version
node --version
pnpm --version
docker --version
docker compose version
echo "Development toolchain is ready."
