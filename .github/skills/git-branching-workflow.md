# Skill: Git Branching Workflow

Git workflow conventions for the Loom project.

---

## Branch Strategy

```
main                   ← always releasable; protected branch
└── feat/us-X.Y-slug   ← feature work (one branch per user story)
└── fix/issue-N-slug   ← bug fixes
└── refactor/slug      ← pure refactoring (no behaviour change)
└── chore/slug         ← build, deps, config changes
└── docs/slug          ← documentation only
```

### Rules

- **Never commit directly to `main`** — all changes go through a PR
- One branch per user story or bug fix
- Branch name must include the story ID: `feat/us-2.1-fsm-states`
- Branches are deleted after merge

---

## Commit Message Convention (Conventional Commits)

```
<type>(<scope>): <short description>

[optional body]

[optional footer: closes #N]
```

### Types

| Type | When to use |
|---|---|
| `feat` | New feature or user story implementation |
| `fix` | Bug fix |
| `test` | Adding or correcting tests only |
| `refactor` | Code change that neither adds a feature nor fixes a bug |
| `chore` | Build scripts, dependencies, config, CI |
| `docs` | Documentation only |
| `style` | Formatting, whitespace (no logic change) |

### Scope

Use the package or story reference:

- `feat(fsm): add PAUSED state with retry budget`
- `fix(github): respect rate-limit header before exhaustion`
- `test(mcp): add loom_checkpoint round-trip test`
- `refactor(store): extract checkpoint serialisation helper`
- `chore(ci): add cross-compilation matrix`

### Short description

- Imperative mood: "add", "fix", "remove" — not "added", "fixed"
- No period at end
- Max 72 characters

---

## Pull Request Protocol

### PR Title

Mirror the most significant commit: `feat(fsm): add PAUSED state with retry budget`

### PR Size

- Aim for PRs under 400 lines of change
- If a story requires more, split into `feat/us-X.Y-part-1` and `feat/us-X.Y-part-2`

---

## Merge Strategy

- **Squash merge** for `feat/` and `fix/` branches
- **Rebase merge** for `refactor/` branches
- No "Merge branch …" commits on `main`

---

## Rebasing Before PR

```bash
git fetch origin
git rebase origin/main
go test ./... -race  # confirm tests still pass
```

---

## Tagging Releases

```bash
git tag -a v0.2.0 -m "Release 0.2.0: FSM + GitHub client"
git push origin v0.2.0
```
