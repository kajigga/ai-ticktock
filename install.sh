#!/usr/bin/env bash
# install.sh — Install the ai-ticktock time-entry Claude Code skill
#
# Default: download latest release binary from GitHub (no Go required)
# --dev:   symlink this repo's skill/ dir and build in place (developer workflow)
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/kajigga/ai-ticktock/main/install.sh | bash
#   bash install.sh --dev   (from inside a clone of this repo)

set -euo pipefail

REPO="kajigga/ai-ticktock"
SKILL_DIR="$HOME/.claude/skills/time-entry"

GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; NC='\033[0m'
info() { echo -e "${GREEN}→${NC} $*"; }
warn() { echo -e "${YELLOW}!${NC} $*"; }
die()  { echo -e "${RED}✗${NC} $*" >&2; exit 1; }
ok()   { echo -e "${GREEN}✓${NC} $*"; }

echo ""
echo "  ai-ticktock · time-entry skill installer"
echo ""

# ── Dev mode: symlink skill/ and build in place ───────────────────────────────
if [[ "${1:-}" == "--dev" ]]; then
  REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  info "Dev mode — symlinking $REPO_DIR/skill → $SKILL_DIR"
  rm -rf "$SKILL_DIR"
  ln -s "$REPO_DIR/skill" "$SKILL_DIR"
  info "Building..."
  (cd "$REPO_DIR/go-src" && go build -o ../skill/timetracker .)
  ok "Done. $("$SKILL_DIR/timetracker" --version)"
  exit 0
fi

# ── Default: download latest release tarball ─────────────────────────────────
ARCH=$(uname -m)
[[ "$ARCH" == "arm64" ]] && ASSET="time-entry-skill-darwin-arm64.tar.gz" \
                          || ASSET="time-entry-skill-darwin-amd64.tar.gz"

info "Fetching latest release ($ARCH)..."
URL=$(python3 - <<PYEOF
import json, urllib.request
data = json.load(urllib.request.urlopen("https://api.github.com/repos/$REPO/releases/latest"))
hits = [a["browser_download_url"] for a in data.get("assets", []) if a["name"] == "$ASSET"]
print(hits[0] if hits else "")
PYEOF
)

[[ -z "$URL" ]] && die "No release found for $ASSET. Has a release been published?"

TMP=$(mktemp -d); trap "rm -rf $TMP" EXIT
info "Downloading..."
curl -fsSL "$URL" -o "$TMP/skill.tar.gz"
tar -xzf "$TMP/skill.tar.gz" -C "$TMP"

[[ -e "$SKILL_DIR" ]] && warn "Replacing existing install."
rm -rf "$SKILL_DIR"
mkdir -p "$SKILL_DIR"
for f in SKILL.md export.py tt.py pull_calendar.swift timetracker; do
  [[ -f "$TMP/$f" ]] && cp "$TMP/$f" "$SKILL_DIR/"
done
chmod +x "$SKILL_DIR/timetracker"

ok "Done. $("$SKILL_DIR/timetracker" --version)"
echo ""
echo "  Open Claude Code and try: /time-entry"
echo ""
