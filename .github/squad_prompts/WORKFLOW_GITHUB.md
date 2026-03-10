# Squad Prompts — GitHub-Native Workflow Guide

How to drive the VECTOR development squad directly from GitHub.com — using Copilot coding agent, Copilot reviewer, GitHub Actions CI, and MCP tools — without opening VS Code.

> For the VS Code IDE workflow see [`WORKFLOW.md`](WORKFLOW.md).

---

## How It Differs From the VS Code Workflow

| Dimension | VS Code (`WORKFLOW.md`) | GitHub-native (this guide) |
|---|---|---|
| Where you work | Local editor | GitHub.com web interface |
| How context is injected | You paste `#file:` references manually | Issue body + `copilot-instructions.md` read automatically |
| Agent activated | One per session, you declare it | @copilot reads the whole `.github/agents/` dir |
| Model selection | Manual (model picker) | Set in repository Copilot settings |
| Branch + commit | Copilot writes to your local workspace | Copilot opens a PR on its own branch |
| Review | You paste `10-review.md` | You click "Request a Copilot review" on the PR |
| Terminal access | Full (Copilot runs `npm`, `tsc`, `vitest`) | CI runs checks; Copilot reads CI results |
| When to use | Exploratory, complex rule logic, debugging | Steady-state feature delivery, async iterations |

---

## Repository Infrastructure (Already In Place)

```
.github/
  copilot-instructions.md        ← read automatically on EVERY @copilot session
  SQUAD.md                       ← squad roster + implementation phases
  agents/                        ← 10 specialist agent definitions
    orchestrator.md
    product-manager.md
    architect.md
    rules-engine-dev.md
    frontend-dev.md
    test-engineer.md
    reviewer.md
    refactoring-agent.md
    debugger.md
    devops-release.md
  skills/                        ← 6 shared skill files
    vector-game-rules.md
    typescript-react-standards.md
    tdd-workflow.md
    git-branching-workflow.md
    review-checklist.md
    epic-story-breakdown.md
  squad_prompts/                 ← ready-to-paste issue bodies (this directory)
```

All of these are automatically available to the Copilot coding agent when it is assigned to an issue. You do not need to copy-paste file contents — reference them with `#file:` paths in the issue body.

### Loom-Specific Master Session

When running **Loom itself** from VS Code, use the persistent master session as
the **Loom MCP Operator** defined in `.github/agents/loom-mcp-operator.md` and
apply `.github/skills/loom-mcp-loop.md`.

This session is distinct from the GitHub-side `@copilot` coding agent. Its job
is to drive `loom_next_step` → GitHub action → `loom_checkpoint`, call
`loom_heartbeat` during waits, and abort safely if the live GitHub state and
Loom checkpoint diverge.

---

## Step 1 — Repository Settings to Configure Once

### 1.1 Enable Copilot Coding Agent

On GitHub.com → your repository → **Settings** → **Copilot** → **Coding agent**:
- Enable: ✅ Allow Copilot to create pull requests
- Branch protection: require PR review before merging (recommended)

### 1.2 Enable Copilot PR Reviews

**Settings** → **Copilot** → **Pull request reviews**:
- Enable automatic review suggestions on opened PRs
- Or trigger manually with "Request a Copilot review" button per PR

### 1.3 Set Up GitHub Actions CI

Create `.github/workflows/ci.yml` with four jobs. This is the gate that Copilot coding agent reads before marking a PR ready for review.

```yaml
# .github/workflows/ci.yml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  type-check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with: { node-version: '20', cache: 'npm' }
      - run: npm ci
      - run: npx tsc --noEmit

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with: { node-version: '20', cache: 'npm' }
      - run: npm ci
      - run: npx eslint src --max-warnings 0

  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with: { node-version: '20', cache: 'npm' }
      - run: npm ci
      - run: npx vitest run --coverage

  build:
    runs-on: ubuntu-latest
    needs: [type-check, lint, test]
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with: { node-version: '20', cache: 'npm' }
      - run: npm ci
      - run: npm run build
```

### 1.4 Configure GitHub Project Board (Optional but Recommended)

Create a **GitHub Project** (board view) linked to the repository. Add all 12 issues. Map columns to:

| Column | Meaning |
|---|---|
| **Backlog** | Not yet started |
| **Ready** | Preconditions met — can be assigned to @copilot |
| **In Progress** | Assigned to @copilot, PR open |
| **Review** | PR open, awaiting Copilot + human review |
| **Done** | Merged to `main`, AC verified |

---

## Step 2 — MCP Tools to Configure

MCP (Model Context Protocol) servers extend what the Copilot coding agent can do inside an issue or PR session. Configure them in `.github/copilot/mcp.json`.

### Create `.github/copilot/mcp.json`

```jsonc
{
  "mcpServers": {
    "playwright": {
      "type": "stdio",
      "command": "npx",
      "args": ["@playwright/mcp@latest", "--headless"],
      "description": "Browser automation — launches the Vite dev server and runs E2E interactions against the live React app. Used by the Test Engineer agent for integration tests."
    },
    "context7": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@upstash/context7-mcp@latest"],
      "description": "Fetches up-to-date library documentation for React, Vite, Vitest, and TypeScript. Prevents Copilot from using outdated API patterns."
    }
  }
}
```

> **Note**: MCP server configuration in GitHub Copilot coding agent is an evolving feature. Check [GitHub Copilot documentation](https://docs.github.com/en/copilot/customizing-copilot/extending-copilot-coding-agent-with-mcp) for the latest config format. The `mcp.json` path and schema may change.

### What Each MCP Tool Enables

#### Playwright MCP (`@playwright/mcp`)

The Copilot coding agent can:
- Spin up `npm run dev` (Vite) and navigate to `localhost:5173`
- Interact with the SVG board: click squares, select pieces, trigger moves
- Assert visual state: piece positions, Poppy markers, token indicators, UI messages
- Write E2E test files in `src/__e2e__/` using `@playwright/test`

**Relevant epics**: E3 (board rendering), E5 (player interaction), E6 (token selection), E7 (game conclusion)

**Example capability**: After implementing the TokenSelectionModal (E6), Playwright MCP lets @copilot verify the modal appears on capture, token buttons are clickable, and the Poppy is placed — without a human running the browser.

#### Context7 MCP (`@upstash/context7-mcp`)

The Copilot coding agent can:
- Fetch current React 18 / Vite 5 / Vitest 1+ API docs on demand
- Resolve the correct `@testing-library/react` query patterns (e.g., `screen.getByRole`)
- Check current Vite WASM plugin syntax for the future Zig engine migration
- Avoid deprecated patterns (e.g., outdated `act()` wrapping)

**Relevant epics**: All — especially E1 (scaffold), E3 (SVG), E4 (engine), E5–E8 (hooks/persistence)

---

## Step 3 — The Core Loop: Assign → PR → Review → Merge

```
┌──────────────────────────────────────────────────────────────┐
│ 1. Open issue (e.g., #1 — Phase 0 Foundation)               │
│    Issue body = squad_prompt content with #file: refs        │
│                                                              │
│ 2. Assign to @copilot                                        │
│    ↓ Copilot reads copilot-instructions.md automatically     │
│    ↓ Copilot reads all #file: refs in the issue body         │
│    ↓ Copilot creates branch feat/us-X.Y-slug                 │
│    ↓ Copilot implements: write tests → make green → commit   │
│    ↓ CI runs (tsc + eslint + vitest + build)                 │
│                                                              │
│ 3. Copilot opens Pull Request                                │
│    PR body = work summary + checklist                        │
│    ↓ CI status visible on PR                                 │
│                                                              │
│ 4. Request Copilot review                                    │
│    Button: "Request a Copilot review" on the PR page         │
│    ↓ Reviewer agent reads agent/reviewer.md + checklist      │
│    ↓ Produces BLOCKER / SUGGESTION / NIT inline comments     │
│                                                              │
│ 5. Human reviews Copilot's review + code                     │
│    If BLOCKER → comment on issue, @copilot iterates          │
│    If clean → approve and merge (squash)                     │
│                                                              │
│ 6. Close issue, move card to Done on project board           │
└──────────────────────────────────────────────────────────────┘
```

---

## Step 4 — How to Write Issue Bodies That Drive the Squad

The issue body **is** the prompt for @copilot. It replaces the manual paste you do in VS Code. Use the squad_prompts files as templates — either paste the full content into the issue body, or reference the file and add task-specific overrides.

### Issue Body Template

```markdown
## Agent

You are the **[AGENT NAME]** defined in [.github/agents/AGENT.md](.github/agents/AGENT.md).
Also act as the **[SECOND AGENT]** defined in [.github/agents/AGENT2.md](.github/agents/AGENT2.md) where needed.

## Skills

Apply these skills throughout:
- [vector-game-rules](.github/skills/vector-game-rules.md)
- [typescript-react-standards](.github/skills/typescript-react-standards.md)
- [tdd-workflow](.github/skills/tdd-workflow.md)
- [git-branching-workflow](.github/skills/git-branching-workflow.md)

## Preconditions

<!-- State what must already be merged before this work begins -->
- [ ] Issue #N is merged

## User Stories

<!-- Link to the story files for this phase -->
- [US-X.Y title](docs/themes/TH1-mvp/EX-name/user-stories/US-X.Y-slug.md)

## Implementation Steps

<!-- Copy the numbered steps from the matching squad_prompts file -->
1. ...
2. ...

## Definition of Done

<!-- From the squad_prompts file for this phase -->
- [ ] All acceptance criteria in the user story files are met
- [ ] `npx tsc --noEmit` passes with zero errors
- [ ] `npx eslint src --max-warnings 0` passes
- [ ] `npx vitest run --coverage` passes (≥95% branches on engine, ≥80% on UI, ≥85% overall)
- [ ] `npm run build` produces a clean dist with no warnings
- [ ] Branch is `feat/us-X.Y-slug`, commits follow Conventional Commits
```

### Mapping Issues to Squad Prompts

| Issue | Squad Prompt Body Source | Agents Activated |
|---|---|---|
| #1 Phase 0 — Foundation | `01-foundation-e1.md` | DevOps + Frontend Dev |
| #2 Phase 1 — Engine Core | `02-engine-core-e4-us41.md` | Architect + Rules Engine Dev |
| #3 Phase 2 — Lobby | `03-lobby-e2.md` | Frontend Dev |
| #4 Phase 3 — Board Rendering | `04-board-rendering-e3.md` | Frontend Dev + Test Engineer |
| #5 Phase 4 — Rules Complete | `05-rules-engine-e4-complete.md` | Rules Engine Dev + Test Engineer |
| #6 Phase 5 — Player Interaction | `06-player-interaction-e5.md` | Frontend Dev + Rules Engine Dev |
| #7 Phase 6 — Capture & Token UX | `07-capture-evolution-e6.md` | Frontend Dev + Test Engineer |
| #8 Phase 7 — Game Conclusion | `08-game-conclusion-e7.md` | Frontend Dev + Rules Engine Dev |
| #9 Phase 8 — Persistence | `09-persistence-e8.md` | Frontend Dev |
| #10 Review (standing) | `10-review.md` | Reviewer |
| #11 Debug (standing) | `11-debug.md` | Debugger |
| #12 Refactor (standing) | `12-refactor.md` | Refactoring Agent |

---

## Step 5 — The 5-Step Cross-Process Workflow

Each development cycle maps exactly to five cross-process squad prompts:

### 🔵 Step 1 — Orchestrate (Issue Comment)

Before assigning any issue to @copilot, post this comment on the issue (or open a new issue #0-status):

```
@copilot You are the Orchestrator defined in .github/agents/orchestrator.md.

Read .github/SQUAD.md and docs/themes/TH1-mvp/README.md.
Scan src/ and report:
- What is already implemented (which AC are checked in the epic files)?
- What is the next unblocked story in the sequence E1→E4/US-4.1→E2→E3→E4→E5→E6→E7→E8?
- Which issue number should I assign next?

Do not write any code. Report only.
```

@copilot will reply with a status table and name the next issue to action.

---

### 🟢 Step 2 — Implement (Assign Issue to @copilot)

1. Open the target issue (e.g., #1)
2. Verify the issue body contains the full squad_prompt content (steps, DoD, #file: refs)
3. In the **Assignees** panel → type `@copilot` → confirm

@copilot will:
- Read `copilot-instructions.md` (automatic)
- Read all `#file:` references in the issue body
- Create branch `feat/us-X.Y-slug`
- Follow the TDD cycle: write failing tests → implement → make green → commit
- Post progress comments on the issue as it works
- Open a PR when all DoD criteria pass

> **Tip**: If @copilot gets stuck or asks a clarifying question, reply directly in the issue comments. The agent will continue from your answer.

---

### 🟡 Step 3 — Review (Request Copilot Review on PR)

When @copilot opens a PR:

1. Verify all CI checks are green (tsc ✅ lint ✅ test ✅ build ✅)
2. Click **"Request a Copilot review"** on the PR page
3. Wait for inline comments

The Copilot reviewer will apply:
- `.github/agents/reviewer.md` — review persona and thresholds
- `.github/skills/review-checklist.md` — automated + manual gates
- `.github/skills/vector-game-rules.md` — rules correctness verification

**Review output format:**

```
🚫 BLOCKER: [issue] — must fix before merge
⚠️ SUGGESTION: [improvement] — should address
💬 NIT: [minor style] — optional
✅ APPROVED / 🚫 CHANGES REQUESTED
```

If `CHANGES REQUESTED`:
- Post BLOCKERs as a comment on the PR tagging @copilot
- @copilot will push a new commit addressing them
- Re-request Copilot review

---

### 🔴 Step 4 — Debug (Create a Child Issue)

When CI fails or a rule is implemented incorrectly, do not comment on the implementation issue. Instead, **create a new issue** using this body (from `11-debug.md`):

```markdown
## Agent

You are the **Debugger** defined in .github/agents/debugger.md.

Apply: .github/skills/tdd-workflow.md and .github/skills/vector-game-rules.md

## Bug Report

**Observed behaviour**: [FILL IN — e.g., slider moves through opponent Poppy on d3]
**Expected behaviour**: [FILL IN — e.g., slider must stop at c3]
**How to reproduce**: [FILL IN — e.g., `npx vitest run src/engine/__tests__/moves.ortho.test.ts`]
**Related PR / branch**: [FILL IN]

## Instructions

1. Write a regression test that reproduces the bug (must fail first).
2. Commit the failing test on the relevant branch.
3. Trace the root cause in `src/engine/`.
4. Implement the minimal fix.
5. Confirm the regression test now passes and no other tests regress.
6. Comment on this issue with: root cause, file+line fixed, regression test name.

## Definition of Done

- [ ] Regression test written and committed (red before fix, green after)
- [ ] Fix is minimal (no unrelated changes)
- [ ] Full test suite passes
- [ ] Root cause documented in this issue
```

Assign this debug issue to @copilot. It will work on the PR branch, push a fix commit, and report back.

---

### 🟣 Step 5 — Refactor (Create a Refactor Issue)

After each epic is merged, create a new issue using this body (from `12-refactor.md`):

```markdown
## Agent

You are the **Refactoring Agent** defined in .github/agents/refactoring-agent.md.

Apply: .github/skills/review-checklist.md and .github/skills/typescript-react-standards.md

## Target

**Area to refactor**: [FILL IN — e.g., `src/engine/` after E4 merge, or leave blank for full sweep]

## Instructions

1. Confirm `npx vitest run --coverage` is green before touching anything.
2. Scan for smells: magic numbers, functions > 30 lines, `any` types, duplicated logic.
3. For each High/Medium smell: fix it, re-run tests, commit.
4. Update `docs/tech-debt.md` with resolved and deferred items.
5. Open a PR titled `refactor: [area] post-epic-EX cleanup`.

## Definition of Done

- [ ] Test suite still green after every individual change
- [ ] No new `any` types introduced
- [ ] `docs/tech-debt.md` updated
- [ ] PR description lists each smell fixed with before/after
```

---

## Full Implementation Sequence on GitHub

```
GitHub Project Board — "VECTOR Development"
──────────────────────────────────────────────────────────
  Backlog         Ready          In Progress       Review          Done
  ───────         ─────          ───────────       ──────          ────
  #3 Lobby        #1 Found. ──→  @copilot works    PR #A open      
  #4 Board                       on branch         CI ✅           
  #5 Rules                       ...               Copilot review  
  #6–#9                          PR opens    ──────────────────→  #1 ✅
                  #2 Engine  ──→                                   #2 ✅
                  (after #1)                                       ...
```

### Phase Sequence with Dependencies

```
#1 Foundation (E1)
  └─ precondition: none
  └─ after merge → enable #2

#2 Engine Core (E4/US-4.1)
  └─ precondition: #1 merged
  └─ after merge → enable #3 and #4 in parallel

  #3 Lobby (E2)          ←── can run in parallel with #4
  #4 Board Rendering (E3) ←── can run in parallel with #3
    └─ after both merged → enable #5

#5 Rules Engine Complete (E4)
  └─ precondition: #2 merged
  └─ after merge → enable #6

#6 Player Interaction (E5)
  └─ precondition: #4 + #5 merged

#7 Capture & Token UX (E6)
  └─ precondition: #6 merged

#8 Game Conclusion (E7)
  └─ precondition: #7 merged

#9 Persistence (E8)
  └─ precondition: #8 merged
  └─ after merge → tag v0.1.0 🎉

── After every implementation issue ──────────────────────
  Run #10 Review (request Copilot review on the PR)
  Run #12 Refactor after each epic boundary
  Use #11 Debug for any bug found in CI or review
```

---

## Triggering Specific Agents via Issue Comments

You do not need to assign the issue to @copilot to get a targeted agent response. Post a comment tagging @copilot and declare the agent role:

```markdown
@copilot You are the Product Manager defined in .github/agents/product-manager.md.

I need acceptance criteria in Given/When/Then format for US-5.2 (select piece).
Read the story: docs/themes/TH1-mvp/E5-player-interaction/user-stories/US-5.2-select-piece.md
and the rules skill: .github/skills/vector-game-rules.md

Produce the full AC list and post it as a comment.
```

```markdown
@copilot You are the Architect defined in .github/agents/architect.md.

Review the proposed interface for Move in src/engine/types.ts (linked below).
Check it against the Zig-portability rules in your agent file.
Flag any types that won't survive the WASM migration.
```

This keeps the issue thread as the synchronous record of all architectural decisions.

---

## PR Description Convention

When @copilot opens a PR it will follow the template below (already baked into the agent definitions). If it doesn't, add this as a PR template at `.github/PULL_REQUEST_TEMPLATE.md`:

```markdown
## Summary

<!-- What was implemented and why -->

## Stories Closed

Closes #N

## Changes

- `src/engine/` — ...
- `src/ui/` — ...
- `src/__tests__/` — ...

## CI Results

- [ ] tsc --noEmit ✅
- [ ] eslint ✅
- [ ] vitest --coverage ✅ (engine: X%, UI: Y%, overall: Z%)
- [ ] build ✅

## Rules Correctness

<!-- Which rules were implemented; trace one concrete example per rule -->

## Reviewer Checklist

- [ ] No `any` types
- [ ] Engine does not import from UI
- [ ] Poppy passage-tax edge cases covered
- [ ] Token cap (max 2) enforced
- [ ] Sovereign never gains tokens
```

---

## Tips

### Use issue comments as the async terminal

When @copilot is assigned and working, it posts progress comments. You can reply with clarifications, constraints, or new information — it reads the full issue thread as part of its context.

### One story = one branch = one PR = one issue

Never bundle unrelated stories into a single issue. @copilot works best with a focused, well-scoped task that maps to a single epic story.

### Keep `copilot-instructions.md` current

This file is read automatically on every session. Add constraints discovered during the project (e.g., "do not use CSS transitions", "no `any` types", "engine must export pure functions only") to this file so every agent session picks them up without you having to repeat them.

### Protect `main`

Enforce at least one required reviewer (yourself) + passing CI before merge. This prevents @copilot from merging its own PRs without human sign-off.

### Playwright MCP: start the dev server first

For Playwright to work in GitHub Actions, add a step before the E2E job that runs `npm run dev &` and waits for the server (`wait-on http://localhost:5173`). Reference in CI:

```yaml
  e2e:
    runs-on: ubuntu-latest
    needs: [build]
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with: { node-version: '20', cache: 'npm' }
      - run: npm ci
      - run: npm run dev &
      - run: npx wait-on http://localhost:5173 --timeout 30000
      - run: npx playwright test
```

### Sprint rhythm suggestion

```
Monday    — Orchestrate: check project board, move Ready issues to In-Progress, assign to @copilot
Tuesday   — @copilot works, PRs open, CI runs
Wednesday — Review: request Copilot review on PRs, read BLOCKERs, post replies
Thursday  — @copilot addresses BLOCKERs, re-review
Friday    — Merge approved PRs, run refactor issue if epic closed, tag release if v0.1.0
```
