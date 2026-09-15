---
name: prune-npm-overrides
description: Audit the pnpm overrides a repo already carries, remove the ones upstream has since fixed, and require a justification comment on every one that stays. The counterpart to fix-npm-vulnerability. Use when asked to check, clean up, or prune the overrides, or when an override's tracking issue comes up for review.
allowed-tools: Bash(*) Read(*) Edit(*) WebSearch(*)
---

# Prune npm Overrides

`fix-npm-vulnerability` adds an override when no upstream fix exists yet. This skill is the other half: it goes back through the overrides already in the tree and retires the ones that are no longer doing anything.

An override is a workaround, not a fix. It pins a transitive package past what a head dependency asks for, and it keeps doing that long after the head dependency ships a release with a patched range. Left unaudited, `overrides:` only grows, and every stale entry constrains unrelated upgrades.

Each entry ends the run in exactly one of three states:

| Outcome | Condition | Action |
|---|---|---|
| Remove | Tree already resolves to a patched version without the pin, or `pnpm why` reports nothing at all | Delete the entry, close its tracking issue |
| Replace | The package is a direct workspace dependency | Bump it (or its catalog entry) and delete the override |
| Keep | Still resolves to an affected version and no head dependency release fixes it | Keep the entry, ensure it carries a full justification comment |

---

## Step 1 - Locate the workspace root

```bash
dir=$(pwd); while [ "$dir" != "/" ]; do [ -f "$dir/pnpm-workspace.yaml" ] && echo "$dir" && break; dir=$(dirname "$dir"); done
```

All subsequent commands run from this root.

---

## Step 2 - Inventory every override declaration

Overrides hide in more places than the root file:

```bash
# Root workspace overrides (the ones pnpm actually honors)
grep -n -A200 '^overrides:' pnpm-workspace.yaml

# Per-package blocks: ignored by pnpm at install time, but honored by npm
grep -rn --include=package.json -E '"(overrides|resolutions)"' . | grep -v node_modules
```

A per-package `overrides` block in a workspace member is either dead weight or a sign that the package is also installed standalone with `npm` (samples are the usual case). Determine which before touching it, and do not delete one just because pnpm ignores it.

Build a table before changing anything, one row per entry:

| Override | Pinned to | Declared in | Justification present? |
|---|---|---|---|

Entries with no `# JUSTIFICATION:` comment are the priority. Nobody recorded why they exist, so they are the most likely to be stale and the most dangerous to guess at.

---

## Step 3 - Find out who actually pulls each one in

```bash
pnpm why <vulnerable-package> -r --depth 6 2>&1 | head -60
```

Three questions per entry:

- **Is it in the tree at all?** No output means the override is dead. Delete it, no further checking needed.
- **Is it a direct workspace dependency?** Then the override was the wrong mechanism. Bump the dependency or its `catalog:` entry and delete the override.
- **Which head dependency's range is the binding constraint?** That is the package that has to move before the pin becomes unnecessary, and it is what the justification comment must name.

---

## Step 4 - Verify staleness empirically

A pinned version number tells you nothing about what the tree would resolve to on its own. Test it.

Copy the manifest aside first. **Never `git stash`**, not even transiently: this working tree routinely carries a large amount of unrelated in progress work.

```bash
cp pnpm-workspace.yaml /tmp/pnpm-workspace.yaml.bak
```

Comment out every entry you suspect is stale in one pass, then re-resolve without installing:

```bash
pnpm install --lockfile-only
pnpm why <vulnerable-package> -r --depth 6 2>&1 | head -40
```

Compare what the tree now resolves to against the patched range from the advisory:

- **Resolves to a patched version on its own** - upstream caught up. Drop the entry permanently.
- **Still resolves to an affected version** - restore it and go to Step 5. Before you do, check whether a newer head dependency release would fix it, since bumping that is the real fix:

```bash
npm info <head-package> version
npm info <head-package>@latest dependencies --json 2>/dev/null | jq '.["<vulnerable-package>"] // "not direct"'
```

If a head dependency bump removes the need for the pin, prefer that and hand the major-version cases to `fix-npm-vulnerability` Step 6 for the testing checklist.

Restore from the backup if you need to start over:

```bash
cp /tmp/pnpm-workspace.yaml.bak pnpm-workspace.yaml
```

---

## Step 5 - Every surviving entry carries a full justification

Backfill the same comment shape `fix-npm-vulnerability` writes, so the next audit can check the condition mechanically rather than re-deriving it:

```yaml
  # JUSTIFICATION: Fixes <CVE/GHSA-slug>: <one line on the flaw>.
  # Added <YYYY-MM-DD>. Remove once <head-dep> ships a release containing <vulnerable-package>@<patchedVersions>.
  # Tracking: <github-issue-url>
  <vulnerable-package>: '<patchedVersion>'
```

A justification is incomplete unless it answers all four: which advisory, how the package enters the tree, what blocks the real fix, and the concrete condition for deletion. For a backfilled entry whose original advisory cannot be determined, say so explicitly in the comment rather than inventing one, and `WebSearch` the package and version range to recover the advisory ID where possible.

While you are in the entry, tighten it to the narrowest form that still covers the flaw, so it expires on its own:

- `<head-package>><vulnerable-package>` scopes the pin to one dependency path instead of the whole workspace.
- `<package>@<affected range>: '<patched>'` applies only to versions that are actually affected and becomes a no-op once nothing resolves into that range. The existing `vite@>=8.0.0 <=8.0.15` and `minimatch@<3.1.4` entries are the pattern to follow.

A bare `<package>: <version>` is the weakest form: it pins every consumer, including ones that were never affected. Prefer a scoped or range-qualified entry whenever the affected range can be determined.

JSON manifests cannot carry comments. For a sample app's `package.json` block that has to stay, record the rationale in the PR body and the tracking issue instead of inventing a JSON key for it.

Keep the file's existing grouping and do not reorder unrelated lines.

---

## Step 6 - Close out the tracking issues

Each override added through `fix-npm-vulnerability` has a tracking issue. For every entry removed in this run, close its issue with the reason:

```bash
gh issue list --search "Remove Pnpm Override" --state open --limit 50
gh issue close <number> --comment "Removed in <PR/commit>: <head-dep>@<version> now resolves <vulnerable-package>@<version>, so the override is no longer needed."
```

For entries that stay, leave the issue open and make sure the override comment still points at it.

---

## Step 7 - Verify

Entries were removed or narrowed in `pnpm-workspace.yaml`. `pnpm install` needs to run to regenerate
`pnpm-lock.yaml` and confirm the tree still resolves clean — but it re-resolves the whole workspace,
not just the touched entries, so it can shift unrelated transitive versions too. Ask before running it:

> The overrides file is updated. Running `pnpm install` now will regenerate `pnpm-lock.yaml` and let
> me verify nothing regressed — it can also shift unrelated transitive versions in the process. Shall
> I proceed?

Wait for confirmation, then run:

```bash
pnpm install
pnpm audit --audit-level=high
git diff --stat pnpm-lock.yaml
```

The lockfile diff tells you which workspaces actually moved. Build and lint those, because a removed override that shifts a build tool across a major version fails at build time, not install time.

Make sure `pnpm-lock.yaml` ends up staged/committed together with the `pnpm-workspace.yaml` edit —
don't leave the lockfile change uncommitted for a later, unrelated commit to pick up.

Report honestly: if `pnpm audit` still reports findings you deliberately left unpinned, name them and say why, rather than adding a pin to quiet the output.

---

## Step 8 - Final report

Output the table with the outcome and reason per entry:

| Override | Outcome | Reason |
|---|---|---|
| `flatted` | Removed | `nx` now depends on a patched range; tree resolves clean without the pin |
| `postcss` | Kept | Packed through `next@15.5.18`; needs next 16. Justification backfilled |
| `axios` | Replaced | Direct workspace dependency; bumped in the catalog instead |

Then state the count removed, kept, and replaced, and list the tracking issues closed.

---

## Cadence

An override added to close an advisory is usually removable within a release or two of the head dependency. Run this audit on a schedule, not only during vulnerability sweeps, otherwise the overrides block becomes a set of undocumented version pins that nobody can safely delete.
