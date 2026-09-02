# Architectural Decision Records

This directory holds the architectural decision log for ThunderID.

The format is [MADR 4.0.0](https://adr.github.io/madr/).
See [ADR-0000](0000-use-madr.md) for why.

## Index

<!-- Keep this table sorted by number. Add a row in the same PR that adds the ADR.
     The Status column must match the record's own front matter; the ADR lint checks this. -->

| ADR | Title | Status | Date |
| --- | --- | --- | --- |
| [0000](0000-use-madr.md) | Use MADR for architectural decision records | accepted | 2026-09-01 |

## Terminology

These definitions come from [adr.github.io](https://adr.github.io/).

* An **architectural decision (AD)** is a justified design choice that addresses a functional or non-functional requirement that is architecturally significant.
* An **architecturally significant requirement (ASR)** is a requirement that has a measurable effect on the architecture and quality of a system.
* An **architectural decision record (ADR)** captures a single AD and its rationale.
* The collection of ADRs maintained in a project constitutes its **decision log**, which is what this directory is.

Two rules follow from those definitions, and they are the two people get wrong most often.
A record captures one AD, which is why [Granularity](#granularity) insists on one decision per record.
A record is warranted only when the requirement behind the decision is an ASR, which is what the next section tests for.

## When an ADR is required

An ADR is warranted when the requirement behind the change is architecturally significant.
That is hard to judge in the abstract, so the list below is the working test for it.
It follows [Nygard's original categories](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions), which are structure, non-functional characteristics, dependencies, interfaces, and construction techniques, narrowed to the ones that recur in this project.

Write an ADR when a change does any of the following:

* Alters a **wire-visible contract**: token or claim shape, endpoint semantics, error responses, or the choice of a spec profile.
* Introduces a **deliberate deviation** from a specification ThunderID claims to implement, including a decision not to implement an optional feature.
* Changes the **shape of the system**: a new or removed component, a responsibility that changes hands, or a pattern other components are expected to follow, such as an extension point, a flow model, or a concurrency or transaction convention.
* Trades off a **quality attribute** across the product: a security posture, a latency or throughput budget, an availability target, or what an operator has to do to run it.
* Changes a **storage strategy that spans components**: the split of data across stores, an encryption boundary, or a retention and revocation model.
* Adds, replaces, or removes a **runtime dependency** or an external system ThunderID talks to.
* Changes how the product is **built, packaged, or deployed** in a way that operators or downstream consumers have to follow.

Two questions settle most of the cases the list does not.
Would reversing this decision later be expensive, and does it rule out an alternative someone might reasonably propose again?
A choice that is cheap to reverse, or that is confined to a single component and binds nobody outside it, is not an ADR however much debate it took.

Everything else is a pull request description.
A bug fix or a refactor that preserves behavior does not need a record.

If you are unsure, open the discussion first.
The need for an ADR usually becomes obvious once someone disagrees.

### Granularity

One decision per record.
A related cluster of decisions becomes a small set of ADRs that cross-reference each other, not one large one.

A useful test is whether the choices could be reversed independently.
If they could, they are separate records that cite each other.
A choice that follows automatically from one already made, such as the column type used to store a structure whose storage split is already decided, belongs under that record's Consequences rather than in a record of its own.

### Scope

This log covers decisions affecting code in this repository.

A decision that changes code here is recorded here, whatever motivated it.
Records state the constraint being satisfied, not who asked for it.
An ADR naming a specific organization, customer or downstream project is usually recording a circumstance rather than a reason, and circumstances change while the design they produced does not.

Decisions confined to a downstream distribution's own tooling are that distribution's to record, and are out of scope here.
If a decision seems to need recording in two places at once, that is worth raising rather than resolving by splitting the record.

This section is deliberately not an ADR.
It describes where records go rather than making an architectural choice, and it needs to stay editable as the project's distribution arrangements change.

## Process

1. **Discuss.** Design debate happens in a GitHub Discussion or an issue, as it does today. An ADR is the distillation of that debate, not a transcript of it.
2. **Propose.** Open a pull request adding `NNNN-title-with-dashes.md` with `status: "proposed"`, using the next free number. Link the discussion in the `## More Information` section.
3. **Review.** Reviewers argue in the PR. Amend the record until it reflects what was actually decided, including options that were rejected and why.
4. **Accept.** Change `status:` to `"accepted"`, set `date:`, add the row to the index table, and merge. **The merge is the decision.**

A record that is rejected is still merged, with `status: "rejected"`.
Knowing that an option was considered and turned down is as useful as knowing what was chosen, and a rejected record stops the same proposal arriving again in six months.

### Accepted records are immutable

Do not edit the body of an accepted ADR.
Typos and broken links are fine to fix; reasoning and outcomes are not.

To change a decision, write a new ADR and then edit exactly one line in the old one:

```yaml
status: "superseded by ADR-0014"
```

The status field carries the identifier only, not a link. This is a MADR 4.0 convention.

If in-place editing is allowed, the log stops being a decision history and becomes a design document that silently drifts away from what was actually agreed.

### Numbering

Numbers are allocated in the order records are **proposed**, not accepted, and are never reused.
Two open PRs claiming the same number is a trivial rebase; two merged ADRs sharing a number is not.

The log is flat.
MADR supports categorizing records into subdirectories, but that makes numbers unique only within a category, which breaks cross-references and any number an external contributor has already cited.
Revisit this only if the log becomes genuinely hard to navigate.

## Writing a record

Copy [`adr-template.md`](adr-template.md) to `NNNN-title-with-dashes.md`.

Four sections are mandatory: the title, Context and Problem Statement, Considered Options, and Decision Outcome.
A `status` field in the front matter is mandatory too, because the lint rejects a record without one.
Everything else in the template is optional, so delete the sections you do not need rather than filling them with filler.

A few conventions that keep diffs readable and the linter quiet:

* One sentence per line. Reviewers can then comment on a single sentence and rewording produces a one-line diff.
* Asterisks (`*`) as list markers.
* Titles state both the problem and the chosen solution: "Store refresh tokens hashed rather than encrypted", not "Refresh token storage".

### Confirmation

The `### Confirmation` subsection is optional in MADR but should be filled in for anything spec-related.
For a decision about protocol behavior, name the conformance profile or test suite that proves the implementation matches the record.
This is what stops an ADR from decaying into an aspiration.

## Backfill

The log starts mostly empty, and reconstructing every past decision is not worth the effort.
Backfill only where a newcomer currently has to read a mail thread or a GitHub Discussion to understand why the code looks the way it does.

Backfilled records carry the original decision date and say plainly, in `## More Information`, that they were written after the fact.
Their "Considered Options" sections will be thinner than a live record's, and that is honest rather than a defect.

## Repository integration

[`.github/CODEOWNERS`](../../.github/CODEOWNERS) carries an entry for this directory, so architectural review is enforced by the platform rather than by memory:

```text
/docs/adrs/ @darshanasbg @madurangasiriwardena
```

[`.github/pull_request_template.md`](../../.github/pull_request_template.md) carries two additions: a `### Related ADRs` section alongside the existing Related Issues and Related PRs sections, and a checklist item.

The checklist item is phrased as *checked whether* an ADR is required rather than *wrote an ADR*, so a contributor whose change needs no record can still tick it truthfully.
A box that can only be ticked by some contributors is a box everyone learns to ignore.

[`.github/workflows/adr-lint.yml`](../../.github/workflows/adr-lint.yml) runs on any change under this directory.
It checks formatting with markdownlint, using the configuration in [`.markdownlint.yml`](.markdownlint.yml), and then enforces the conventions on this page: the `NNNN-title-with-dashes.md` filename, a `status` holding one of the recognized values, an index row whose Status column matches the record's front matter, and unique record numbers.

## Reviewing against the log

The log only pays for itself if it is read.
The mechanism is review: when objecting to an architectural choice in a pull request, cite the ADR number that the change contradicts, or say that no record covers it and one is needed.

An objection grounded in a merged record is a different conversation from an objection grounded in a reviewer's preference, and that difference matters most with contributors who are not in your timezone or your organization.
