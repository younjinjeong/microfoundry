# MicroFoundry — Claude Code Project Rules

## Branch Strategy: Release-Candidate Flow

```
main (stable)
  └── rc (release-candidate, integration branch)
        ├── epic/feature-a  →  PR targets rc
        ├── epic/feature-b  →  PR targets rc
        └── epic/feature-c  →  PR targets rc
```

### Rules

1. **`main`** is the stable release branch. Only `rc` merges into `main` when validated.
2. **`rc`** is the integration branch. All Epic PRs target `rc`, not `main`.
3. **`epic/*`** branches are created from `rc` (never from `main`).
4. When `rc` accumulates a validated set of features, it is merged to `main` and optionally tagged as a release.
5. Never stack PRs by merging one epic branch into another. Each epic branch is independent and based on `rc`.

### CRITICAL — Never Push to `main` Directly

- **NEVER run `git push origin main`** under any circumstances.
- A pre-push hook (`.git/hooks/pre-push`) blocks direct pushes to `main` as a safety net.
- The `sync-rc-to-main` GitHub Action automatically syncs `main` when PRs merge to `rc`.
- If `main` falls behind `rc`, let the next PR merge trigger the sync — do NOT manually push.
- All PRs MUST have labels applied before requesting review. Use `--label` flag or `gh api` to add labels.

### Branch Cleanup

- After a PR is merged, delete the feature branch (remote and local).
- Run `git fetch --prune` periodically to clean up stale remote tracking branches.
- Never leave merged branches lingering on the remote.

### Creating a New Epic Branch

```bash
git checkout rc
git pull origin rc
git checkout -b epic/new-feature
```

### Merging Flow

```
epic/feature → PR → rc (merge) → validate → PR → main (merge + tag)
```

---

## Agent Workflow Rules

### Analyzer Agent — Dependency Check (MANDATORY)

Before planning any new Epic, the Analyzer agent MUST:

1. **Check for open PRs** — Run `gh pr list --state open` and identify any unmerged PRs.
2. **Identify dependencies** — Determine if the new Epic relies on code from any open PR (shared files, APIs, models, routes, templates).
3. **If dependencies exist** — STOP and ask the user to merge the dependent PRs into `rc` first. Do NOT proceed with implementation until dependencies are cleared.
4. **Report dependency status** in the Analyzer report:

```markdown
### Dependency Check
| PR | Title | Status | Required? |
|----|-------|--------|-----------|
| #26 | UAA Keycloak | Open | YES — new Epic uses auth middleware |

**Action Required**: Merge PR #26 into `rc` before starting this Epic.
```

5. **New Epic branches MUST be based on `rc`** — never on `main` or another epic branch.

### All Agents — Branch Awareness

- When creating a new branch, always branch from `rc`.
- When the `rc` branch does not exist yet, ask the user to create it first.
- PR base branch must always be `rc` (use `gh pr create --base rc`).

### Document Expert Agent — Periodic Docs Sync (EVERY 5 EPICS)

After every 5 merged Epics (counted from the last `docs: sync documentation` commit), trigger the Document Expert:

1. **Audit** — Collect all merged PRs since last sync, identify new features/commands/endpoints/config changes.
2. **Classify** — Categorize impact: Critical (new feature), High (new API), Medium (bugfix), Low (refactor), None (skip).
3. **Draft** — Update `README.md` (Development History, capabilities, structure) and `docs/` files (user-manual, admin-guide, architecture).
4. **Self-review** — Validate all links, examples, screenshots, and config references.
5. **PR** — Create `docs/sync-N` branch, commit, and open PR targeting `rc`.

Full spec: `.github/agents/doc-expert.md`

---

## Project Conventions

- **Language**: Go 1.25+
- **CLI Framework**: cobra
- **Config**: viper + YAML (`configs/mf.yaml`)
- **K8s Client**: client-go with multi-cluster ClientManager
- **Admin UI**: Go templates + HTMX + Tailwind CSS (CDN)
- **Templates**: embed.FS with clone pattern (one clone per page)
- **Auth**: OIDC via coreos/go-oidc/v3 + Keycloak
- **Monitoring**: Prometheus + Loki + Grafana + Beyla
- **Settings storage**: K8s ConfigMap/Secret (not file-based)
- **No new dependencies** unless absolutely necessary — prefer stdlib

## Build & Verify

```bash
go build ./...          # Must pass
go vet ./...            # Must pass
go test ./...           # Must pass
```
