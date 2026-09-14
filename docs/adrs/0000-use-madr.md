---
status: "accepted"
date: "2026-09-01"
decision-makers: "@madurangasiriwardena @darshanasbg"
---

# Use MADR for architectural decision records

## Context and Problem Statement

ThunderID has no decision log, in the sense of a maintained collection of records that each capture one architectural decision (AD) and its rationale.
Design reasoning currently lives in GitHub Discussions, pull request threads, and, for decisions inherited from earlier work, internal mail threads that external contributors cannot read at all.

This has three costs that grow as the project does.
Contributors cannot tell whether an approach was already considered and rejected, so the same proposals resurface.
Decisions that were deliberate deviations from a specification are indistinguishable from oversights, which matters disproportionately for software in the identity domain.
And the reasoning behind a design is reconstructed from a thread whose conclusion is often implicit, if it survives at all.

The project is governed under the Open Wallet Foundation, and external contributors build production systems on it.
Those contributors need the same access to design rationale that maintainers have.

How should decisions that address an architecturally significant requirement (ASR) be recorded, so that the reasoning survives the thread it was formed in?

## Decision Drivers

* Records must be readable and writable by external contributors with no access to internal systems.
* The format must add little enough friction that it is actually used under delivery pressure.
* Records must be reviewable through the mechanism the project already uses for changes: pull requests.
* Decision history must be versioned alongside the code it describes.
* The scheme must survive the project outliving any individual maintainer.

## Considered Options

* Continue with GitHub Discussions only
* Michael Nygard's original ADR format
* MADR 4.0.0
* Y-statements
* A wiki or Confluence space

## Decision Outcome

Chosen option: "MADR 4.0.0", because it is the only option that keeps records in the repository under normal pull request review, offers enough structure to make rejected options explicit, and remains lean enough to write in one sitting.

### Consequences

* Good, because decision history is versioned with the code and travels with any fork or clone of the repository.
* Good, because proposing a decision uses the same pull request mechanism external contributors already use, giving them a legitimate path to influence architecture rather than only implementation.
* Good, because the log is complete with respect to the code in this repository: a decision affecting code here is recorded here, so a reader of the repository is never missing rationale that exists elsewhere.
* Good, because the structure forces rejected options to be written down, which is the part reconstruction from a thread always loses.
* Neutral, because MADR is a template rather than a tool; no tooling dependency is added, and none is provided either.
* Bad, because writing a record is real work that competes with delivery, and the practice will decay unless the trigger conditions and review integration in `README.md` are enforced.
* Bad, because a partially adopted log is arguably worse than none. Readers may reasonably infer that an undocumented decision was never deliberately made.

### Confirmation

Adoption is enforced by three mechanisms rather than by intent:

* A `CODEOWNERS` entry on `docs/adrs/` requiring architectural review on every record.
* A checkbox in the pull request template asking whether the change requires an ADR.
* An `adr-lint` workflow that fails on malformed records and on records missing from the index.

Whether the log is actually being *read* is confirmed by a softer signal: architectural objections in code review citing ADR numbers.
If that has not started happening within two release cycles, this decision should be revisited rather than quietly ignored.

## Pros and Cons of the Options

### Continue with GitHub Discussions only

* Good, because it costs nothing and is already in use.
* Good, because it captures the full debate, including the arguments that were abandoned.
* Neutral, because discussions are public and therefore already accessible to external contributors.
* Bad, because a discussion has no canonical conclusion. The outcome is implicit in the last few comments, or absent.
* Bad, because there is no status, so a superseded decision looks identical to a current one.
* Bad, because discussions are not versioned with the code and do not travel with a fork.

### Michael Nygard's original ADR format

* Good, because it is the most widely recognized ADR format and the origin of the term.
* Good, because it is extremely short: context, decision, status, consequences.
* Bad, because it has no dedicated section for options considered, which is the section that most often prevents a settled question from being reopened.
* Neutral, because MADR is a direct descendant and the two are close enough that migration in either direction is mechanical.

### MADR 4.0.0

* Good, because only four sections are mandatory, so a minor record can be fifteen lines.
* Good, because "Pros and Cons of the Options" makes the comparison explicit, with Good, Neutral and Bad prefixes.
* Good, because the "Confirmation" subsection asks how compliance will be verified, which maps directly onto conformance suites for protocol decisions.
* Good, because it ships markdownlint configuration and templates in four levels of verbosity.
* Neutral, because the format has changed across major versions, with section names differing between 3.x and 4.x, so records copied from other projects may need adjusting.
* Bad, because the fuller template is verbose enough that contributors may skip it entirely rather than delete the optional sections; the minimal template variants mitigate this.

### Y-statements

* Good, because a decision compresses to a single structured sentence, making it very cheap to write.
* Bad, because that compression discards the option comparison and the consequences, which is most of the value for a long-lived project.
* Neutral, because a Y-statement works well as a summary line *inside* a MADR record.

### A wiki or Confluence space

* Good, because it is easy to write in and easy to reorganize.
* Bad, because pages are mutable by default, so there is no reliable history of what was decided when.
* Bad, because it is not reviewable through pull requests.
* Bad, because access for external contributors is an administrative problem that recurs with every new contributor, and is fundamentally at odds with an Open Wallet Foundation project.

## More Information

* Architectural decision terminology, including AD and ASR: <https://adr.github.io/>
* MADR project: <https://adr.github.io/madr/>
* MADR 4.0.0 release: <https://github.com/adr/madr/releases/tag/4.0.0>
* Nygard's original article: <https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions>
* Trigger conditions, process, and repository integration: [README.md](README.md)

This decision should be revisited if the log falls out of use, or if the project adopts documentation tooling that makes a different format cheaper to maintain.
