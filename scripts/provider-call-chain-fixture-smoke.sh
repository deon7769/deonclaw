#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<EOF
Usage: bash scripts/provider-call-chain-fixture-smoke.sh [--cli-only|--library-only]

Runs Task 22.37 provider-call chain fixture smoke / CI guard.

Default: library e2e + CLI e2e (isolated temp dirs via go test).

Options:
  --library-only  Run internal/retrievalcontext fixture smoke only.
  --cli-only      Run CLI fixture smoke only (builds deonctl in test).
  -h, --help      Show this help.
EOF
}

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

mode="all"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --library-only)
      mode="library"
      shift
      ;;
    --cli-only)
      mode="cli"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

run_library() {
  echo "==> provider-call chain fixture smoke (library e2e)"
  go test ./internal/retrievalcontext -run '^TestProviderCallChainFixtureSmokeE2E$' -count=1 -v
}

run_cli() {
  echo "==> provider-call chain fixture smoke (CLI e2e)"
  go test ./internal/retrievalcontext -run '^TestProviderCallChainFixtureCLISmokeE2E$' -count=1 -v
}

case "$mode" in
  library)
    run_library
    ;;
  cli)
    run_cli
    ;;
  all)
    run_library
    run_cli
    ;;
esac

echo "provider-call chain fixture smoke: ok"
