#!/usr/bin/env bash
# install.sh — Install the ai-ticktock time-entry Claude Code skill
#
# Two modes:
#   default  Download the pre-built binary from the latest GitHub Release.
#            No Go required. This is the path for end users.
#
#   --dev    Symlink this repo's skill/ dir directly into ~/.claude/skills/
#            and build the Go binary in place. For developers who have the
#            repo cloned and want live edits reflected immediately.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/kajigga/ai-ticktock/main/install.sh | bash
#   bash install.sh --dev   (must be run from inside the repo)

set -euo pipefail  # exit on error, undefined vars, or pipe failures

REPO="kajigga/ai-ticktock"
SKILL_DIR="$HOME/.claude/skills/time-entry"  # where Claude Code looks for skills

info() { echo "→ $*"; }
warn() { echo "! $*"; }
die()  { echo "error: $*" >&2; exit 1; }
ok()   { echo "✓ $*"; }

echo ""
echo "  ai-ticktock · time-entry skill installer"
echo ""

# ── Dev mode ──────────────────────────────────────────────────────────────────
# Replace any existing skill dir with a symlink to this repo's skill/ folder.
# Changes to SKILL.md and scripts are picked up immediately without reinstalling.
# The Go binary is built here so the symlinked dir has a working binary.
if [[ "${1:-}" == "--dev" ]]; then
  REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  info "Dev mode — symlinking $REPO_DIR/skill → $SKILL_DIR"
  rm -rf "$SKILL_DIR"                              # remove existing dir or symlink
  ln -s "$REPO_DIR/skill" "$SKILL_DIR"
  info "Building..."
  (cd "$REPO_DIR/go-src" && go build -o ../skill/timetracker .)
  ok "Done. $("$SKILL_DIR/timetracker" --version)"
  exit 0
fi

# ── Default: download latest release ─────────────────────────────────────────
# Detect CPU architecture to pick the right tarball.
# GitHub Actions builds both arm64 (Apple Silicon) and amd64 (Intel) tarballs.
ARCH=$(uname -m)
[[ "$ARCH" == "arm64" ]] && ASSET="time-entry-skill-darwin-arm64.tar.gz" \
                          || ASSET="time-entry-skill-darwin-amd64.tar.gz"

# GitHub's /releases/latest/download/<asset> always redirects to the most
# recent release — no API call or JSON parsing needed.
URL="https://github.com/$REPO/releases/latest/download/$ASSET"

TMP=$(mktemp -d); trap "rm -rf $TMP" EXIT  # temp dir cleaned up on exit
info "Downloading latest release ($ARCH)..."
curl -fsSL "$URL" -o "$TMP/skill.tar.gz" || die "Download failed. Has a release been published?"
tar -xzf "$TMP/skill.tar.gz" -C "$TMP"

# Replace any existing install cleanly.
[[ -e "$SKILL_DIR" ]] && warn "Replacing existing install."
rm -rf "$SKILL_DIR"
mkdir -p "$SKILL_DIR"

# Copy skill files. pull_calendar (Swift binary) is intentionally excluded —
# it's compiled locally on first use by the skill itself to avoid Gatekeeper.
for f in SKILL.md export.py tt.py pull_calendar.swift timetracker; do
  [[ -f "$TMP/$f" ]] && cp "$TMP/$f" "$SKILL_DIR/"
done
chmod +x "$SKILL_DIR/timetracker"

ok "Done. $("$SKILL_DIR/timetracker" --version)"
echo ""
echo "  Open Claude Code and try: /time-entry"
echo ""
