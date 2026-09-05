# Internal Design Docs

This directory holds the design artifacts that back non-trivial changes to ThunderID: the
specification and the threat model for each feature area. These documents are written before
development starts and reviewed on their own, separate from the implementation.

**The workflow lives in the contributor guide:
[Propose a Design](https://thunderid.dev/community/contributing/propose-a-design).** Read
it first. It covers when the two documents are required, how the design discussion comes before them,
how the pull request is reviewed, and what happens after it is merged.

## Templates

- [templates/spec.md](templates/spec.md) defines the feature: summary, architecture, detailed design,
  requirements, and acceptance criteria.
- [templates/threat-model.md](templates/threat-model.md) defines the security posture: trust
  boundaries, actors, interactions, threat assessment, and the security review checklist.

## Layout

One directory per feature, named after the feature area in kebab-case:

```
docs-internals/<feature-name>/
├── spec.md
├── threat-model.md
└── assets/           # mockups and diagrams referenced by the two documents
```

Reference images with a relative path, for example `assets/login-screen.png`.

## Before you submit

- Every template instruction and placeholder has been removed or filled in.
- Conditional sections that do not apply (data model, API, UI, configuration) have been omitted
  rather than left empty.
- Every requirement and acceptance criterion is covered by the design, with anything deferred moved
  out to a future specification.
- The threat model records no exploitable, unmitigated threat. Route those to a private
  [GitHub Security Advisory](https://github.com/thunder-id/thunderid/security/advisories) and keep
  only a bounded residual here.
