# Agent Onboarding Flow Specification

- **Status:** Final
- **Version:** 0.1

## Summary

Agent onboarding in ThunderID is currently a fixed sequence embedded in the Admin Console. The console decides what information is collected, the order of the steps, and how the resulting agent credentials are presented. Because the sequence lives in the console, organizations cannot adapt onboarding to their own security, governance, or operational requirements, and any change to the sequence requires a product change and a console release.

This specification moves agent onboarding to a configurable flow that is resolved and executed at runtime, matching the model ThunderID already uses for other administrator-driven operations such as user onboarding and user deletion. A default Agent Onboarding flow ships so the feature works out of the box, and organizations customize the experience through the existing flow management capabilities.

The governing design decision is to implement agent onboarding as an Administrator Flow that reuses the existing flow model, rather than introducing a new flow type or keeping a hardcoded console sequence. The flow orchestrates the onboarding experience, while the existing agent management service stays authoritative for validation, agent creation, credential generation, and inbound client configuration.

## Architecture

The feature is built on the existing flow infrastructure and the existing agent management layer. No new flow type is introduced.

The participating components and their ownership boundaries are:

- **Admin Console.** Resolves the configured flow handle and executes the resolved flow. It no longer owns the onboarding sequence or a completion screen.
- **Configuration.** Holds the flow handle that maps agent onboarding to a flow definition, so the default flow can be replaced without a console change.
- **Flow Store.** Stores and returns the administrator flow definition for the resolved handle.
- **Flow Engine.** Provides sequencing, prompting, conditional execution, authorization, and runtime data. It executes the onboarding flow and carries generated credentials as runtime data.
- **Provisioning step and providers.** A generalized provisioning step resolves a provisioning mode and delegates creation to the correct provider. The entity provider handles user provisioning; a new agent provider handles agent creation.
- **Agent Management Service (and Agent API).** Remains the authoritative path for agent validation, agent record creation, credential generation, and inbound client configuration.
- **Agent type schema.** Continues to determine which agent attributes can be collected and stored.

The onboarding flow is resolved as an administrator flow at runtime:

```mermaid
sequenceDiagram
    participant Console
    participant Configuration
    participant FlowStore as Flow Store
    participant FlowEngine as Flow Engine

    Console->>Configuration: resolve flow handle
    Configuration-->>Console: configured handle
    Console->>FlowStore: resolve administration flow
    FlowStore-->>Console: flow definition
    Console->>FlowEngine: execute flow
```

## Detailed design

### Flow resolution and execution

Agent onboarding is executed through a flow definition resolved at runtime. The console reads a configurable flow handle from configuration, resolves the matching administrator flow definition from the flow store, and hands execution to the flow engine. Because the handle is configurable, the default onboarding flow can be replaced without changing the console implementation. This mirrors the resolution pattern already used by other administrator-driven operations.

Ownership: the console owns only resolution and execution kickoff. All sequencing, prompting, and conditional logic belong to the flow engine and the flow definition.

### Default Agent Onboarding flow

A default Agent Onboarding flow provides the baseline experience so the feature works without configuration. Organizations can add, remove, reorder, or branch from these steps through flow configuration.

```mermaid
flowchart LR
    START([START]) --> Perm["Permission Check"] --> Type["Agent Type Selection"] --> OU["Organization Unit"] --> Owner["Owner Resolution"] --> Attr["Attribute Collection"] --> Prov["Agent Provisioning"] --> Cred["Credential Presentation"] --> END([END])
```

The default steps are a starting point, not a fixed contract. The only invariants are that provisioning delegates to the agent management service and that credential presentation happens within the flow.

### Owner resolution executor

A new executor resolves the agent owner during the flow. It runs as a flow step and makes the resolved owner available as runtime data for later steps, including provisioning. Keeping owner resolution in its own executor lets organizations reorder or replace it without affecting provisioning logic.


### Provisioning delegation and the agent provider

The provisioning step is generalized so that it provides the common orchestration for collecting the required information while delegating the actual creation to a provider selected by a resolved provisioning mode. The user path delegates to the entity provider, and the agent path delegates to a new agent provider. The agent provider connects to the existing agent management service, which performs validation, creates the agent record, generates credentials, and configures the inbound client.

```mermaid
flowchart LR
    Step["Provisioning step"]
    Mode{"resolve<br/>provisioning mode"}
    Entity["Entity provider"]
    Agent["Agent provider"]
    EntitySvc["entity and entity type<br/>management services"]
    AgentSvc["agent management<br/>service"]
    Rules["validation<br/>agent record<br/>credential generation<br/>inbound client configuration"]

    Step --> Mode
    Mode -->|"user"| Entity
    Mode -->|"agent"| Agent
    Entity --> EntitySvc
    Agent --> AgentSvc
    AgentSvc --> Rules
```

Ownership: the flow owns orchestration and information collection. Creation rules stay entirely in the agent management service and are not duplicated in the flow.

### Credential presentation and completion

After provisioning, the agent provider ensures the newly created agent has the required authentication configuration and returns the generated credentials as flow runtime data. The final flow step presents those credentials to the administrator. Credential presentation is a flow step, not a hardcoded console completion screen.

```mermaid
flowchart LR
    Prov["Agent Provisioning"] --> AgentCred["Agent and Credentials"] --> Runtime["Flow Runtime Data"] --> Present["Credential Presentation"]
```

### Failure handling and compensation

Onboarding must not leave behind a partially configured or unusable agent, and it must not leave an unintended live credential.

Failures that occur during the creation operation itself continue to be handled by the agent management service, which is responsible for not producing a partial agent record.

### Data model

No new schema is introduced by this feature. Agent records, credentials, and inbound client configuration continue to be owned and stored by the existing agent management service. The default Agent Onboarding flow is provided as a flow definition through bootstrap file.

### API

No new external agent API is introduced. The existing Agent API and agent management service remain the authoritative path for agent validation and creation, and the agent provider calls that existing path. Flow execution uses the existing flow engine interfaces.

### UI

This feature changes user-facing behavior in the Admin Console:

- The hardcoded agent creation wizard is replaced by a flow-driven experience that executes the resolved Agent Onboarding flow.
- Credential presentation moves from a console completion screen into a flow step.
- Flow authoring and customization for Agent Onboarding is supported through the existing flow builder, including an Agent Onboarding template based on the default flow.

### Configuration

Conditional. A configurable flow handle maps agent onboarding to a flow definition and allows the default flow to be replaced without a console change.

Open item: define the exact configuration key or keys, their defaults, their validation, and whether the handle is deployment-level or organization-level. Flow validation is extended where required to support agent onboarding. See Open questions.

## Requirements

### R1. Agent onboarding is a configurable flow

**Requirement:** Agent onboarding must be executed through a flow definition resolved at runtime rather than through a hardcoded console sequence. A default Agent Onboarding flow is provided so the feature works out of the box, and organizations can customize the flow through the existing flow management capabilities.

**Acceptance criteria:**

- **AC1.1:** Given an organization using the default configuration, when an administrator starts agent onboarding, then the console resolves the configured flow handle and executes the resolved flow definition instead of a hardcoded sequence.
- **AC1.2:** Given an organization that has customized the Agent Onboarding flow, when an administrator starts agent onboarding, then the customized flow is executed with no console code change or console release.
- **AC1.3:** Given a fresh deployment with no customization, when an administrator runs agent onboarding, then the default Agent Onboarding flow completes onboarding end to end and produces a usable agent with credentials.

### R2. The organization controls the onboarding experience

**Requirement:** The flow definition determines which steps are included, the order of the steps, what information is collected, and which additional organization-specific steps are performed. The agent type schema continues to determine which agent attributes can be collected and stored.

**Acceptance criteria:**

- **AC2.1:** Given a flow definition that inserts an organization-specific step, when the flow runs, then that step executes in its configured position.
- **AC2.2:** Given a flow definition that reorders the onboarding steps, when the flow runs, then the steps execute in the configured order.
- **AC2.3:** Given a resolved agent type, when attribute collection runs, then only attributes permitted by the agent type schema can be collected and stored.


## Change log

| Version | Date | Change |
|---|---|---|
| 0.1 | 2026-09-07 | Initial specification. |
