#!/usr/bin/env bash

set -euo pipefail

repo_root=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

required_files=(
  "docs/DEPLOYMENT.md"
  "infrastructure/infineon.service"
  "infrastructure/cloudflared/config.yml.example"
  "scripts/healthcheck.mjs"
  "scripts/install-windows-service.ps1"
)

for file in "${required_files[@]}"; do
  [[ -f "$file" ]] || { printf 'Missing native deployment asset: %s\n' "$file" >&2; exit 1; }
done

bash -n scripts/install.sh scripts/validate-native-deployment.sh
grep -q '127.0.0.1' infrastructure/infineon.service
grep -q '127.0.0.1:8080' infrastructure/cloudflared/config.yml.example
grep -q 'cmd/infineon' Makefile Dockerfile docs/DEPLOYMENT.md
grep -q 'sc.exe create' scripts/install-windows-service.ps1
grep -q "SetEnvironmentVariable.*'Process'" scripts/install-windows-service.ps1
grep -q 'DATABASE_URL=' scripts/install-windows-service.ps1
grep -q 'JWT_SECRET=' scripts/install-windows-service.ps1
if grep -q "SetEnvironmentVariable.*'Machine'" scripts/install-windows-service.ps1; then
  printf 'Windows installer must not persist service secrets at machine scope.\n' >&2
  exit 1
fi
node --check scripts/healthcheck.mjs

if grep -R -nE '^(<<<<<<<|=======|>>>>>>>)' --exclude-dir=.git --exclude-dir=node_modules --exclude-dir=.freebuff .; then
  printf 'Unresolved merge-conflict markers remain in the repository.\n' >&2
  exit 1
fi

printf 'Native deployment assets validated.\n'
