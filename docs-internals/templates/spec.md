# <FEATURE> Specification

- **Status:** Draft
- **Version:** 0.1
- **Related documents:** <ISSUES, PROTOCOLS, THREAT MODEL, OR OTHER REFERENCES>

<Summary, Architecture, Detailed design, Requirements and acceptance criteria, and Change log are
required. Open questions and the subsections under Detailed design are conditional. Omit conditional sections that do not apply. Remove all template instructions before the specification is accepted.>

## Summary

<Explain the problem, the proposed solution, its scope, and the governing design decision. Keep this
section concise and do not repeat the detailed requirements.>

## Architecture

<Describe the components, ownership boundaries, and existing seams used by the feature.>

## Detailed design

<Use one subsection for each independent mechanism or responsibility. Describe its behavior, ownership,
validation, failure behavior, lifecycle, and rationale where relevant. Keep cross-cutting security analysis in the separate threat model, but state security constraints that directly affect the mechanism.>

### <SUBSECTION TITLE>

<Title each subsection after the mechanism or responsibility it describes. Structure the content in whatever way the design needs. Repeat this subsection
as required, ahead of the fixed subsections below.>

### Data model

<Conditional. Describe the schema changes: tables, columns, indexes, and migrations. State explicitly when
an existing table is reused. Omit this subsection when the feature has no database changes.>

### API

<Conditional. Describe endpoints, request and response models, authorization, validation, and error
responses. Omit this subsection when the feature has no API changes.>

### MCP exposure

<Required, and checked in CI. Decide whether AI agents get a ThunderID MCP tool for the feature. Write
one `Decision:` line with one of the values below, then what that value needs.

- `required`: a table with a row for every API operation the feature adds or changes. The tool is
  `thunderid_<snake_case>`, `not exposed: <reason>`, or `deferred <issue>`. The tier is `read`,
  `write` or `destructive`; a `not exposed` row may leave it empty. An operation backing several
  tools takes a row per tool, and a tool combining several operations a row per operation.
- `not required`: a `Reason:` line saying why no operation becomes a tool.
- `deferred`: an `Issue:` line holding `#<n>` or `https://github.com/thunder-id/thunderid/issues/<n>`.

For example:>

    Decision: required

    | Operation | Tool | Tier |
    |---|---|---|
    | GET /connections | thunderid_list_connections | read |
    | POST /connections/{id}/test | not exposed: runs a live request against the endpoint | |
    | DELETE /connections/{id} | deferred #1234 | destructive |

### UI

<Conditional. Include a mockup for every new or changed screen.
Omit this subsection when the feature has no user-facing changes.>

### Configuration

<Conditional. List configuration keys, defaults, validation, and whether each setting is deployment-level
or organization-level. Omit this subsection when the feature has no configuration changes.>

## Requirements

<Every requirement and acceptance criterion below must be covered by the preceding design. Do not keep a
requirement that is deferred or partially covered. Move unsupported requirements to a future specification.>

### R1. <REQUIREMENT TITLE>

**Requirement:** <CAPABILITY STATEMENT OR USER STORY.>

**Acceptance criteria:**

- **AC1.1:** Given <INITIAL STATE>, when <EVENT>, then <OBSERVABLE RESULT>.
- **AC1.2:** Given <INITIAL STATE>, when <EVENT>, then <OBSERVABLE RESULT>.

## Change log

| Version | Date | Change |
|---|---|---|
| 0.1 | YYYY-MM-DD | Initial specification. |
