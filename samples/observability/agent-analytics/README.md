# Agent Analytics Dashboards

Dashboard definitions for agent activity in ThunderID, built on the structured events the observability subsystem already publishes. They answer which agents are issuing tokens, whether issuance is healthy, which agents act on a person's behalf, and why issuance fails.

These are definitions only. ThunderID ships no analytics runtime, and nothing here is a deployment:you load a definition into an analytics product you already run.

**Start with the [Agent Analytics guide](../../../docs/content/deployment/agent-analytics.mdx).** It covers the event schema, how to get events into a pipeline, the transformations two of the panels require, and the query behind every panel written out so you can rebuild them anywhere.

## What is here

| Path | Product | How to load it |
|---|---|---|
| `grafana/agent-analytics.json` | Grafana Tempo | Provision it from a file, or import through **Dashboards, New, Import**. Expects a Tempo datasource with uid `thunderid-tempo` |
| `moesif/agent-analytics.json` | Moesif | Import through the interface. One file carries every panel and the dashboard |

## Panels

| Panel | Question it answers |
|---|---|
| Agent tokens issued            | How much work did agents do, and how much failed |
| Agent token issuance over time | Is agent workload rising, falling, or spiking |
| Most active agents             | Which agents carry the workload |
| Grant types                 | Which mechanism each agent uses to obtain access |
| Delegated against direct       | How much was performed on a person's behalf |
| Work performed for a person    | Whose work are agents doing |
| Issuance latency               | How long agents wait for access |
| Failure reasons                | Why agent work is failing |

## Before you load them

**Every token panel filters on three conditions together**, not one: the acting principal is an agent, the component is the authentication handler, and the outcome is success or failure. Dropping either of the last two silently roughly doubles every count, because flow events and the in-progress half of each issuance also carry an agent principal. The definitions here already apply all three.

**Three fields need reshaping in the collector first.** The delegation indicator is a boolean, which neither product groups on cleanly, so the collector republishes it as the string `Delegated` or `Direct`. Failure reasons needs the error object split into scalar fields. The guide gives the configuration for both.

**Latency needs an integer copy of the duration.** `data.duration_ms` is published as a string. The collector converts it with `Int()`; a `Double()` copy is a valid numeric attribute that the metrics functions silently return no series over.

**The Moesif definition needs no substitution**, but it does assume Moesif's own field names: attributes arrive under `metadata.*` with dots replaced by underscores, so `agent.acting_for_user` becomes `metadata.agent_acting_for_user.raw`. Time buckets are aligned to UTC; change `time_zone` in the time-series panels for local day boundaries. Both definitions read the same OpenTelemetry export, so one pipeline feeds both.

## Adapting them

Panels select agent traffic on the acting principal type and label agents by client identifier. To widen a panel to all traffic, drop that condition. The event schema and the shape of these queries transfer to Elasticsearch, OpenSearch, Splunk, Datadog, or anything else that ingests JSON, and the guide states each panel's question, source fields, and derivation in product-neutral terms for exactly that purpose.
