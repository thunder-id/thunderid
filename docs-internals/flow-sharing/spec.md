# Flow Sharing and Variable Overrides Specification

- **Status:** Draft
- **Version:** 0.1
- **Related documents:** [threat-model.md](threat-model.md), [#6076](https://github.com/thunder-id/thunderid/pull/6076) (flows are organization unit bound), [#6138](https://github.com/thunder-id/thunderid/pull/6138) (the `VARIABLES` node and `{{var(...)}}` placeholders), [#6219](https://github.com/thunder-id/thunderid/pull/6219) (flow sharing and variable overrides), [#6315](https://github.com/thunder-id/thunderid/pull/6315) (`ouId` of a shared flow), [#6571](https://github.com/thunder-id/thunderid/pull/6571) (Console flow editor), [#6722](https://github.com/thunder-id/thunderid/pull/6722) (`onFailure` on the `VARIABLES` node), [Share a Flow](../../docs/content/guides/flows/share-a-flow.mdx), [Resource Sharing Across Organization Units](../../docs/content/key-concepts/resource-sharing.mdx)

## Summary

Flows are deployment scoped. Before this feature, a flow had no owning organization unit, so nothing tied a flow to a customer unit, and a unit could not tune one. A deployment that served many customer units could only create a separate flow for each, with nothing recording which unit it was for, or run one flow for everyone with no per-unit configuration. Separate flows drift from the original and multiply the maintenance. A single flow cannot carry a per-unit identity provider, a per-unit option list, or a per-unit toggle.

This feature lets one organization unit own a flow and share it with the units beneath it, without copying it. The owner declares typed variables on a `VARIABLES` node and uses them as `{{var(identifier)}}` placeholders in executor properties. A sharing policy then states, per variable and per unit, whether the unit may set its own value, what value it starts with or is pinned to, and what it may choose from. Each unit stores its own values. At sign-in the engine resolves every variable to the value of the unit the sign-in runs under, so the same flow behaves differently for each unit while the definition stays in one place.

The feature spans six changes. Flows became organization unit bound (#6076), which gives a flow an owner. The `VARIABLES` node and placeholders (#6138) make the definition parametric. Flow sharing and overrides (#6219) register flows with the resource sharing framework and add the per-unit values. The shared view carries the requesting unit's `ouId` (#6315). The Console edits variables, bindings and terms (#6571). A `VARIABLES` node can route a missing connection to a prompt (#6722).

One decision governs the rest:

> A shared flow is read only to the unit it is shared with, and the only thing a unit can configure is the set of variables the owner named for it, within the terms the owner set.

A unit never edits the nodes, interceptors, or layout of a flow it does not own. The owner's definition is the code, the variables are the parameters, and the sharing policy is the contract between them. Everything that follows keeps the contract enforceable: the policy is checked against the definition when either changes, a unit's value is checked against the variable and the policy when it is written, and the engine reads a unit's values only for the unit the sign-in actually runs under.

Scope is sharing flows with variable overrides, the runtime resolution of those variables, and the lifecycle rules that keep policies, values, and definitions consistent. Sharing of other resource types, the sharing framework's reach and narrowing algebra, and the `VARIABLES` node's type vocabulary are covered where they constrain flows and owned by the existing documents otherwise. Sharing a flow so that a unit can fork or edit its nodes, per-unit flow versions, variables in prompt `meta` or conditions, and sharing through an API with deployment-wide reach are out of scope.

## Architecture

The feature adds no service. It is a flow-management declaration on the sharing framework, a per-unit value store that the framework already provides, and a runtime lookup in the engine.

| Component | Responsibility |
|---|---|
| Flow management service | Owns the flow definition, its owning organization unit (`OU_ID`), the declared variables, and the checks that keep a definition consistent with its sharing. Serves the per-unit reads and writes of variables |
| Flow sharing declaration | Registers flows with the sharing framework as resource type `flow`. Declares one dynamic field, `variables`, so each variable is a field `variables.<identifier>`. Resolves the owning unit of a flow and validates connection members of a policy |
| Sharing framework | Stores policies, units reached, overlay rules, and per-unit values. Resolves reach, narrowing, effective rules, and visibility. Shared with applications, roles, connections, resource servers, and users |
| `VARIABLES` node and engine | The node declares variables. The graph builder lowers it to a task that resolves the whole set into runtime data under `var.<name>`. The engine applies a unit's values once the unit is known and resolves placeholders in executor properties before the executor runs |
| OU resolver executor | Decides which unit a sign-in runs under, from the request, the application, or the user's pick |
| Connection service | Owns identity provider connections. Counts a flow's variables as a use of a connection, so a connection a flow names cannot be deleted |
| Console | Edits variables, binds them to executor properties, and edits the terms per unit |

Ownership boundary: the flow management service owns what a variable means (its type, constraints, default, and the connections it may take). The sharing framework owns who a policy reaches and how terms narrow. Neither reaches into the other: the framework asks the flow declaration to validate members it cannot understand, and the flow service asks the framework to resolve rules, never to interpret a variable.

The feature uses these existing seams without changing their contracts:

- **Resource sharing framework.** Policies, the `share` and `reshare` stages, `childUnits`, `otherTreeRoots`, `allUnits`, overlay rules (`editable`, `value`, `allowedValues`, `excludedValues`), and per-unit overlay values.
- **Organization unit ownership.** Introduced for flows by #6076, as the first step of this feature. `FLOW.OU_ID` is the owner, as for connections, roles, and groups. A unit that owns a flow cannot be deleted.
- **Applications.** An application is owned by a unit or shared with units. The application decides which units a sign-in may run under.
- **Declarative resources and import.** A flow file carries `sharingPolicies`, applied on load and import.
- **Authorization.** The `read flow` and `update flow` actions, checked against the unit being acted on.

## Detailed design

### Ownership and visibility

Every flow has exactly one owning organization unit. The owner can always read, change, delete, and share the flow, and needs no policy. A flow with no policy is visible to its owner only.

A unit sees a flow it does not own when a policy reaches it. The sharing framework answers visibility, so a flow is visible to the owner and to every unit a policy reaches, directly or through resharing. Visibility is cached and re-evaluated when a policy changes.

For a caller confined to one unit (a token with an `ouId` claim), the unit is implied. List and read take the unit from the token when the request names none. A request that names another unit lists nothing or is refused with the same answer as a flow that does not exist, so the response does not reveal whether a flow exists elsewhere.

### The shared view

A unit that a flow is shared with reads a reduced view of it:

- It carries `isOwnedByOU: false` and `isReadOnly: true` in the list.
- Its `ouId` is the requesting unit's, never the owner's (#6315). The owner is recoverable only by reading the flow without a unit or through the `initiatingOuId` of a policy the caller may read.
- A single read returns the variables that a policy names for the unit and nothing else of the definition: no nodes, interceptors, layout, version history, or timestamps.
- Each variable is resolved for the unit. Its `defaultValue` is the value the unit's rule sets, or the owner's default when the rule sets none. A `SELECT` lists only the options the unit may choose. A variable no policy names for the unit is omitted entirely, and so is any detail the owner holds beyond what applies to the unit.

The owner's own default, held on the definition, is therefore not an entitlement. Nothing governs a variable for a unit until a policy names it, so a new variable stays invisible and unsettable for every unit until the owner opens it.

### Terms: sharing policies and overlay rules

Flows use the common sharing policy as defined in [Resource Sharing Across Organization Units](../../docs/content/key-concepts/resource-sharing.mdx). The flow-specific parts are the field, the rules, and the validation.

**The field.** A flow declares one dynamic field, `variables`. Each declared variable is a member `variables.<identifier>`. The field's default rule is `editable: true` and represents the owner's own standing, not terms for any unit.

**The rule.** For one variable and one unit, a rule has these parts:

| Part | Meaning for a variable |
|---|---|
| `editable` | Whether the unit may set its own value |
| `value` | At most one value. The value the unit starts with, or is pinned to when `editable` is `false` |
| `allowedValues` | The values the unit may choose from. Omitted means the variable's whole range |
| `excludedValues` | Subtracted from `value` and `allowedValues`, last. A policy that only withholds a connection is not a use of it |

A value is written in its raw string form: `"true"`, `"120"`, an option, or a connection ID. A rule that names a value pins the variable only when it carries exactly one value that is not excluded.

**Validation at write.** A policy is refused unless every rule names a variable the flow declares, `value` holds no more than one value, and every value is valid for the variable's type and constraints. For a connection variable each named value (including those only offered) must be an existing connection of a vendor the variable allows. Values a policy only withholds are checked for form, not existence, because nothing is stored for them.

**Connection members.** A unit issuing a policy that names a connection must own it or have it shared with it, so a unit cannot hand on a connection it cannot itself use. This is checked per member by the flow declaration through the sharing framework's visibility. The owner's own default is not checked per unit, so the owner can run a flow shared with a unit on a connection that unit cannot see.

**Resharing.** A unit reached by a policy can reshare the flow to the units beneath it. A resharing policy states `childUnits` only, narrows the terms it was given and can never widen them, and is one per unit per flow. The framework enforces narrowing. The flow service adds only the validations above.

**Deployment-wide reach.** `POST` and `PUT` on `/flows/{flowId}/sharing-policies` refuse any section whose reach is not bounded by where the issuer sits (`allUnits`, and `otherTreeRoots` where the owner has a parent unless `resource_sharing.allow_child_ou_cross_tree_sharing` is `true`) with `FLM-1043`. Such reach is a deployment decision and is made only through a resource file or an import.

**Authorization.** Creating, changing, or deleting a policy requires the `update flow` action in the unit that issues the policy, which is the owner by default and the resharing unit otherwise. A flow deployed declaratively or by the control plane is refused (`FLM-1048` for the latter), because its sharing belongs to its file.

### Per-unit values

A unit stores its own values with `PUT /flows/{flowId}/variables`. The values are the unit's overrides, held in the sharing framework's per-unit value store, keyed by flow, unit, and field.

- **Replace semantics.** A `PUT` replaces everything the unit held. A variable left out returns to the policy's value, or the owner's default when none applies. There is no partial update and no `null`.
- **Check before write.** Every value is checked before anything is written. A variable the flow does not declare, or that no policy names for the unit, is refused as unknown. A variable whose rule is not editable is refused. A value must be valid for the variable's type and constraints, permitted by `allowedValues` and `excludedValues`, and for a connection variable must be an existing connection of an allowed kind that the unit owns or has shared with it.
- **Atomic.** The replacement is one transaction in the sharing store, whichever store holds the flow. A failed write leaves the previous values.
- **Authorization.** The caller must be allowed to `update flow` in the unit whose values are written, and the flow must be visible to that unit. `GET /flows/{flowId}/variables` requires `read flow` in either the owner or the unit.
- **Reads.** Three reads answer three questions: what to render (`GET /flows/{flowId}`, resolved for the unit), what the unit has set (`GET /flows/{flowId}/variables`), and what it may change (`GET /flows/{flowId}/overlay-rules`). Only a value the unit set itself is an override. A value a policy fixes is not.
- **Cleared by narrowing.** When the owner changes a policy so that a unit can no longer set a variable (for example by pinning it), the value the unit saved for it is dropped. A later change to the flow is then not refused over a value the unit could not clear.

### Consistency of definition, policies, and values

Three records refer to a variable: the definition, the policies, and the unit values. The service keeps them consistent at the moment any of them changes.

| Change | Rule |
|---|---|
| Update flow, or restore a version, that breaks a recorded policy | Refused with `409` and `FLM-1041` |
| Update flow, or restore a version, that breaks a value a unit saved | Refused with `409` and `FLM-1042`. The message lists up to a bounded number of conflicts and a count of the rest |
| Update flow, or restore a version, that no longer declares a variable | The variable's rules in every policy and every unit's saved value for it are removed first. A variable still used by a `{{var(...)}}` placeholder cannot be removed |
| Policy written | Validated against the current definition (see Terms) |
| Declarative or import replay of policies | Validated against the new definition, applied, then the flow is re-checked, because policies the document does not name are kept and may still conflict. Runs in the transaction of the update, so a refusal undoes the update |
| Flow deleted | The flow's policies are deleted after the flow itself. See Failure behavior |
| Connection deleted | Refused while a flow uses it: as a variable default, as a policy value or offered value, or as a unit's saved value. A policy that only withholds the connection is not a use |
| Organization unit deleted | Refused while it owns a flow (`OU-1006`) |

A flow may change a variable that stays, for example a new type or fewer options, only when no policy and no saved value contradicts the change.

### Runtime resolution

A `VARIABLES` node declares the variables. The graph builder lowers it to a task that resolves every variable of the flow into runtime data (`var.<name>`), and `{{var(name)}}` is resolved in executor `properties` just before the executor runs. A whole-value placeholder keeps the variable's type. A placeholder inside longer text is substituted as text. Prompt `meta` and conditions are not resolved.

**Which unit's values apply.** The unit is the one the sign-in runs under, and no other: not the unit that owns the flow, and not the unit that owns the application. It is decided as follows:

1. If the request names a unit, the engine checks that the application is reachable from it. An unknown unit and a unit the application is not shared with give the same refusal, so the answer does not reveal which.
2. Otherwise, when the application is not shared, the application's own unit applies.
3. Otherwise the `OUResolverExecutor` decides, from the caller, from the user's own pick among the units they are eligible for, or from a full tree prompt. A unit decided by steps 1 or 2 is marked `ouFixed` and is never replaced by a later step.

**Effective values.** Once the unit is known, the engine reads the unit's effective values: for each variable a policy names for the unit, the value the unit saved, or else the value the policy fixes. Anything else resolves to the owner's default. Two preconditions apply, and a unit that fails either runs on the owner's values:

- When the unit is not the application's owner, the application must be shared with it. Naming a unit on a prompt is not enough to run on its values.
- The flow must be visible to the unit, and the unit must not be the flow's owner.

The lookup runs with the server's authority, because it reads values on behalf of an end user who holds no flow permissions, and it is bounded by the preconditions above.

**Placement.** Every route to a `VARIABLES` node in an authentication flow passes an OU resolver, and an OU resolver cannot follow a session save or load node (`FLM-1044`). This keeps the unit decided before values are read and prevents a session carrying a stale unit.

**A connection that is unset.** A connection variable may have an empty default, which means each unit must bring its own. When the unit has not set one, the node ends the flow with `FES-1022` ("Identity provider not found"). When the node's `onFailure` names a `PROMPT`, the engine instead forwards to it with the error. It also forgets a unit that a step of the flow picked, together with the submitted selection, so the resolver asks again. A unit that the request or an unshared application fixed is kept, so a retry cannot move the sign-in outside its boundary.

### Failure behavior

- **Flow and sharing are separate stores.** Deleting a flow deletes the flow first, because only that delete knows whether it will be refused, and then deletes its policies. The two cannot share a transaction, so a failure of the second step leaves policies behind a flow that is gone, and the caller is told. Retrying the delete finds the flow gone, removes the leftover policies, and succeeds.
- **Declared policies are not deletable through the API.** A policy that comes from a resource file answers with a "policy is declared" error to a delete and is skipped by a flow delete, because its file governs it.
- **Losing reach cleans up.** Deleting a policy removes the state of every unit that loses visibility because of it, including the reshares beneath it, in the policy's transaction. Caches are cleared whether the transaction committed or rolled back.
- **Unreadable stored value.** A stored value that no longer parses for its variable is a server error on read, logged with the flow, unit, and variable, and is never silently replaced by the default.
- **Lookup failure at sign-in.** A failure to read a unit's effective values fails the sign-in with a server error. It does not fall back to the owner's values.

### Data model

No table is added by flow sharing. It reuses:

| Table | Use |
|---|---|
| `FLOW` | `OU_ID VARCHAR(36) NOT NULL` is the owner. Added by #6076. The owner is immutable after creation |
| `FLOW_VERSION` | Holds the definition, including the `VARIABLES` node. Versions are visible to the owner only |
| `RESOURCE_SHARING_POLICY` | One row per flow and initiating unit, with the stage (`share` or `reshare`) and the sections stated |
| `RESOURCE_SHARING_POLICY_UNIT` | Units a policy names one by one, with `reach` |
| `RESOURCE_SHARING_POLICY_OVERLAY_RULE` | Rules per policy, section, unit, and field key (`variables.<id>`) |
| `RESOURCE_OVERLAY_VALUE` | A unit's own value for one field, keyed by resource type, resource, unit, and field key |

Flows live in the config database. Policies and per-unit values live in the sharing store, and a declarative flow's policies come from its file. A flow and its sharing records are in separate stores, so some operations cannot be one transaction (see Failure behavior).

### API

All paths are in `api/flow-management.yaml`. Errors are `FLM-` codes unless noted.

| Endpoint | Purpose | Notes |
|---|---|---|
| `POST /flows` | Create a flow | `ouId` required (`FLM-1029` when missing, `FLM-1027` when unknown) |
| `GET /flows` | List flows | `filter=ouId eq "<id>"` for a unit's view. Owned first, then shared. Invalid filter is `FLM-1040` |
| `GET /flows/{flowId}` | Read a flow | `?ouId=` for the shared view. Without it the owner's whole definition |
| `PUT`, `DELETE /flows/{flowId}` | Change or delete | Owner only. A unit the flow is shared with gets `403` `FLM-1039`. Any other unit gets the generic forbidden answer |
| `POST`, `GET /flows/{flowId}/sharing-policies` | Record or list policies | Owner or resharing unit. Deployment-wide reach is `FLM-1043` |
| `GET`, `PUT`, `DELETE /flows/{flowId}/sharing-policies/{policyId}` | Read, replace, or delete one policy | A change is validated against the definition |
| `GET /flows/{flowId}/overlay-rules?ouId=` | The unit's terms | A unit no policy reaches gets `404` |
| `GET /flows/{flowId}/variables?ouId=` | The unit's saved values | `ouId` required |
| `PUT /flows/{flowId}/variables` | Replace the unit's values | `ouId` required in the body. Never null. Unknown or fixed variable, or a value outside the terms, is a validation error |

Connection listing gains an `ouId` filter, with `CON-1039` for an invalid filter.

| Endpoint | Owner | Unit the flow is shared with |
|---|---|---|
| `GET /flows` | `isOwnedByOU: true` | `isOwnedByOU: false`, `isReadOnly: true` |
| `GET /flows/{flowId}` | Whole definition | Only the variables that apply, resolved |
| `PUT`, `DELETE /flows/{flowId}` | Allowed | `403` `FLM-1039` |
| `/versions`, `/restore`, `/usages` with `ouId` | Allowed | `404` for a read, `403` for a write |
| `GET`, `PUT /flows/{flowId}/variables` | Allowed | Allowed, within the terms |
| `/sharing-policies` | Allowed | Only to reshare what it may reshare |
| `GET /flows/{flowId}/overlay-rules` | Allowed | Its own unit |

Declarative flow files carry `sharingPolicies`. A flow document must name its owner with `ouId` or `ouHandle`.

### UI

The Console exposes three surfaces, all in the Control Plane flow editor (#6571):

- **Variables node.** A panel that declares each variable (text, email, phone, number, date, checkbox, select, connection select), with a preview of how a delegated administrator sees them. The preview reuses the SDK's flow variables component. The node has a Failure output that loads and saves as `onFailure` (#6722).
- **Variable binding.** A pen beside an executor property offers the declared variables, or declares a new one, and the property holds its `{{var(identifier)}}` placeholder.
- **Share view.** The units a flow is shared with, and for each variable whether the unit may change it, the unit default, and the choices. A save that would break a policy or a saved value is refused with the server's message. Renaming or removing a variable that is bound or shared asks for confirmation first.

### Configuration

| Key | Level | Default | Effect |
|---|---|---|---|
| `resource_sharing.allow_child_ou_cross_tree_sharing` | Deployment | `false` | Whether an owner that has a parent unit may state `allUnits` or `otherTreeRoots`. A root owner may always state them. Through the flow API both are refused regardless (`FLM-1043`), so this governs files and imports only |

## Requirements

### R1. A flow has one owning organization unit

**Requirement:** As an administrator, I want every flow to belong to one organization unit so that ownership, sharing, and deletion rules have an anchor.

**Acceptance criteria:**

- **AC1.1:** Given a request to create a flow without `ouId`, when it is submitted, then it is refused with `FLM-1029`.
- **AC1.2:** Given a flow, when an update names a different `ouId`, then it is refused with `FLM-1028`. An update that omits `ouId` keeps the owner.
- **AC1.3:** Given an organization unit that owns a flow, when it is deleted, then the delete is refused with `OU-1006`.
- **AC1.4:** Given a declarative or imported flow document with neither `ouId` nor `ouHandle`, when it is loaded, then the load fails.

### R2. An owner can share a flow without copying it

**Requirement:** As the owner of a flow, I want to share it with the units beneath me so that they run one definition that I maintain.

**Acceptance criteria:**

- **AC2.1:** Given a flow with no policy, when a unit other than the owner reads or lists it, then the flow is not visible to that unit.
- **AC2.2:** Given a policy that reaches a unit, when the unit lists flows filtered to itself, then the flow appears with `isOwnedByOU: false` and the unit's own `ouId`.
- **AC2.3:** Given a policy through `POST /flows/{flowId}/sharing-policies` that states `allUnits`, or `otherTreeRoots` where the owner has a parent, when it is submitted, then it is refused with `FLM-1043`.
- **AC2.4:** Given a unit reached by a policy as a `reach: unit` child, when it issues a policy for the flow, then the policy states `childUnits` only and cannot widen the terms it received.
- **AC2.5:** Given a policy, when the owner deletes it, then the unit it reached no longer sees the flow and its saved values no longer apply at sign-in.

### R3. A shared flow is read only and shows only variables

**Requirement:** As the owner, I want a unit to see and configure only the variables I named for it so that my definition stays private and unchanged.

**Acceptance criteria:**

- **AC3.1:** Given a flow shared with a unit, when the unit reads it, then the response contains the variables that apply to the unit and no nodes, interceptors, layout, versions, or timestamps.
- **AC3.2:** Given a flow shared with a unit, when the unit sends `PUT` or `DELETE` on the flow, then it is refused with `FLM-1039`.
- **AC3.3:** Given a variable no policy names for a unit, when the unit reads the flow, then the variable is absent.
- **AC3.4:** Given a `SELECT` variable with an `allowedValues` or `excludedValues` rule, when the unit reads the flow, then only the permitted options are listed.
- **AC3.5:** Given a unit the flow is not shared with, when it requests the flow, its versions, or its overlay rules, then the answer does not reveal whether the flow exists.

### R4. A unit sets its own variable values within the owner's terms

**Requirement:** As a delegated administrator of a unit, I want to set my own values so that the flow suits my organization without changing the flow.

**Acceptance criteria:**

- **AC4.1:** Given an editable variable, when the unit sends a valid value, then it is stored and returned by `GET /flows/{flowId}/variables`.
- **AC4.2:** Given a variable whose rule is not editable, when the unit sends a value for it, then the request is refused and nothing is written.
- **AC4.3:** Given a value outside `allowedValues` or inside `excludedValues`, when the unit sends it, then the request is refused and nothing is written.
- **AC4.4:** Given a request with one invalid value among valid ones, when it is submitted, then no value is written.
- **AC4.5:** Given a connection variable, when the unit sends a connection that does not exist, is not of an allowed kind, or that the unit neither owns nor was shared, then the request is refused.
- **AC4.6:** Given a `PUT` that omits a variable the unit had set, when it is applied, then the unit's override for that variable is removed.
- **AC4.7:** Given a `null` value, when it is submitted, then it is refused.
- **AC4.8:** Given a caller not authorized to `update flow` in the unit, when it sends a `PUT`, then it is refused.

### R5. A policy only states terms a flow can honour

**Requirement:** As the owner, I want a policy that names an invalid variable or value to be refused so that terms never silently fail at sign-in.

**Acceptance criteria:**

- **AC5.1:** Given a rule that names a variable the flow does not declare, when the policy is written, then it is refused.
- **AC5.2:** Given a rule whose `value` holds more than one value, when the policy is written, then it is refused.
- **AC5.3:** Given a value that is invalid for the variable's type or constraints, when the policy is written, then it is refused.
- **AC5.4:** Given a connection variable and a policy naming a connection that does not exist or is of a disallowed kind, when the policy is written, then it is refused.
- **AC5.5:** Given a unit that issues a policy naming a connection it neither owns nor was shared, when the policy is written, then it is refused.

### R6. Definitions, policies, and values stay consistent

**Requirement:** As the owner, I want a change that would strand a policy or a unit's value to be refused or cleaned up so that no unit is left with terms that cannot run.

**Acceptance criteria:**

- **AC6.1:** Given a recorded policy, when an update or restore would break it, then the change is refused with `FLM-1041` and nothing changes.
- **AC6.2:** Given a value a unit saved, when an update or restore would invalidate it, then the change is refused with `FLM-1042` and the response lists the conflicts.
- **AC6.3:** Given an update or restore that no longer declares a variable, when it is applied, then every policy rule and every unit value for the variable is removed with it.
- **AC6.4:** Given a variable used by a `{{var(...)}}` placeholder, when an update would remove it, then the update is refused.
- **AC6.5:** Given a change to a policy that stops a unit from setting a variable, when it is applied, then the unit's saved value for that variable is removed.
- **AC6.6:** Given a replay of policies from a declarative file or import, when it conflicts with the new definition, then the whole update is refused and undone.
- **AC6.7:** Given a connection that a flow uses as a default, as a policy value or offered value, or as a unit's saved value, when the connection is deleted, then the delete is refused. A policy that only withholds the connection does not count.

### R7. At sign-in a flow runs on the values of the right unit

**Requirement:** As an end user, I want a sign-in through a shared flow to use the values of the unit I sign in under, and as a relying party, I want a sign-in never to borrow another unit's values.

**Acceptance criteria:**

- **AC7.1:** Given a unit that saved a value for a variable, when a sign-in runs under that unit, then the variable resolves to that value.
- **AC7.2:** Given a policy that pins a variable for a unit and a unit that saved nothing, when a sign-in runs under that unit, then the variable resolves to the pinned value.
- **AC7.3:** Given a unit with no policy naming a variable, when a sign-in runs under it, then the variable resolves to the owner's default.
- **AC7.4:** Given a unit the application does not belong to and is not shared with, when it is named by a prompt, then the sign-in runs on the owner's values.
- **AC7.5:** Given a request that names a unit the application is not reachable from, or that does not exist, when a sign-in starts, then it is refused with the same error in both cases.
- **AC7.6:** Given a unit fixed before the flow ran, when a later step tries to pick another unit, then the fixed unit is kept.
- **AC7.7:** Given a flow whose route to a `VARIABLES` node does not pass an OU resolver, or whose OU resolver follows a session save or load node, when it is saved, then it is refused.
- **AC7.8:** Given the owner's own unit, when a sign-in runs under it, then the variables resolve to the owner's defaults.

### R8. A missing connection fails safely

**Requirement:** As a flow author, I want a connection variable that has no connection for the unit to end the flow, or show an error and let the user pick another unit, so that a unit never runs on a connection it did not choose.

**Acceptance criteria:**

- **AC8.1:** Given a connection variable with an empty default and a unit that set none, when the `VARIABLES` node runs, then the flow ends with `FES-1022`.
- **AC8.2:** Given the same state and an `onFailure` that names a `PROMPT`, when the node runs, then the flow shows the error on that prompt.
- **AC8.3:** Given AC8.2 and a unit that a step of the flow picked, when the flow returns to the OU resolver, then it asks for the unit again.
- **AC8.4:** Given AC8.2 and a unit fixed by the request or by an unshared application, when the flow returns to the OU resolver, then the same unit applies.
- **AC8.5:** Given an `onFailure` that does not name a `PROMPT`, when the flow is saved, then it is refused.

### R9. Sharing is declarable in a file

**Requirement:** As an operator, I want to keep a flow's sharing policies with the flow in source control so that a deployment is reproducible.

**Acceptance criteria:**

- **AC9.1:** Given a flow file with `sharingPolicies`, when it is loaded or imported, then the policies are validated against the flow and applied.
- **AC9.2:** Given a dry run of an import, when a document would be refused for real, then the dry run refuses it too.
- **AC9.3:** Given a file policy for a unit that already has a declared policy, when it is replayed, then it is skipped with a warning, and a policy removed to match the document is logged.
- **AC9.4:** Given a flow deployed from a file or by the control plane, when a caller changes its sharing through the API, then the change is refused.

### R10. A shared flow leaves nothing behind

**Requirement:** As an operator, I want deleting a flow to remove its sharing so that a flow created later with the same id does not come back shared.

**Acceptance criteria:**

- **AC10.1:** Given a flow with policies, when it is deleted, then its policies are deleted.
- **AC10.2:** Given a delete whose policy cleanup failed, when the delete is retried, then the leftover policies are removed and the delete succeeds.
- **AC10.3:** Given a flow that a unit has saved values for, when the flow is deleted, then no value remains reachable.

## Change log

| Version | Date | Change |
|---|---|---|
| 0.1 | 2026-11-16 | Initial specification, written from the merged changes #6076, #6138, #6219, #6315, #6571, and #6722. |
