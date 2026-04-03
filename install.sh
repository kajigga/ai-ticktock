#!/usr/bin/env bash
# install.sh — Install the ai-ticktock time-entry Claude Code skill
#
# Modes (auto-detected, or pass a flag):
#   default        Download latest release from GitHub (no Go required)
#   --from-source  Clone repo + build from Go source
#   --dev          Symlink this repo's skill/ dir (developer workflow)
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/kajigga/ai-ticktock/main/install.sh | bash
#   curl -fsSL .../install.sh | bash -s -- --from-source
#   bash install.sh --dev   (from inside a clone of this repo)

set -euo pipefail

REPO="kajigga/ai-ticktock"
SKILL_DIR="$HOME/.claude/skills/time-entry"
MODE="auto"

for arg in "$@"; do
  case "$arg" in
    --dev)         MODE="dev" ;;
    --from-source) MODE="source" ;;
  esac
done

# ── Colours ──────────────────────────────────────────────────────────────────
GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; NC='\033[0m'
info()    { echo -e "${GREEN}→${NC} $*"; }
warn()    { echo -e "${YELLOW}!${NC} $*"; }
die()     { echo -e "${RED}✗${NC} $*" >&2; exit 1; }
ok()      { echo -e "${GREEN}✓${NC} $*"; }

echo ""
echo "  ai-ticktock · time-entry skill installer"
echo ""

# ── Helpers ───────────────────────────────────────────────────────────────────

# Copy skill text files and binary from a source directory into SKILL_DIR,
# then compile pull_calendar from Swift source if swiftc is available.
install_from_dir() {
  local src="$1"
  mkdir -p "$SKILL_DIR"
  for f in SKILL.md export.py tt.py pull_calendar.swift; do
    [[ -f "$src/$f" ]] && cp "$src/$f" "$SKILL_DIR/"
  done
  cp "$src/timetracker" "$SKILL_DIR/timetracker"
  chmod +x "$SKILL_DIR/timetracker"
}

# ── Dev mode: symlink skill/ and build in place ───────────────────────────────
if [[ "$MODE" == "dev" ]]; then
  REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  info "Dev mode — symlinking $REPO_DIR/skill → $SKILL_DIR"
  rm -rf "$SKILL_DIR"
  ln -s "$REPO_DIR/skill" "$SKILL_DIR"
  info "Building Go binary..."
  (cd "$REPO_DIR/go-src" && go build -o ../skill/timetracker .)
  ok "Installed (dev/symlink). Binary: $("$SKILL_DIR/timetracker" --version)"
  exit 0
fi

# ── Source mode: clone + build (also used when Go is available in auto mode) ──
do_source_install() {
  command -v go &>/dev/null || die "Go is required for --from-source. Install from https://go.dev/dl/"
  info "Cloning repository..."
  TMP=$(mktemp -d)
  trap "rm -rf $TMP" EXIT
  git clone --depth 1 "https://github.com/$REPO.git" "$TMP/repo" --quiet
  info "Building from source..."
  (cd "$TMP/repo/go-src" && go build -o ../skill/timetracker .)
  [[ -d "$SKILL_DIR" && ! -L "$SKILL_DIR" ]] && warn "Replacing existing install at $SKILL_DIR"
  rm -rf "$SKILL_DIR"
  install_from_dir "$TMP/repo/skill"
}

if [[ "$MODE" == "source" ]]; then
  do_source_install
  ok "Installed from source. Binary: $("$SKILL_DIR/timetracker" --version)"
  exit 0
fi

# ── Auto mode: prefer source if Go available, otherwise download release ──────

if command -v go &>/dev/null; then
  info "Go detected — building from source."
  do_source_install
  ok "Installed from source. Binary: $("$SKILL_DIR/timetracker" --version)"
  exit 0
fi

# No Go — download latest GitHub release tarball
ARCH=$(uname -m)
[[ "$ARCH" == "arm64" ]] && TARBALL_NAME="time-entry-skill-darwin-arm64.tar.gz" \
                          || TARBALL_NAME="time-entry-skill-darwin-amd64.tar.gz"

info "Fetching latest release info (arch: $ARCH)..."
RELEASE_URL=$(python3 - "$TARBALL_NAME" <<'PYEOF'
import json, sys, urllib.request
name = sys.argv[1]
try:
    url = "https://api.github.com/repos/kajigga/ai-ticktock/releases/latest"
    data = json.load(urllib.request.urlopen(url))
    assets = [a["browser_download_url"] for a in data.get("assets", [])
              if a["name"] == name]
    print(assets[0] if assets else "")
except Exception as e:
    sys.exit(f"GitHub API error: {e}")
PYEOF
)

[[ -z "$RELEASE_URL" ]] && die "No release tarball found. Try: bash install.sh --from-source"

info "Downloading $RELEASE_URL ..."
TMP=$(mktemp -d)
trap "rm -rf $TMP" EXIT
curl -fsSL "$RELEASE_URL" -o "$TMP/skill.tar.gz"
tar -xzf "$TMP/skill.tar.gz" -C "$TMP"

[[ -d "$SKILL_DIR" && ! -L "$SKILL_DIR" ]] && warn "Replacing existing install at $SKILL_DIR"
rm -rf "$SKILL_DIR"
install_from_dir "$TMP"

ok "Installed from release. Binary: $("$SKILL_DIR/timetracker" --version)"
echo ""
echo "  The time-entry skill is ready. Open Claude Code and try: /time-entry"
echo ""
