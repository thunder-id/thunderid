---
# status is required. One of:
#   proposed | rejected | accepted | deprecated | superseded by ADR-0123
status: "proposed"
#
# supersedes is required when this record replaces another.
# supersedes: "ADR-0123"
#
# Everything below is optional. Remove what you do not use.
# date: {YYYY-MM-DD when the decision was last updated}
# decision-makers: {GitHub handles of everyone involved in the decision}
---

# {Short title, stating both the problem and the chosen solution}

## Context and Problem Statement

{Two or three sentences stating what forced the decision.
Phrasing the problem as a question often works well.
Link the GitHub Discussion or issue where this was debated.}

## Considered Options

* {option 1}
* {option 2}
* {option 3}

## Decision Outcome

Chosen option: "{option 1}", because {justification}.

<!-- Optional, except when this record is deprecated. Remove if not used. -->
## More Information

{Links to the originating discussion and related records.
A deprecated record says here why it was deprecated, because no successor record carries that reasoning.}
