---
name: code-quality
description: 'Code review checklist, security audit (OWASP Top 10), architecture compliance, refactoring patterns, code conventions. Use when: reviewing code, refactoring, auditing security, checking conventions, assessing code quality.'
---

# Code Quality Skill

## Code Review Checklist

### 1. Correctness
- Does the code correctly implement the acceptance criteria?
- Are all BDD scenarios handled?
- Are edge cases covered (null, empty, boundary values)?
- Are there logic errors, off-by-one mistakes, or race conditions?
- Are error paths handled gracefully?

### 2. Security (OWASP Top 10)
- **Injection**: SQL injection, XSS, command injection, template injection?
- **Broken Access Control**: Missing authz checks, IDOR, privilege escalation?
- **Cryptographic Failures**: Plaintext secrets, weak algorithms, hardcoded keys?
- **Insecure Design**: Missing rate limiting, no input validation at boundaries?
- **Security Misconfiguration**: Debug mode on, default credentials, verbose errors?
- **Vulnerable Components**: Known CVEs in dependencies?
- **Auth Failures**: Weak passwords, missing MFA, broken session management?
- **Integrity Failures**: Unsigned updates, unvalidated deserialization?
- **Logging Failures**: Missing audit logs, logging sensitive data?
- **SSRF**: Unvalidated URLs, internal network access?

### 3. Architecture Compliance
- Does it respect component boundaries from `docs/architecture/components.md`?
- Does it follow patterns established in `docs/ADRs/`?
- Are dependencies between modules appropriate (no circular deps)?
- Does it match the tech stack in `docs/architecture/tech-stack.md`?

### 4. Code Quality
- Follows existing project conventions (naming, structure, patterns)?
- Readable and self-documenting (minimal comments needed)?
- Complexity proportional to the problem (no over-engineering)?
- No duplication (DRY within reason)?
- Functions/methods have single responsibility?

### 5. Test Quality
- Tests are meaningful (not just testing mocks)?
- Tests cover BDD scenarios from the story?
- Both happy path and error cases tested?
- Tests are deterministic (no flakiness)?
- Test names describe the scenario being tested?

## Review Severity Levels

| Level | Label | Action |
|:---|:---|:---|
| Critical | `must-fix` | Blocks approval — security holes, data loss, crashes |
| Suggestion | `should-fix` | Important but not blocking — quality, patterns |
| Nit | `optional` | Style, naming, minor improvements |

## Refactoring Patterns

When refactoring at epic boundaries, look for:

| Pattern | Signal | Action |
|:---|:---|:---|
| Extract utility | Same logic in 2+ files | Create shared module |
| Normalize pattern | Inconsistent approach to same problem | Adopt dominant convention |
| Simplify | Cyclomatic complexity > 10 | Decompose into smaller functions |
| Remove dead code | Unreachable or unused code | Delete it |
| Extract config | Hardcoded values repeated | Move to configuration |
| Introduce type | Raw dicts/maps with known shape | Create a typed structure |

### Refactoring Rules
- **Behavior-preserving**: All tests must still pass
- **Minimal**: Targeted improvements, not rewrites
- **Documented**: Every change has a stated rationale
- **Tested**: Run full test suite after refactoring, not just affected tests

## Documentation Review Checklist

When reviewing documentation quality:
- README files are up-to-date with current functionality
- API documentation matches implementation
- Architecture docs reflect actual component structure
- ADRs are not contradicted by implementation
- CHANGELOG entries are accurate and complete
- User story acceptance criteria match what was actually built

## UX / Accessibility Checklist (UI Projects Only)

> **Applies only when the project includes user-facing UI components.** Skip this section for backend-only, CLI-only, or infrastructure projects.

### WCAG Compliance
- Semantic HTML elements used (headings, landmarks, lists, buttons)
- All images have meaningful `alt` text (or `alt=""` for decorative)
- Form inputs have associated `<label>` elements
- ARIA attributes used correctly where native semantics are insufficient
- Page has a logical heading hierarchy (h1 → h2 → h3, no skipped levels)

### Keyboard Navigation
- All interactive elements reachable via Tab key in logical order
- Focus indicator is visible on all focusable elements
- Modal dialogs trap focus and return focus on close
- Custom controls handle Enter/Space for activation, Escape for dismiss
- No keyboard traps — user can always Tab away

### Color & Contrast
- Text meets WCAG AA contrast ratio (4.5:1 normal text, 3:1 large text)
- Information is not conveyed by color alone (use icons, patterns, or labels too)
- UI is usable in high-contrast mode and with color-blindness simulation
