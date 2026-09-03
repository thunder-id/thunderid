# Internal Design Docs

This directory holds the design artifacts that back non-trivial changes to ThunderID: the
specification and the threat model for each feature area. These documents are written before
development starts and reviewed on their own, separate from the implementation.

Templates live in [templates/](templates):

- [templates/spec.md](templates/spec.md) defines the feature: summary, architecture, detailed design,
  requirements, and acceptance criteria.
- [templates/threat-model.md](templates/threat-model.md) defines the security posture: trust
  boundaries, actors, interactions, threat assessment, and the security review checklist.

## When these documents are required

A committed `spec.md` and `threat-model.md` are required before development begins for:

- a refined feature issue (`Type/NewFeature`),
- a major improvement (`Type/Improvement`) that changes behavior, APIs, schema, or trust boundaries,
- a refactoring that moves ownership boundaries between components or changes an existing seam.

They are not required for bug fixes, documentation changes, dependency updates, test-only changes, or
improvements that stay inside a single component without changing its contract.

If you are unsure which side of the line your change falls on, open the design discussion first and
ask. A maintainer will tell you whether the documents are needed.

## Workflow

### 1. Agree on the high-level design first

Open a [Design Discussion](https://github.com/thunder-id/thunderid/discussions/new?category=design) using the design
template and link the refined feature or improvement issue. Propose the high-level approach:
architecture, impacted areas, security considerations, alternatives you rejected, and the open
questions you want input on.

Do not open a specification PR yet. The discussion exists to settle direction cheaply, before anyone
spends effort on a detailed design. Iterate there until maintainers and reviewers agree on the
approach.

### 2. Submit the specification and threat model as a PR

Once the high-level design is agreed, create a feature directory under `docs-internals/` and fill in
both templates:

```
docs-internals/<feature-name>/
├── spec.md
├── threat-model.md
└── assets/           # mockups and diagrams referenced by the two documents
```

Use a short kebab-case directory name that matches the feature area, for example
`docs-internals/ldap-user-sync/`. Reference images with a relative path, for example
`assets/login-screen.png`.

Open a single PR containing only these documents, and link both the feature issue and the design
discussion in the PR description.

Before submitting, check that:

- every template instruction and placeholder has been removed or filled in,
- conditional sections that do not apply (data model, API, UI, configuration) have been omitted
  rather than left empty,
- every requirement and acceptance criterion is covered by the design, with anything deferred moved
  out to a future specification,
- the threat model records no exploitable, unmitigated threat. Route those to a private
  [GitHub Security Advisory](https://github.com/thunder-id/thunderid/security/advisories) and keep
  only a bounded residual here.

### 3. Review

Minor issues are resolved in the PR: unclear wording, a missing acceptance criterion, a threat that
needs a stated mitigation, a subsection in the wrong place. Discuss and address them with review
comments, and keep the PR open.

Major concerns send the design back a step. If review shows the agreed approach does not hold, for
example the architecture is wrong for the problem, the scope is misjudged, or a trust boundary cannot
be defended, the PR is closed and the conversation returns to the design discussion. Reopen a new
specification PR once the discussion converges again.

### 4. Start development

Development starts after the specification PR is merged. Implementation PRs should reference the
merged specification, and the acceptance criteria in `spec.md` are what the tests are written
against.

## Keeping the documents current

The specification and threat model describe the feature as built, not only as planned. When
implementation forces a design change, update the documents in the same PR that changes the behavior
and add a row to the change log table in `spec.md`. If the change alters a trust boundary, an actor's
entitlements, or an interaction, update `threat-model.md` alongside it.
