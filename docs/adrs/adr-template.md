---
# status is required. One of:
#   proposed | rejected | accepted | deprecated | superseded by ADR-0123
status: "proposed"
#
# Everything below is optional. Remove what you do not use.
# date: {YYYY-MM-DD when the decision was last updated}
# decision-makers: {GitHub handles of everyone involved in the decision}
# consulted: {handles of subject-matter experts consulted; two-way communication}
# informed: {handles of everyone kept up to date; one-way communication}
---

# {Short title, stating both the problem and the chosen solution}

## Context and Problem Statement

{Two or three sentences, or a short illustrative story.
State what forced the decision. Phrasing the problem as a question often works well.
Link the GitHub Discussion or issue where this was debated.}

<!-- Optional. Remove if not used. -->
## Decision Drivers

* {driver 1: a force, a constraint, a concern}
* {driver 2}

## Considered Options

* {option 1}
* {option 2}
* {option 3}

## Decision Outcome

Chosen option: "{option 1}", because {justification: it is the only option meeting a
knock-out criterion, or it resolves a specific force, or it comes out best against the
drivers above}.

<!-- Optional but strongly encouraged. Remove if not used. -->
### Consequences

* Good, because {positive consequence}
* Neutral, because {consequence that weighs neither way but is worth recording}
* Bad, because {negative consequence accepted as part of this decision}

<!-- Optional. Fill this in for any decision about protocol or spec behavior. -->
### Confirmation

{How will compliance with this decision be verified?
Name the conformance profile, test suite, integration test, or review gate that proves
the implementation matches this record. "Reviewed by the architecture group" is a weak
answer; an executable check is a strong one.}

<!-- Optional. Remove if not used. -->
## Pros and Cons of the Options

### {option 1}

{Optional description, example, or pointer to more information.}

* Good, because {argument}
* Neutral, because {argument}
* Bad, because {argument}

### {option 2}

{Optional description, example, or pointer to more information.}

* Good, because {argument}
* Bad, because {argument}

<!-- Optional. Remove if not used. -->
## More Information

{Links to the originating discussion, related ADRs, relevant specifications and RFCs.
Note here if this record was written retroactively.
Record when the decision should be revisited, if that is knowable.}
