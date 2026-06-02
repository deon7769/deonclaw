#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<EOF
Usage: bash scripts/validated-commit-push.sh [-m "commit message"] [--dry-run]

Runs DeonClaw validation and, only when it passes, stages current repo changes, commits, and pushes.

Options:
  -m, --message TEXT  Commit message. Defaults to DEONCLAW_COMMIT_MESSAGE, MSG, or a generic validated update message.
  --dry-run          Run validation and show what would be committed without committing or pushing.
  -h, --help         Show this help.
EOF
}

commit_message="${DEONCLAW_COMMIT_MESSAGE:-${MSG:-}}"
dry_run=0
remote="${DEONCLAW_GIT_REMOTE:-origin}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    -m|--message)
      if [[ $# -lt 2 ]]; then
        echo "missing value for $1" >&2
        exit 2
      fi
      commit_message="$2"
      shift 2
      ;;
    --dry-run)
      dry_run=1
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

if [[ -z "${commit_message// }" ]]; then
  commit_message="chore: validated deonclaw update"
fi

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

remote_url="$(git remote get-url "$remote" 2>/dev/null || true)"
if [[ "$remote_url" != *deonclaw* ]]; then
  echo "refusing to ship: remote $remote does not look like DeonClaw ($remote_url)" >&2
  exit 1
fi

branch="$(git branch --show-current)"
if [[ -z "$branch" ]]; then
  echo "refusing to ship: detached HEAD" >&2
  exit 1
fi

mapfile -t go_files < <(git ls-files --cached --others --exclude-standard "*.go")
if [[ ${#go_files[@]} -gt 0 ]]; then
  gofmt -w "${go_files[@]}"
fi

git diff --check
go test ./...

if [[ -z "$(git status --porcelain)" ]]; then
  echo "validation passed; no changes to commit"
  exit 0
fi

if [[ "$dry_run" -eq 1 ]]; then
  echo "validation passed"
  git status --short
  echo "dry run: would stage all current repo changes"
  echo "dry run: would commit and push to $remote/$branch"
  echo "dry run: commit message: $commit_message"
  exit 0
fi

git add -A

if git diff --cached --quiet; then
  echo "validation passed; no staged changes to commit"
  exit 0
fi

echo "validation passed"
git status --short

git commit -m "$commit_message"
git push "$remote" "$branch"
