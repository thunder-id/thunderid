---
name: spec-sync
description: Keep a feature's specification, threat model, and documentation in step with its implementation. Use whenever implementing, changing, or fixing a feature that has a spec.md or threat-model.md under docs-internals/, or behaviour described in docs/content, and whenever an implementation differs from what those documents say.
allowed-tools: Read Write Edit Bash
---

# Spec Sync

Specifications and threat models describe the feature as built, not only as planned. When the implementation and a document disagree, one of them changes.

## Step 1: Find the Documents

- The feature's `docs-internals/<feature>/spec.md` and `threat-model.md`. The spec's **Related documents** line links the threat model, issues, and discussions.
- User documentation under `docs/content/` that describes the behaviour: config tables, API references, error codes, flows.
- The API spec in `api/*.yaml` for any endpoint the change touches.

## Step 2: Compare

Check the implementation against the spec for: requirements and acceptance criteria, behaviour and failure handling, API shape and error codes, data model, config keys and defaults, event names, and anything the spec marks as out of scope.

For each mismatch:

- If the spec is right, fix the code.
- If the implementation is the intended design, update the spec.
- If it is unclear which is right, ask before changing either.

Never leave a mismatch unrecorded.

## Step 3: Update the Spec

- Change the affected section so it describes what is built.
- Bump the minor version in the header (`0.3` becomes `0.4`). For a considerable change to a released feature, breaking or not, bump the major version instead (`1.2` becomes `2.0`).
- Add a row to the **Change log** table: the new version, today's date, and a one-line change that references the update.
- Keep **Status** as it is.
- Requirements and acceptance criteria that moved out of scope go to a future specification, not deleted silently.

```markdown
| 0.4 | 2026-10-20 | Retries stop after the configured attempt limit, as implemented in #5700. |
```

## Step 4: Update the Threat Model

Update `threat-model.md` when the change alters a trust boundary, an actor's entitlements, an interaction, or a mitigation the model relies on. Bump its minor version and add a change log row the same way.

## Step 5: Update User Docs

- Update every `docs/content/` page that describes the changed behaviour, in the same PR. Follow the `docs` skill.
- An API change updates `api/*.yaml` too.
