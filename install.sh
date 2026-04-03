#!/usr/bin/env bash
# install.sh — Install the ai-ticktock time-entry skill
#
# Installs into Claude Code (~/.claude/skills/time-entry) and, if opencode is
# detected, also into opencode (~/.config/opencode/skills/time-entry). Both
# apps load skills from their respective skills/ directories using the same
# SKILL.md format, so a single install works for both.
#
# Two modes:
#   default  Download the pre-built binary from the latest GitHub Release.
#            No Go required. This is the path for end users.
#
#   --dev    Symlink this repo's skill/ dir directly into the skills dirs
#            and build the Go binary in place. For developers who have the
#            repo cloned and want live edits reflected immediately.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/kajigga/ai-ticktock/main/install.sh | bash
#   bash install.sh --dev   (must be run from inside the repo)

set -euo pipefail  # exit on error, undefined vars, or pipe failures

REPO="kajigga/ai-ticktock"
CLAUDE_SKILL_DIR="$HOME/.claude/skills/time-entry"      # Claude Code skill location
OPENCODE_SKILL_DIR="$HOME/.config/opencode/skills/time-entry"  # opencode skill location

info() { echo "→ $*"; }
warn() { echo "! $*"; }
die()  { echo "error: $*" >&2; exit 1; }
ok()   { echo "✓ $*"; }

echo ""
echo "  ai-ticktock · time-entry skill installer"
echo ""

# Detect whether opencode is installed — if so, install there too.
# opencode uses the same SKILL.md format and skills/ directory convention.
OPENCODE_INSTALLED=false
command -v opencode &>/dev/null && OPENCODE_INSTALLED=true

# ── Dev mode ──────────────────────────────────────────────────────────────────
# Replace any existing skill dirs with symlinks to this repo's skill/ folder.
# Changes to SKILL.md and scripts are picked up immediately without reinstalling.
# The Go binary is built here so the symlinked dir has a working binary.
if [[ "${1:-}" == "--dev" ]]; then
  REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

  info "Dev mode — symlinking $REPO_DIR/skill → $CLAUDE_SKILL_DIR"
  rm -rf "$CLAUDE_SKILL_DIR"
  ln -s "$REPO_DIR/skill" "$CLAUDE_SKILL_DIR"

  if $OPENCODE_INSTALLED; then
    info "opencode detected — symlinking $REPO_DIR/skill → $OPENCODE_SKILL_DIR"
    rm -rf "$OPENCODE_SKILL_DIR"
    ln -s "$REPO_DIR/skill" "$OPENCODE_SKILL_DIR"
  fi

  info "Building Go binary..."
  (cd "$REPO_DIR/go-src" && go build -o ../skill/timetracker .)
  ok "Done. $("$CLAUDE_SKILL_DIR/timetracker" --version)"
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

# ── Install into Claude Code ───────────────────────────────────────────────────
[[ -e "$CLAUDE_SKILL_DIR" ]] && warn "Replacing existing Claude Code install."
rm -rf "$CLAUDE_SKILL_DIR"
mkdir -p "$CLAUDE_SKILL_DIR"

# Copy skill files. pull_calendar (Swift binary) is intentionally excluded —
# it's compiled locally on first use by the skill itself to avoid Gatekeeper.
for f in SKILL.md export.py tt.py pull_calendar.swift timetracker; do
  [[ -f "$TMP/$f" ]] && cp "$TMP/$f" "$CLAUDE_SKILL_DIR/"
done
chmod +x "$CLAUDE_SKILL_DIR/timetracker"
ok "Claude Code: installed."

# ── Install into opencode (if detected) ───────────────────────────────────────
# opencode uses the same SKILL.md format. We copy the same files into its
# skills directory. The timetracker binary is shared via the same binary path.
if $OPENCODE_INSTALLED; then
  [[ -e "$OPENCODE_SKILL_DIR" ]] && warn "Replacing existing opencode install."
  rm -rf "$OPENCODE_SKILL_DIR"
  mkdir -p "$OPENCODE_SKILL_DIR"
  for f in SKILL.md export.py tt.py pull_calendar.swift timetracker; do
    [[ -f "$CLAUDE_SKILL_DIR/$f" ]] && cp "$CLAUDE_SKILL_DIR/$f" "$OPENCODE_SKILL_DIR/"
  done
  chmod +x "$OPENCODE_SKILL_DIR/timetracker"
  ok "opencode: installed."
fi

ok "Done. $("$CLAUDE_SKILL_DIR/timetracker" --version)"
echo ""
echo "  Open Claude Code or opencode and try: /time-entry"
echo ""
