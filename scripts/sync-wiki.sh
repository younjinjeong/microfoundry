#!/bin/bash
# Sync docs/ markdown files to GitHub Wiki
# Usage: ./scripts/sync-wiki.sh
#
# Source of truth: docs/*.md in the main repo
# Target: https://github.com/younjinjeong/microfoundry.wiki.git

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WIKI_DIR="${WIKI_DIR:-/tmp/microfoundry-wiki}"
WIKI_REPO="https://github.com/younjinjeong/microfoundry.wiki.git"

echo "Syncing docs/ → GitHub Wiki"

# Clone or pull wiki repo
if [ -d "$WIKI_DIR/.git" ]; then
    echo "Pulling existing wiki clone..."
    cd "$WIKI_DIR" && git pull origin master
else
    echo "Cloning wiki repo..."
    rm -rf "$WIKI_DIR"
    git clone "$WIKI_REPO" "$WIKI_DIR"
fi

# Copy and rename files (repo name → wiki name)
cp "$REPO_ROOT/docs/user-manual.md"                    "$WIKI_DIR/User-Manual.md"
cp "$REPO_ROOT/docs/admin-guide.md"                     "$WIKI_DIR/Admin-Guide.md"
cp "$REPO_ROOT/docs/architecture.md"                    "$WIKI_DIR/Architecture.md"
cp "$REPO_ROOT/docs/development-workflow.md"            "$WIKI_DIR/Development-Workflow.md"
cp "$REPO_ROOT/docs/cloudfoundry-vs-microfoundry.md"    "$WIKI_DIR/CloudFoundry-vs-MicroFoundry.md"
cp "$REPO_ROOT/docs/cloudfoundry-architecture.md"       "$WIKI_DIR/CloudFoundry-Architecture.md"
cp "$REPO_ROOT/docs/observability-capacity.md"          "$WIKI_DIR/Observability-and-Capacity.md"

# Fix relative links in Development-Workflow.md for wiki context
sed -i 's|\](../README.md)|](https://github.com/younjinjeong/microfoundry#readme)|g' "$WIKI_DIR/Development-Workflow.md"
sed -i 's|\](../CLAUDE.md)|](https://github.com/younjinjeong/microfoundry/blob/rc/CLAUDE.md)|g' "$WIKI_DIR/Development-Workflow.md"
sed -i 's|\](../ai/AGENTS.md)|](https://github.com/younjinjeong/microfoundry/blob/rc/ai/AGENTS.md)|g' "$WIKI_DIR/Development-Workflow.md"
sed -i 's|\](../.github/agents/doc-expert.md)|](https://github.com/younjinjeong/microfoundry/blob/rc/.github/agents/doc-expert.md)|g' "$WIKI_DIR/Development-Workflow.md"

# Commit and push if there are changes
cd "$WIKI_DIR"
git add -A
if git diff --staged --quiet; then
    echo "No changes to sync."
else
    git commit -m "docs: sync from main repo $(date +%Y-%m-%d)"
    git push origin master
    echo "Wiki updated successfully."
fi
