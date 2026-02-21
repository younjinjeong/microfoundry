# Development Workflow: Human-AI Collaborative Development

> Back to [README](../README.md) | Related: [CLAUDE.md](../CLAUDE.md) (agent rules), [ai/AGENTS.md](../ai/AGENTS.md) (agent onboarding)

MicroFoundry is developed through a structured **Human-AI pair programming workflow** using [Claude Code](https://claude.ai/claude-code) (Anthropic's AI coding agent). This isn't casual AI assistance — it's a formalized development process where AI participates in every phase of the software development lifecycle.

---

## The Workflow

```
┌────────────────────────────────────────────────────────────────────┐
│                    Epic Development Lifecycle                       │
│                                                                    │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐    │
│  │ Analyzer  │───▶│  Issue   │───▶│  Agent   │───▶│   Plan   │    │
│  │  Check   │    │ Creation │    │Discussion│    │  Review  │    │
│  └──────────┘    └──────────┘    └──────────┘    └──────────┘    │
│       │                                                │          │
│       │  ┌──────────────────────────────────────────────┘          │
│       │  │                                                        │
│  ┌────▽──▽───┐    ┌──────────┐    ┌──────────┐    ┌──────────┐  │
│  │   Code    │───▶│    PR    │───▶│  Agent   │───▶│  Merge   │  │
│  │  Implement│    │ Creation │    │  Review  │    │  to rc   │  │
│  └───────────┘    └──────────┘    └──────────┘    └──────────┘  │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
```

**Each Epic follows this process:**

1. **Analyzer Check** — Verify no open PR dependencies; ensure `rc` branch is clean
2. **Issue Creation** — Create a GitHub issue with full Epic scope, architecture decisions, and file plan
3. **Agent Discussion** — 7 specialized agents comment on the issue with their domain expertise
4. **Plan Review** — Human reviews agent feedback and approves the implementation plan
5. **Implementation** — Claude Code implements all code changes (typically 10-25 files per Epic)
6. **PR Creation** — Create a pull request targeting the `rc` branch
7. **Agent Review** — All 7 agents post review comments on the PR with findings and recommendations
8. **Merge** — Human reviews and merges to `rc`

---

## The 7+1 Agent Personas

Every issue and PR receives comments from **7 specialized review agents**. Additionally, a **Document Expert** agent runs periodically to keep docs in sync.

### Per-Epic Review Agents (every Epic)

| Agent | Role | Focus Area |
|-------|------|------------|
| **Security Architect** | Identifies vulnerabilities, auth gaps, injection risks | OWASP, authentication flows, secrets management, policy bypass |
| **Platform Engineer** | Reviews reliability, performance, crash safety | Nil guards, error handling, middleware ordering, resource leaks |
| **API Designer** | Ensures API consistency, spec compliance, contracts | REST conventions, SCIM RFC compliance, pagination, error schemas |
| **Frontend Engineer** | Reviews UI/UX, template patterns, accessibility | HTMX interactions, Tailwind consistency, error states, responsive design |
| **DevOps Engineer** | Evaluates deployment safety, observability, CI/CD | Rolling updates, log formats, monitoring integration, build pipeline |
| **QA Engineer** | Designs test plans, verification matrices, regression checks | Unit/integration/manual test cases, edge cases, acceptance criteria |
| **Product Manager** | Assesses scope, user impact, prioritization | User stories, release blocking decisions, success metrics, communication |

### Periodic Batch Agent (every 5 Epics)

| Agent | Role | Focus Area |
|-------|------|------------|
| **Document Expert** | Syncs README and docs/ with codebase reality | CLI commands, API endpoints, config changes, admin pages, architecture |

The Document Expert activates after every 5th merged Epic. It audits all changes since the last sync, classifies their documentation impact (Critical → None), updates README.md and all docs files, then creates a docs-only PR. Full workflow spec: [`.github/agents/doc-expert.md`](../.github/agents/doc-expert.md).

---

## Branch Strategy: Release-Candidate Flow

We use a **three-tier branching model** designed for structured integration and safe promotion:

```
main (stable release — production-ready)
  └── rc (release-candidate — integration & validation)
        ├── epic/feature-a  →  PR targets rc
        ├── epic/feature-b  →  PR targets rc
        └── epic/feature-c  →  PR targets rc
```

| Branch | Purpose | Merges From | Merges To |
|--------|---------|-------------|-----------|
| **`main`** | Stable release. Always deployable. Tagged for releases. | `rc` only | — |
| **`rc`** | Integration branch. All Epics land here first. Validated before promoting to `main`. | `epic/*` branches | `main` |
| **`epic/*`** | Feature branches. One per Epic. Created from `rc`, never from `main` or another epic. | — | `rc` |

**Key rules:**

1. **All PRs target `rc`**, never `main` directly
2. **Epic branches are independent** — never stack PRs by merging one epic into another
3. **Analyzer checks dependencies** — before starting a new Epic, verify no open PRs conflict
4. **`rc` → `main` promotion** happens when a set of features is validated (build passes, agents reviewed, human approved)
5. **Fast-forward merges preferred** for `rc` → `main` to keep clean history

**Typical flow:**

```bash
git checkout rc && git pull origin rc
git checkout -b epic/new-feature        # Branch from rc
# ... implement ...
gh pr create --base rc                  # PR targets rc
# ... agent review + human merge ...
# When ready to release:
git checkout main && git merge rc       # Promote to main
```

---

## Why This Workflow?

This structured process serves several purposes:

- **Quality through diverse perspectives** — Each agent catches issues specific to their domain. Security finds auth bypasses, QA designs test matrices, DevOps flags deployment risks — simultaneously.
- **Documented decision trail** — Every architectural decision, trade-off, and review finding is captured in GitHub issues and PR comments, creating a permanent knowledge base.
- **Consistent velocity** — Epics averaging 15-25 files are implemented, reviewed, and merged in single sessions with comprehensive coverage.
- **Human oversight** — The human developer ([@younjinjeong](https://github.com/younjinjeong)) reviews all agent feedback, approves plans, and makes final merge decisions. AI proposes; human disposes.
