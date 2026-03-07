# US-8.6 — Release workflow: `git tag v*` triggers binary upload to GitHub Releases

## Epic
E8: Integration & Hardening

## Assigned Agent

**[DevOps / Release Manager](../../../../.github/agents/devops-release.md)** — apply [`git-branching-workflow`](../../../../.github/skills/git-branching-workflow.md).


## Goal
Add a GitHub Actions release workflow that triggers on `v*` tag pushes, cross-compiles all 5 binaries, and uploads them as assets to a GitHub Release — making Loom installable without building from source.

## Acceptance Criteria

```
Given `.github/workflows/release.yml` is committed
When `git tag v0.1.0 && git push --tags` is executed
Then the release workflow triggers
  And a GitHub Release named `v0.1.0` is created
  And 5 binary assets are attached: `loom-linux-amd64`, `loom-linux-arm64`, `loom-darwin-amd64`, `loom-darwin-arm64`, `loom-windows-amd64.exe`
```

```
Given the release workflow runs
When it compiles the binaries
Then all 5 binaries are built with `CGO_ENABLED=0`
  And checksums (`sha256sum`) for each binary are attached as a `checksums.txt` asset
```

```
Given a push to `main` (not a tag)
When the CI workflow runs
Then the release workflow does NOT trigger
  And no draft release is created
```

## Tasks

1. [ ] Write a workflow-validation test that parses `release.yml` and asserts the trigger is `tags: ['v*']` (write test first)
2. [ ] Create `.github/workflows/release.yml` with `on: push: tags: ['v*']`
3. [ ] Add cross-compile matrix steps for all 5 platforms with `CGO_ENABLED=0`
4. [ ] Generate `checksums.txt` using `sha256sum` on all binaries
5. [ ] Use `softprops/action-gh-release` to create the release and upload assets
6. [ ] Create and push a test tag `v0.0.1-test` pointing at a commit on `main` and confirm the workflow produces all 5 binary assets

## Dependencies
- US-8.5

## Size Estimate
S
