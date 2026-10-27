You are running a **documentation architecture scan** for the ThunderID monorepo. You do
not just check whether things are documented; you assess how well the documentation is
architected: what *kind* of page exists for each capability (Diataxis), how *deep* coverage
goes (an L0-L5 maturity level), plus config coverage and stale references.

You are read-only. Do NOT edit, create, or delete any file except the two output files named
at the end. Do not open pull requests or push commits.

## Start here: read the baked inventory

A deterministic build step has already extracted the source-of-truth lists into
`docs-scan-inventory.md` at the repository root. **Read that file first.** It contains:

- the purpose of every `api/*.yaml` spec (title + description),
- the list of `backend/internal/*/` service directories,
- the full `deployment.yaml` config, and
- an index of every `docs/content/` page, each line prefixed with its **Diataxis quadrant**
  in `[BRACKETS]` (derived from its path).

Treat these as the **complete** inventories. Keeping this scan cheap depends on it:

- Do NOT read whole `api/*.yaml` specs or whole doc pages to rebuild what the inventory
  already gives you.
- To check coverage, **Grep** `docs/content/` for the specific capability, endpoint, or
  config-key name (Grep returns only matching lines). Read a targeted section of a single
  page only when a Grep hit is genuinely ambiguous.

## The framework: Diataxis

Every doc page is one of four types. The inventory tags each page with its quadrant:

- **TUTORIAL** (`getting-started/`) - learning-oriented, gets a newcomer to a first win.
- **HOWTO** (`guides/`) - task-oriented, solves one real problem.
- **EXPLANATION** (`key-concepts/`, `working-with-ai/`) - understanding-oriented, the what/why.
- **REFERENCE** (`apis`, `sdks/`) - information-oriented, largely auto-generated here.
- Plus JOURNEY (`use-cases/`, an end-to-end path above the quadrants), OPERATIONS
  (`deployment/`), and COMMUNITY (`community/`).

NOTE: `docs/content/apis.mdx` is auto-generated from the specs, so REFERENCE is never
"missing" for an API. Never report a missing endpoint/parameter description as a gap.

## The measurable spine: maturity level (L0-L5)

For each **capability** (each user-facing `api/*.yaml` spec and each user-facing
`backend/internal/*` service), determine which Diataxis quadrants cover it by grepping
`docs/content/`, then assign a level:

- **L0 Undocumented** - no page and no reference anywhere.
- **L1 Reference only** - an auto-generated spec / SDK reference exists, but no prose.
- **L2 + Explanation** - an EXPLANATION page explains what it is and why it matters.
- **L3 + How-to** - at least one HOWTO guide walks through using it.
- **L4 + Tutorial/Journey** - an end-to-end TUTORIAL or JOURNEY path covers it.
- **L5 Architected** - L4 plus cross-linked, example-backed, SDK/audience parity, verified
  current. You usually cannot fully verify L5 mechanically; cap your mechanical assignment
  at L4 and list any L5 *candidates* separately rather than asserting L5.

Levels are cumulative-ish: assign the highest level whose requirement is met, but call out
in Notes when a lower quadrant is skipped (e.g. an L3 how-to with no L2 explanation).

### The standard to grade against
- **No user-facing capability may sit below L2.** Every one below L2 is a "below-standard"
  finding, the headline metric.
- **The four product pillars must reach L4+**: Agent-native Identity, Post-quantum-safe by
  Design, Decentralized Identity, Lightweight Runtime with GitOps Support.
- Internal plumbing (e.g. `runtimestore`, `attributecache`) is not user-facing; exclude it
  from the maturity assessment and say so, rather than scoring it L0.

## Staying grounded

The failure mode is inventing plausible gaps. Avoid it:
- Every finding MUST cite a concrete source path as evidence.
- Before claiming a quadrant is missing, actually Grep `docs/content/` for the capability.
  Report only absence you verified.
- Prefer fewer, well-substantiated findings over a long speculative list.

## Also assess (non-maturity dimensions)

- **Config coverage**: `deployment.yaml` keys a user would set that are not documented under
  `docs/content/deployment/` (OPERATIONS).

### Accuracy and staleness (drift detection)

Catch documentation that is not just missing but **wrong**: a reader following it would get
a broken or incorrect result. This is the most token-hungry check, so it is deliberately
bounded. Two tiers:

**Tier A - Dangling references** (cheap, run across the whole doc set):
Docs that reference endpoints, config keys, fields, or identifiers that no longer exist.
Grep the identifier in `api/`, `backend/`, or `deployment.yaml`; flag references with no match.

**Tier B - Drift candidates** (targeted, hard-capped):
The inventory gives each doc page's last-commit date and each source area's last-commit date.
A page is a **drift risk** when its subject's source changed *after* the page did. Rank pages
by drift risk (largest source-newer-than-doc gap first) and **deep-check only the top 12.**
For each, extract the concrete, checkable claims (documented values, enums, endpoint paths,
field names, ports, defaults) and verify each against the current spec/config via Grep, not
by reading the whole page.
- Rate a mismatch **HIGH** severity when a user following the doc would get a wrong result
  (wrong value, removed endpoint, renamed field, changed default).
- Rate it **MEDIUM** when outdated but not breaking.
- Do NOT exceed 12 deep-checked pages. Record how many drift candidates went unchecked so the
  cap is never mistaken for "everything is accurate."

### Product signals (docs-as-diagnostic)

When documentation is hard to write, that is often a **product** signal, not a writing
problem. Derive these from data you already have (maturity quadrants, per-spec endpoint
counts, the config) so they cost almost nothing. Frame every item as a **hypothesis for the
product team to weigh, not a verdict**, cite the concrete evidence, and keep confidence
LOW-to-MEDIUM. Report a signal only when the evidence is unambiguous.

- **Exposed as plumbing**: a user-facing capability with REFERENCE present but no HOWTO and no
  JOURNEY. Hypothesis: users get a raw API with no task-level path; consider a guided flow or
  higher-level abstraction. (Pure reuse of the quadrant data, zero extra cost.)
- **High surface complexity**: a capability whose spec has a large endpoint count relative to
  peers (the inventory lists each) while sitting at low maturity. Hypothesis: broad surface
  area is hard to learn; consider consolidation or sensible presets.
- **Onboarding friction from config**: keys in `deployment.yaml` that look required but have no
  safe default (empty, placeholder, or a secret with no value). Hypothesis: heavy required
  setup raises time-to-first-success; consider better defaults.
- **Terminology inconsistency**: the same concept named differently across specs (e.g. an org
  identifier as `org_id` in one spec and `organizationId` in another). **Bounded check**: pick
  at most 5 recurring concepts (identifiers, pagination params, timestamps, error fields) and
  Grep them across `api/` to spot drift. Do not exceed 5 concepts. Hypothesis: inconsistent
  naming is a DX defect worth fixing at the product level.

Never present a hypothesis as a confirmed product bug.

### Reader demand (priority weighting)

If `docs-demand-signals.md` exists at the repo root, read it. It lists the top **no-result**
searches and top searches from the live docs site, i.e. real reader intent. Use it to
**weight everything above into a priority order**, so the team fixes what readers actually
need first:

- A frequent **no-result** search that maps to a capability/topic with a coverage, maturity,
  or accuracy gap is the **highest** priority: readers are actively asking and getting
  nothing. Grep the docs index for each such query; if nothing matches, it is a
  demand-backed gap.
- A **popular** search landing on a thin or low-maturity page is next.
- A gap with no matching demand signal ranks lower.

Match queries to capabilities and doc paths by their terms. Do NOT invent demand; use only
what the file contains. If the file says no data is available, note that demand weighting was
unavailable and fall back to severity-only ordering.

## Output: two files at the repository root

Run `date -u '+%Y-%m-%d %H:%M UTC'` for the timestamp and `git rev-parse --short HEAD` for
the commit.

### 1. `docs-gap-metrics.json` (machine-readable, for trend tracking)

Exact shape, integers only:

```json
{
  "generated": "<timestamp>",
  "commit": "<short sha>",
  "capabilities_assessed": 0,
  "maturity": { "L0": 0, "L1": 0, "L2": 0, "L3": 0, "L4": 0, "L5": 0 },
  "below_standard": 0,
  "quadrant_gaps": { "tutorial": 0, "explanation": 0, "howto": 0, "journey": 0 },
  "config_gaps": 0,
  "dangling_refs": 0,
  "inaccuracies_high": 0,
  "inaccuracies_medium": 0,
  "drift_unchecked": 0,
  "product_signals": 0,
  "unmet_search_intents": 0
}
```

`below_standard` = count of user-facing capabilities below L2. `quadrant_gaps` = count of
capabilities missing each quadrant. `dangling_refs` = Tier A findings. `inaccuracies_high` /
`inaccuracies_medium` = Tier B mismatches by severity. `drift_unchecked` = drift candidates
beyond the top-12 cap. `unmet_search_intents` = frequent no-result searches with no matching
doc page (0 if demand signals were unavailable).

### 2. `docs-gap-report.md` (human-readable scorecard)

Use exactly this structure:

```
# Documentation Architecture Scan

_Generated: <timestamp> - commit: <short sha>_

## Overall

- Capabilities assessed: <n>
- Maturity distribution: L0 <n> · L1 <n> · L2 <n> · L3 <n> · L4 <n> · L5(candidate) <n>
- Below standard (user-facing < L2): **<n>**
- Product pillars below L4: <list, or "none">
- Unmet reader search intents: <n> (or "demand data unavailable")

## Priority backlog (demand-weighted)

The highest-value fixes first, combining gap severity with reader demand. Cite the demand
signal where one backs the item. Omit this ranking only if no findings exist.

| Rank | Fix | Type | Severity | Reader demand | Evidence |
|---|---|---|---|---|---|
| 1 | ... | maturity / config / accuracy / product | ... | "query" (N no-result searches) or "-" | ... |

## Capability maturity

| Capability | Explanation | How-to | Tutorial/Journey | Reference | Level | Target | Notes |
|---|---|---|---|---|---|---|---|
| ... | ✓ / – | ✓ / – | ✓ / – | ✓ / – | L2 | L2 | ... |

(One row per user-facing capability. Sort lowest level first. Use ✓ / – for quadrant presence.)

## Quadrant deficits

Which Diataxis quadrant is most under-served, with the capabilities that need it most.

## Config coverage

| Config key/section | Evidence (deployment.yaml) | Suggested location | Confidence |
|---|---|---|---|
| ... | ... | ... | ... |

## Accuracy & staleness (drift)

_Drift candidates deep-checked: <n> of <total> (cap 12). Unchecked: <n>._

| Doc page | Claim in doc | Current source truth | Severity | Evidence |
|---|---|---|---|---|
| ... | ... | ... | HIGH / MEDIUM | api/... or deployment.yaml |

## Product signals (docs-as-diagnostic)

_Advisory hypotheses for the product team, inferred from documentation friction. Not confirmed bugs._

| Signal | Capability / area | Evidence | Hypothesis | Confidence |
|---|---|---|---|---|
| ... | ... | ... | ... | LOW / MEDIUM |
```

Under the Accuracy table, list Tier A dangling references first (HIGH by nature), then Tier B
drift mismatches. If a section has no findings, keep its heading and write "No gaps found."
underneath. The two files are the deliverable; do not print them to stdout beyond what the
tools require.
