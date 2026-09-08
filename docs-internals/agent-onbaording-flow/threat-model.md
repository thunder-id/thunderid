# Agent Onboarding Flow Threat Model

- **Status:** Draft
- **Version:** 0.1
- **Companion spec:** Agent Onboarding Flow Specification v0.1

This model covers the configurable Agent Onboarding administrator flow: resolution and execution of the flow, the onboarding steps, delegation of agent creation to the agent provider, presentation of the generated credentials.

## Overview

Agent onboarding creates a new agent principal and issues it authentication credentials. It runs as an administrator flow that the Admin Console resolves at runtime from a configurable handle and executes through the flow engine. The flow orchestrates the experience, while the existing agent management service stays authoritative for validation, agent record creation, credential generation, and inbound client configuration.

The security-relevant behaviour is the issuance of a live agent credential as the result of an administrator-driven flow, and the fact that the flow itself is configurable by organizations. The primary entry point is the authenticated administrator session in the Admin Console.

Cross-cutting concerns covered elsewhere: administrator authentication and session establishment, the flow engine's generic execution and step authorization internals, agent credential format and signing, and agent runtime authentication. These are referenced here as trust inputs and are not re-analysed.

## Scope

This model covers:
- Flow resolution and execution kickoff (console, configuration, flow store, flow engine).
- The onboarding steps that are specific to agent onboarding: permission check, organization unit and owner resolution, attribute collection.
- Provisioning delegation from the generalized provisioning step to the agent provider and the agent management service.
- Presentation of generated credentials carried as flow runtime data.
- Authoring and customization of the Agent Onboarding flow.

## Architecture

```mermaid
flowchart LR
  subgraph Untrusted
    ADM[Administrator browser]
  end
  subgraph Trusted [trust boundary]
    CON[Admin Console backend]
    CFG[(Configuration)]
    FST[(Flow Store)]
    FE[Flow Engine]
    AP[Agent Provider]
    AMS[Agent Management Service]
    CST[(Agent and credential store)]
  end
  ADM -->|HTTPS authenticated admin session| CON
  CON -->|resolve flow handle| CFG
  CON -->|load flow definition| FST
  CON -->|execute flow| FE
  FE -->|provisioning delegation| AP
  AP -->|create agent| AMS
  AMS -->|persist record and credential| CST
```

### Components

| Component | Task |
| --- | --- |
| Admin Console backend | Resolves the configured flow handle, loads the flow definition, and starts execution. Renders flow prompts and the credential presentation step. Does not own the onboarding sequence. |
| Configuration | Holds the flow handle that maps agent onboarding to a flow definition. Changing it changes which flow runs. |
| Flow Store | Stores and returns administrator flow definitions, including the default Agent Onboarding flow and any customized versions. |
| Flow Engine | Executes the flow steps, evaluates conditions, enforces step-level authorization, and holds runtime data including the generated credentials. |
| Provisioning step and agent provider | Resolves the provisioning mode and delegates agent creation to the agent provider, which calls the agent management service and returns the created agent and its credentials as runtime data. |
| Agent Management Service | Authoritative for agent validation, agent record creation, credential generation, and inbound client configuration. Owns the creation rules. |

### Actors

#### Actors

| Actor | Description | Roles or permissions |
| --- | --- | --- |
| Onboarding administrator | Authenticated administrator who runs the flow and views the presented credentials. | Requires the agent onboarding permission |
| Flow author | Administrator who creates or customizes the Agent Onboarding flow definition. | Requires the flow management permission |
| Unauthorized administrator | Authenticated user without the onboarding permission who should not be able to onboard agents. | No onboarding permission |
| New agent | The principal created by the flow. It is the target of onboarding and becomes an authenticating principal afterwards. | Holds the issued credential and inbound client configuration |
| Agent Management Service | Internal service invoked by the agent provider to create the agent. Trusted dependency. | N/A |

#### Entitlement matrix

| Actor | Run onboarding flow | View presented credentials | Author or customize onboarding flow | Create agent record |
| --- | --- | --- | --- | --- |
| Onboarding administrator | [Yes] | [Yes] | [No] | [No, delegated to the service] |
| Flow author | [No, unless also granted onboarding] | [No] | [Yes] | [No] |
| Unauthorized administrator | [No] | [No] | [No] | [No] |
| New agent | [No] | [No] | [No] | [No] |

### External Dependencies (not owned)

| Dependency | Description (usage, purpose, authentication, authorization, security) |
| --- | --- |
| Administrator authentication and session | Establishes the authenticated administrator identity that enters this flow. Owned by the authentication flow model. |
| Flow engine | Provides sequencing, conditional execution, step authorization, and runtime data. Owned by the flow engine model. |
| Agent Management Service | Performs validation, agent creation, credential generation, and inbound client configuration. Owned by the agent management model. |
| Credential issuance and agent authentication | Defines the format, signing, and later authentication of the issued agent credential. Owned by the client authentication and token model. |
| Configuration and flow storage | Persist the flow handle and flow definitions. Integrity of these stores is assumed and owned by the platform. |

## Threats and mitigations

### Out-of-scope interactions and risks

- Compromise of the administrator session or credential theft before the flow starts. Owned by the authentication flow model.
- Weaknesses in the credential format, signing, or agent runtime authentication after issuance. Owned by the client authentication and token model.
- Internal correctness of agent validation and record creation. Owned by the agent management model.

### Interactions

#### [01]: Flow resolution and execution kickoff

**Description**

The console reads the configured flow handle, loads the matching flow definition from the flow store, and starts execution in the flow engine. The handle is configurable so the default flow can be replaced.

**Assets involved**

| Initiator | Intermediate | Target |
| --- | --- | --- |
| Onboarding administrator | Admin Console, Configuration | Flow Store, Flow Engine |

**Data flow**

```mermaid
sequenceDiagram
  autonumber
  participant A as Administrator
  participant C as Admin Console
  participant CFG as Configuration
  participant FST as Flow Store
  participant FE as Flow Engine
  A->>C: start agent onboarding
  C->>CFG: resolve flow handle
  CFG-->>C: configured handle
  C->>FST: load flow definition
  FST-->>C: flow definition
  C->>FE: execute flow
```

**Security considerations**

| Area | Response | Comments |
| --- | --- | --- |
| Data confidentiality | [C-Low] | Handle and definition are configuration, not secrets. |
| Communication medium | [M-IN] | Internal calls between console and platform stores. |
| Transport security | [TLS] | Administrator request over HTTPS; internal calls over the platform's secured channel. |
| Authentication | Authenticated administrator session | Session owned by the authentication model. |
| Accessibility | [Restricted] | Reachable only from an authenticated admin session. |
| Authorization and Access Control | Onboarding permission required to start the flow | Enforced server-side, not only in the console. |

**Threat assessment**

| ID | Category | Threat | Materializable | Mitigation / comment |
| --- | --- | --- | --- | --- |
| 1 | Tampering | The configured flow handle is changed to point at a weaker or malicious flow that omits the permission check or leaks credentials, leading to unauthorized onboarding or credential exposure. | [No] | Changing the handle requires configuration or flow management permission and is audited. Security controls (authorization, schema validation) are enforced server-side and do not depend on the flow content. See residual R1 for the org-level scope question. |
| 2 | Spoofing | An unauthenticated request starts the flow. | [No] | The flow starts only from an authenticated administrator session (authentication model). |

#### [02]: Authorization for agent onboarding

**Description**

The permission check step gates onboarding in the flow. Authorization is also enforced at the provisioning boundary so that removing the step in a customized flow cannot bypass it.

**Assets involved**

| Initiator | Intermediate | Target |
| --- | --- | --- |
| Onboarding administrator | Flow Engine | Agent Provider, Agent Management Service |

**Data flow**

```mermaid
sequenceDiagram
  autonumber
  participant A as Administrator
  participant FE as Flow Engine
  participant AP as Agent Provider
  participant AMS as Agent Management Service
  A->>FE: proceed through onboarding
  FE->>FE: permission check step
  FE->>AP: provision (later step)
  AP->>AMS: create agent
  AMS-->>AP: authorize and create, or reject
```

**Security considerations**

| Area | Response | Comments |
| --- | --- | --- |
| Data confidentiality | [C-Low] | Authorization decision only. |
| Communication medium | [M-IN] | Internal calls. |
| Transport security | [TLS] | Platform secured channel. |
| Authentication | Authenticated administrator session | |
| Accessibility | [Restricted] | |
| Authorization and Access Control | Onboarding permission checked in the flow and re-checked at provisioning | Defense in depth across UI and back end. |

**Threat assessment**

| ID | Category | Threat | Materializable | Mitigation / comment |
| --- | --- | --- | --- | --- |
| 1 | Elevation of Privilege | An administrator without the onboarding permission runs the flow and creates an agent, gaining the ability to mint a live credential. | [No] | The permission check step gates the flow, and the agent management service independently authorizes the create call. Authorization must not rely on the flow step alone. Tracked as requirement R-A in residuals. |
| 2 | Elevation of Privilege | A customized flow removes the permission check step, so the only remaining gate is the back end. | [No] | Because authorization is enforced at the provisioning boundary, removing the flow step degrades user experience but does not bypass the security control. See residual R-A. |
| 3 | Process Risk | Customization removes an approval or governance step the organization intended, weakening oversight without a code change. | [No] | Flow authoring is privileged and audited. Organizations own their own governance for custom flows. Noted as a governance consideration, not a product-side exploit. |

#### [03]: Agent type resolution and attribute collection

**Description**

The flow resolves the agent type and organization unit, resolves the owner, and collects attributes. The agent type schema governs which attributes may be collected and stored.

**Assets involved**

| Initiator | Intermediate | Target |
| --- | --- | --- |
| Onboarding administrator | Flow Engine, agent type schema | Agent Management Service |

**Data flow**

```mermaid
sequenceDiagram
  autonumber
  participant A as Administrator
  participant FE as Flow Engine
  participant AMS as Agent Management Service
  A->>FE: select type, unit, owner, attributes
  FE->>AMS: submit for validation on create
  AMS-->>FE: accept or reject against schema
```

**Security considerations**

| Area | Response | Comments |
| --- | --- | --- |
| Data confidentiality | [C-Medium] | Attributes may include organization-specific or personal data. |
| Communication medium | [M-IN] | Internal calls. |
| Transport security | [TLS] | |
| Authentication | Authenticated administrator session | |
| Accessibility | [Restricted] | |
| Authorization and Access Control | Attribute set constrained by the agent type schema, enforced server-side | The schema is the source of truth for permitted attributes. |

**Threat assessment**

| ID | Category | Threat | Materializable | Mitigation / comment |
| --- | --- | --- | --- | --- |
| 1 | Tampering | The client submits attributes outside the agent type schema, or an owner or unit the administrator is not entitled to assign, to create an over-privileged or misattributed agent. | [No] | The agent type schema and owner and unit assignment are validated server-side by the agent management service, not trusted from the client. |
| 2 | Elevation of Privilege | Attribute values are used to grant the new agent broader access than intended. | [No] | Permitted attributes and their effect are bounded by the schema and validated on create. Custom flows cannot widen the schema. |
| 3 | Privacy Risk | A custom flow collects personal or unnecessary attributes beyond what onboarding needs. | [No] | Collection is bounded by the schema. Organizations that add custom steps own data minimization for those steps. Recorded under privacy considerations. |

#### [04]: Agent provisioning delegation

**Description**

The generalized provisioning step resolves the provisioning mode and delegates to the agent provider, which calls the agent management service to validate, create the agent, generate credentials, and configure the inbound client.

**Assets involved**

| Initiator | Intermediate | Target |
| --- | --- | --- |
| Flow Engine | Provisioning step, Agent Provider | Agent Management Service, agent and credential store |

**Data flow**

```mermaid
sequenceDiagram
  autonumber
  participant FE as Flow Engine
  participant PS as Provisioning step
  participant AP as Agent Provider
  participant AMS as Agent Management Service
  FE->>PS: provision
  PS->>PS: resolve provisioning mode
  PS->>AP: delegate (agent mode)
  AP->>AMS: create agent
  AMS-->>AP: agent and credentials
  AP-->>FE: runtime data
```

**Security considerations**

| Area | Response | Comments |
| --- | --- | --- |
| Data confidentiality | [C-High] | The response includes newly generated credential material. |
| Communication medium | [M-IN] | Internal service call. |
| Transport security | [TLS] | Secured internal channel. |
| Authentication | Service-to-service trust within the trust boundary | |
| Accessibility | [Internal] | Not reachable directly by external actors. |
| Authorization and Access Control | Provisioning re-checks onboarding permission; provider path fixed by resolved mode | |

**Threat assessment**

| ID | Category | Threat | Materializable | Mitigation / comment |
| --- | --- | --- | --- | --- |
| 1 | Tampering | The provisioning mode is manipulated to invoke a different provider or to bypass agent-specific validation. | [No] | The mode is resolved server-side inside the provisioning step, and each provider enforces its own validation before creation. |
| 2 | Elevation of Privilege | A crafted provisioning request creates an agent with elevated privileges. | [No] | The agent management service is the authoritative validator and applies creation rules regardless of the caller. |
| 3 | Denial of Service | Repeated onboarding requests exhaust credential generation or storage. | [No] | The path is restricted to authorized administrators. Rate limiting on provisioning is recommended and tracked as a checklist item. |

#### [05]: Credential presentation

**Description**

The generated credentials are carried as flow runtime data and shown to the administrator in the final flow step, rather than through a hardcoded console screen.

**Assets involved**

| Initiator | Intermediate | Target |
| --- | --- | --- |
| Flow Engine | Runtime data | Onboarding administrator |

**Data flow**

```mermaid
sequenceDiagram
  autonumber
  participant FE as Flow Engine
  participant C as Admin Console
  participant A as Administrator
  FE->>C: runtime data with credentials
  C->>A: present credentials once
```

**Security considerations**

| Area | Response | Comments |
| --- | --- | --- |
| Data confidentiality | [C-High] | The credential is a secret that authenticates the new agent. |
| Communication medium | [M-NT] | Delivered to the administrator browser. |
| Transport security | [TLS] | Presented only over HTTPS. |
| Authentication | Authenticated administrator session | Only the initiating administrator sees the result. |
| Accessibility | [Restricted] | |
| Authorization and Access Control | Runtime data scoped to the flow session of the requesting administrator | |

**Threat assessment**

| ID | Category | Threat | Materializable | Mitigation / comment |
| --- | --- | --- | --- | --- |
| 1 | Information Disclosure | The credential is written to server logs, audit logs, the URL, or persisted runtime storage in cleartext, exposing it to unintended parties. | [No] | The credential must be excluded from logs and URLs, presented over TLS, and not persisted in cleartext beyond what the flow requires. Enforcement is a requirement, tracked as R-B in residuals. |
| 2 | Information Disclosure | Runtime data leaks the credential to another administrator or session. | [No] | Runtime data is scoped to the requesting flow session. |
| 3 | Repudiation | Credential issuance cannot be traced to the administrator who performed it. | [No] | An audit record captures who onboarded which agent and when, without recording the secret itself. |

#### [06]: Failure compensation

**Description**

If a step after successful creation fails, the created agent is compensated so onboarding does not leave an unintended live credential.

**Assets involved**

| Initiator | Intermediate | Target |
| --- | --- | --- |
| Flow Engine | Agent Provider | Agent Management Service, agent and credential store |

**Data flow**

```mermaid
sequenceDiagram
  autonumber
  participant FE as Flow Engine
  participant AP as Agent Provider
  participant AMS as Agent Management Service
  FE->>FE: post-creation step fails
  FE->>AP: compensate created agent
  AP->>AMS: revoke or remove agent and credential
  AMS-->>AP: compensation result
```

**Security considerations**

| Area | Response | Comments |
| --- | --- | --- |
| Data confidentiality | [C-High] | Concerns the live credential that must not survive a failed onboarding. |
| Communication medium | [M-IN] | Internal call. |
| Transport security | [TLS] | |
| Authentication | Service-to-service trust within the boundary | |
| Accessibility | [Internal] | |
| Authorization and Access Control | Compensation acts only on the agent created in the same flow session | |

**Threat assessment**

| ID | Category | Threat | Materializable | Mitigation / comment |
| --- | --- | --- | --- | --- |
| 1 | Operational Risk | Compensation itself fails, leaving an orphan agent with a live, unintended credential. | [No] | Compensation runs on post-creation failure. A failed compensation must be logged and alerted so the orphan can be cleaned up. Bounded residual R-C. |
| 2 | Repudiation | Compensation is not recorded, so a created-then-removed agent leaves no trace. | [No] | Creation and compensation are both audited. |

#### [07]: Flow authoring and customization

**Description**

An administrator with flow management permission creates or customizes the Agent Onboarding flow through the flow builder. Flow validation is extended to support agent onboarding.

**Assets involved**

| Initiator | Intermediate | Target |
| --- | --- | --- |
| Flow author | Flow builder, flow validation | Flow Store |

**Data flow**

```mermaid
sequenceDiagram
  autonumber
  participant FA as Flow author
  participant FB as Flow builder
  participant V as Flow validation
  participant FST as Flow Store
  FA->>FB: edit onboarding flow
  FB->>V: validate definition
  V-->>FB: accept or reject
  FB->>FST: save definition
```

**Security considerations**

| Area | Response | Comments |
| --- | --- | --- |
| Data confidentiality | [C-Low] | Flow definitions are configuration. |
| Communication medium | [M-IN] | |
| Transport security | [TLS] | |
| Authentication | Authenticated administrator session | |
| Accessibility | [Restricted] | Requires the flow management permission. |
| Authorization and Access Control | Authoring restricted to flow management permission; changes audited with a diff | |

**Threat assessment**

| ID | Category | Threat | Materializable | Mitigation / comment |
| --- | --- | --- | --- | --- |
| 1 | Tampering | A flow author inserts a step that exfiltrates the presented credential or alters the provisioning behaviour. | [No] | Credential handling and provisioning validation are enforced server-side and are not defined by flow content. Authoring is privileged and audited. |
| 2 | Elevation of Privilege | A custom flow is authored to skip authorization or schema checks. | [No] | Authorization and schema validation are enforced at the back end regardless of flow content, per residuals R-A and the schema controls in interaction 03. |
| 3 | Repudiation | Flow changes cannot be attributed or reviewed. | [No] | Configuration change auditing records the author and the difference between versions. |

## Security Review Checklist

The states below reflect the intended design posture for a Draft threat model and must be confirmed at implementation. Items that a deployer owns are noted.

### Security considerations

| # | Consideration | State | Comments |
| --- | --- | --- | --- |
| 1 | Are all inputs and outputs validated (syntactic and semantic)? | [Partial] | Attribute, owner, unit, and type inputs validated server-side against the agent type schema. Confirm coverage for custom step inputs at implementation. |
| 2 | Are rate limits in place where necessary? | [Partial] | Recommended on provisioning to bound mass onboarding. Confirm and set thresholds at implementation. |
| 3 | Are permissions, roles, and entitlements defined on least privilege and business need? | [Yes] | Separate onboarding and flow management permissions; provisioning re-checks authorization. |
| 4 | Are authentication and authorization validated at both UI and API layers? | [Yes] | Permission check step in the flow plus server-side authorization at provisioning. |
| 5 | Are proper isolations in place between components to reduce blast radius? | [Yes] | Flow orchestration is separated from creation rules, which stay in the agent management service. |
| 6 | Have default credentials been changed and default superuser accounts avoided? | [N/A] | No default credentials introduced by this feature. |
| 7 | Has the implementation followed best-practice guidelines? | [Partial] | Aligned to OWASP proactive controls in design. Confirm at implementation. |
| 8 | Are secrets and internal-only material kept out of the public source tree and git history? | [Yes] | No secrets committed. Generated credentials are runtime only. |
| 9 | Was a security-focused code review conducted and findings addressed? | [No] | To be done during implementation. |
| 10 | Is SAST or IaC scanning conducted and findings addressed? | [Partial] | Applies through the standard pipeline. Confirm coverage for new code. |
| 11 | Is SCA conducted or integrated and findings addressed? | [Partial] | No new external dependency expected. Confirm at implementation. |
| 12 | Is DAST or API scanning conducted on non-production and findings addressed? | [No] | To be scheduled for the flow-driven path. |
| 13 | Are audit logs generated for critical functionality and available to authorized users? | [Yes] | Onboarding, credential issuance, and compensation are audited. Note retention with the platform default. |
| 14 | Do audit logs for critical configuration changes record old versus new versions? | [Yes] | Flow definition changes are recorded with a diff. |
| 15 | Are data in transit and at rest encrypted? | [Yes] | TLS in transit. Credential material at rest owned by the agent management and credential store. |
| 16 | Are sensitive values stored in a secret store or vault? | [Yes] | Credential storage owned by the agent management model and its store. |
| 17 | Is personal, sensitive, or confidential data kept out of logs? | [Yes] | Credentials and personal attributes excluded from logs. Enforce for custom steps. |
| 18 | Have users been given clear instructions for secure usage? | [Partial] | Documentation to describe secure customization and credential handling. |

### Business impact and resilience

| # | Consideration | State | Comments |
| --- | --- | --- | --- |
| 1 | Has a business impact analysis been done to identify resilience requirements? | [Partial] | Onboarding availability follows the platform's flow engine and agent service. Deployer owns targets. |

Resilience details to record:
- High availability requirements: inherited from the flow engine and agent management service.
- Disaster recovery requirements: inherited from the platform.
- Backups, frequency, and retention: flow definitions and agent records follow platform backup policy.
- Health checks: provided by the underlying services.
- User banners: not specific to this feature.

### Dependency and component health

| # | Consideration | State | Comments |
| --- | --- | --- | --- |
| 1 | Are dependencies, base images, and runtimes monitored and kept current? | [Partial] | Through the standard pipeline. Confirm for any new code. |
| 2 | Are any End-of-Life or End-of-Service components in use? | [No] | None introduced by this feature. |
| 3 | Is hardening guidance published for operators? | [Partial] | Provide guidance on secure flow customization and credential handling. |

### Privacy considerations

| # | Consideration | State | Comments |
| --- | --- | --- | --- |
| 1 | Is the purpose and legal basis for processing personal data clearly defined? | [Partial] | Agent attributes are usually non-personal, but custom steps may collect personal data. Deployer owns basis for custom collection. |
| 2 | Are collection through disposal aligned with data minimization? | [Partial] | Bounded by the agent type schema. Custom steps owned by the deployer. |
| 3 | Is personal data stored securely? | [Yes] | Storage owned by the agent management model. |
| 4 | Are privacy notices updated for new processing? | [N/A] | No new default personal-data processing. Revisit if custom steps collect personal data. |
| 5 | Is access to personal data granted on a need-to-know basis? | [Yes] | Restricted to authorized administrators. |
| 6 | Are data retention requirements considered? | [Partial] | Agent records follow platform retention. |
| 7 | Is there a process to dispose of personal data on request? | [Partial] | Follows platform agent lifecycle. |
| 8 | Are records of processing maintained in the data inventory? | [Partial] | Update if custom steps introduce personal data. |

## Change log

| Version | Date | Change |
|---|---|---|
| 0.1 | 2026-09-07 | Initial threat model. |
