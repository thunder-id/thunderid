---
name: pr-preflight
description: Self-review a ThunderID change against the conventions reviewers check. Run only when a developer explicitly asks for a preflight or self-review of their change.
allowed-tools: Read Edit Bash
disable-model-invocation: true
---

# PR Preflight

An optional review pass over a finished change. The `backend` and `backend-tests` skills already apply these rules while code is written, so run this only when asked. Fix what it finds, then report.

## Step 1: Collect the Change

- Base: the merge base with `main` on the `thunder-id/thunderid` remote, or the release branch the change targets.
- Read the full diff against the base, including uncommitted and untracked files.
- Review **added lines only**. Do not flag or fix existing code the change did not touch.

## Step 2: Check Every Added Line

### All Files

- [ ] No bare short form of the product name (`AGENTS.md`, Product Name Rules).
- [ ] No em dashes or double hyphens in copy, UI strings, i18n locale files, or error messages.
- [ ] New source files carry the license header with the current year only, never a range.
- [ ] No private keys or real credentials outside test fixtures.
- [ ] Nothing outside the task: no unrelated refactoring, no comments on untouched code, no new dependencies, CI, or Makefile changes without approval.

### Backend Go

Apply each file of the `backend` skill to the added code:

- [ ] `layout.md`: file names, what goes in which file, method order, package comment.
- [ ] `visibility.md`: nothing exported that the package's consumers do not use, exposed interfaces named `<Thing>Interface`, dependencies and config passed through `Initialize`.
- [ ] `errors.md`: error constants in `error_constants.go`, codes in ascending order and unused elsewhere, `{{param(name)}}` placeholders, a description that adds context.
- [ ] `logging.md`: logger on the struct, no logger name constant, no new Info logs for routine operations, no client error or executor error logs, no PII.
- [ ] `idiom.md`: typed constants, header names from `internal/system/constants`, no hand-rolled helper the language or `internal/system/utils` already has, no unaccepted lint suppressions, minimal comments.
- [ ] `resource.md` and `config.md` checklists, when the change adds a resource or a config key.
- [ ] Query IDs in `store_constants.go` are in ascending order with the new one at the end (`db` skill).

### Tests

- [ ] New test files use a testify suite named after the file. No hand-written mocks (`backend-tests` skill).
- [ ] Integration tests cover the change. A bug fix has a regression test when the case is reproducible.
- [ ] Mocks are regenerated after any interface change.

### Spec and Docs

- [ ] The spec, threat model, user docs, and `api/*.yaml` match the implementation, with the spec's minor version bumped and a change log row added (`spec-sync` skill).

### Frontend and Docs

- [ ] Frontend changes follow `frontend/AGENTS.md`.
- [ ] Documentation changes pass the `docs` skill's `review.md`.

## Step 3: Validate

Run the inner-loop checks for the area touched (`backend/AGENTS.md`, `frontend/AGENTS.md`), then `make pr_checks` before opening a PR.
