# Resource Server Sharing Specification

- **Status:** Draft
- **Version:** 0.1
- **Related documents:** [Issue #5634: Share resource servers with organization units](https://github.com/thunder-id/thunderid/issues/5634), [Threat model](threat-model.md), [Machine-to-Machine Access Across Organization Units](../m2m-access-across-ous/spec.md), [Resource sharing](../../../docs/content/key-concepts/resource-sharing.mdx)

## Summary

A resource server belongs to one organization unit. Sharing it, through the resource sharing framework, lets other organization units use its permissions. Until now the share recorded intent and nothing read it: any organization unit's roles could hold any resource server's permissions, and a confined administrator read the resource servers of the units above its own through organization unit inheritance.

This specification makes the share the bound. A resource server is **available** in an organization unit that owns it or that one of its sharing policies reaches, and its permissions are used only there: in role writes, in what a role confers where it is assigned, in tokens issued for an organization unit, in what a confined administrator reads, and in what a delegated administrator may count as held.

A policy shares the whole resource server. Sharing part of one, by naming the permissions a policy shares, is out of scope.

## Architecture

| Component | Task |
|---|---|
| Resource service | Answers availability (`IsResourceServerAvailableInOU`) and validates permissions in a named organization unit (`ValidatePermissionsInOU`). Lists the resource servers an organization unit owns, then the ones shared with it, and reads a shared one for a confined caller. Answers availability once per request that acts for an organization unit. |
| Resource server sharing declaration | Registers resource servers as a sharing resource type, with one fixed `permissions` field. Refuses a sharing change that takes the resource server from an organization unit whose roles use it (`OverlayCleaner`). |
| Sharing framework | Owns policies and visibility. `IsSharedWith` answers whether a policy reaches an organization unit, for a caller that has already read the owner. |
| Role service | Validates a role's permissions in the role's organization unit on every write and on declared and imported roles. Drops the permissions of a resource server the assignment unit may not use when it resolves what a role confers there. Reports to the dependency registry whether an organization unit's roles use a resource server. |
| Effective permission resolver | Resolves what an administrator may use, narrowed per assignment unit, and what an entity, group or role holds, not narrowed. |
| Resource indicator helpers | Ask the runtime resource server provider whether the accessing organization unit may use a resource server, through an unexported interface the server's own provider implements. The public `ResourceServerProvider` is unchanged, and a provider without the method, which has no resource server sharing, is not bound. |

## Detailed design

### Availability

A resource server is available in organization unit X when X owns it or one of its sharing policies reaches X. Availability is strict: a resource server no policy names is available only in its owner.

`IsResourceServerAvailableInOU` reads the resource server without the caller's authorization, since a token request and a role write both ask it whoever the caller is. It compares the owner first and then asks the sharing framework's `IsSharedWith`, so the resource server is read once. A request that acts for an organization unit, through `/ou/{ouId}/...` or a flow, carries a memo on its context, and each answer for a resource server and organization unit is worked out once for the rest of the request.

A resource server the organization unit cannot see, an organization unit that does not exist, and a resource server that does not exist all answer not available. A failure to answer is a server error, never "available".

The bootstrap shares neither the System resource server nor the Control Plane resource server. Both are available only in the default organization unit, and an operator shares the System resource server with the organization units whose roles need its permissions, such as the ones whose administrators hold `org`.

### Role writes

Creating a role, updating its permissions, and moving it to another organization unit validate its permissions in the organization unit the role is in after the write, through `ValidatePermissionsInOU`. A resource server that organization unit cannot use is refused with `ROL-1047`. The resource service answers the refusal with `RES-1034`, which the role service maps; it never reaches a client as `RES-1034`.

A role imported, or declared in a resource file, is checked the same way. A declared role that breaks the rule fails startup with the role's name. Resource servers and their declared policies load before roles, so a policy the same deployment declares counts.

### What a role confers where it is assigned

A role shared into an organization unit that cannot use one of its resource servers is still shared, and can still be assigned there. Where it is assigned in that organization unit, the resource server's permissions are dropped when permissions are resolved for that assignment unit: `GetAuthorizedPermissionsByResourceServer` and `ExpandHeldDescendants` answer nothing for it. Token issuance names the assignment unit, so tokens follow the rule.

Read in that organization unit (`GetRoleInOU`, behind `GET /roles/{id}?ouId=`, a confined caller's read, and the MCP role tools), the shared role lists only the permissions of the resource servers the organization unit can use.

Sharing the resource server with that organization unit later makes every such assignment confer its permissions. The share is the approval: whoever may share the resource server decides that the roles assigned in the organization unit it reaches may use it, and no assignment is reviewed again.

A resolution that names no assignment unit counts the assignments made in every organization unit and is not narrowed. AuthZEN evaluation is one, so an AuthZEN policy service can see permissions that tokens leave out.

### Listing and reading

`GET /resource-servers?filter=ouId eq "<id>"` lists the resource servers the organization unit owns, then the ones shared with it, each with `isOwnedByOU`. A shared one is listed under that organization unit and is read only there. The shared page is read with one query by IDs.

A caller confined to an organization unit lists the same without a filter, and reads a shared resource server by ID, with its resources and actions, under its organization unit, read only, and with `isOwnedByOU: false`. A read of a resource server in its owning organization unit carries `isOwnedByOU: true`. The confined caller no longer reads the resource servers of the organization units above its own through inheritance: `rs:view` and `rs:list` left the inheritance policy.

The Console's role permission picker lists the resource servers for the role's organization unit and marks shared ones. The resource server list and page mark a shared one **Shared** rather than **System**. A connection's authorization mapping picker leaves shared ones out, since a connection's mappings accept only the resource servers its organization unit owns.

The MCP `list_resource_servers` tool takes `ouId` and marks shared resource servers, and the role tools say the role's organization unit must be able to use the resource server.

### Delegated administration

What an administrator holds counts, for each organization unit its roles are assigned in, only the permissions of the resource servers that organization unit can use (`GetUsablePermissions`). The permissions are read with one query that carries the assignment unit, and each assignment unit and resource server is checked once. This only narrows what the administrator may grant, so it cannot widen a grant.

What is granted is not narrowed. Assigning a role, adding to a group, and taking control of an entity still compare against every permission the role, group or entity holds, wherever it is assigned (`GetAllPermissions`, `ResolveHeldByEntity`). AuthZEN evaluation counts every assignment, so a check narrowed to the assignment unit would let an administrator hand out a permission AuthZEN grants. The MCP user and agent tools list the permissions usable in each assignment unit.

### Narrowing and moving

A sharing change that takes a resource server from an organization unit is refused with `RES-1039` while a role that organization unit owns holds a permission of it. The declaration's `OverlayCleaner` asks the dependency registry, which the role service answers.

Moving a resource server is refused first with `RES-1037` while it has sharing policies, since its policies were issued against its organization unit and project. A resource server without policies is then refused with `RES-1039` while roles of its current organization unit hold its permissions, since that organization unit would no longer own it. Deleting the resource server is not refused: its delete removes those permissions from the roles.

Two paths do not reach the refusal:

- **A declared policy narrowed in its resource file.** The policy is reseeded at startup from the file, so no sharing change runs. The roles of an organization unit the new policy no longer reaches keep their permissions and confer none of them there.
- **An import or a Control Plane deploy that drops an organization unit from the policy and the permissions from the roles at once.** Resource servers apply before roles, so the narrowing is refused with `RES-1039` while the roles still hold the permissions. Split the change: first remove the permissions from the roles, then narrow the policy.

The refusal only looks at roles the organization unit owns. Shared roles assigned there and tokens issued there lose the resource server without being refused, and their tokens are not revoked by this change.

### Upgrade

On upgrade no policy exists, so every resource server is available only in its owner:

- A role outside the default organization unit that holds System resource server permissions, such as an organization unit administrator role holding `org`, confers nothing of them until the System resource server is shared with its organization unit. Sharing through the API or the Console needs the upgraded server, so it happens after the upgrade.
- Such a role cannot be updated without dropping those permissions.
- A declared role in another organization unit that holds System resource server permissions stops the server from starting, and the System resource server has no resource file to declare a policy in. Move the role's System permissions out of its file, start the server, share the System resource server with the role's organization unit, and put the permissions back.
- A token requested on the organization unit path for a resource server that is not shared there gets `invalid_target`.

### Not in this specification

Sharing part of a resource server: a policy naming the permissions it shares, and a token for an organization unit carrying only those. That belongs to a future specification, along with narrowing AuthZEN evaluation per assignment unit and revoking tokens when a resource server is unshared.

## Requirements

### R1. A resource server's permissions are used only where it is available

**Requirement:** A role confers, and a token carries, permissions of a resource server only in the organization units that own it or that a policy of it reaches.

**Acceptance criteria:**

- **AC1.1:** Given a resource server no policy names, when a role in another organization unit is created or updated with its permissions, then the request is refused with `ROL-1047`.
- **AC1.2:** Given a declared or imported role in an organization unit that cannot use one of its resource servers, when the server starts or the import runs, then it fails naming the role.
- **AC1.3:** Given a role shared into an organization unit the resource server is not shared with, when the role is assigned there and a token is issued for that assignment unit, then the token leaves out the resource server's permissions, and the role read in that organization unit does not list them.
- **AC1.4:** Given a token request for an organization unit whose target resource server is not available there, when the token is requested, then it fails with `invalid_target`, as for a resource server that does not exist.

### R2. An organization unit sees the resource servers it can use

**Requirement:** A confined administrator lists and reads the resource servers its organization unit owns and the ones shared with it, and no others.

**Acceptance criteria:**

- **AC2.1:** Given a resource server shared with organization unit X, when a caller lists with `filter=ouId eq "X"`, then it is listed after the owned ones, under X, read only, with `isOwnedByOU: false`.
- **AC2.2:** Given a caller confined to X, when it reads a shared resource server by ID, then it gets the resource server, its resources and its actions, under X and read only. When it reads one X owns, `isOwnedByOU` is true.
- **AC2.3:** Given a resource server owned by X's parent and not shared with X, when a caller confined to X reads it, then the read is refused with `403`.

### R3. Delegated administration counts only usable permissions as held

**Requirement:** An administrator can grant only what it can use, and every grant still counts everything it confers.

**Acceptance criteria:**

- **AC3.1:** Given an administrator whose role in X holds permissions of a resource server X cannot use, when it grants a role holding them, then the grant is refused.
- **AC3.2:** Given a shared role holding permissions of a resource server the assignment unit cannot use, when an administrator that does not hold them assigns it there, then the assignment is refused.

### R4. A resource server is not taken from an organization unit whose roles use it

**Requirement:** A sharing change or a move does not silently leave an organization unit's roles conferring nothing.

**Acceptance criteria:**

- **AC4.1:** Given roles of X holding a resource server's permissions, when a policy change through the API stops reaching X, by update or delete, then it is refused with `RES-1039`.
- **AC4.2:** Given a resource server with sharing policies, when an update moves it to another organization unit, then it is refused with `RES-1037`, before any check of the roles.
- **AC4.3:** Given a resource server without policies whose organization unit's roles hold its permissions, when an update moves it, then it is refused with `RES-1039`.

## Change log

| Version | Date | Change |
|---|---|---|
| 0.1 | 2026-11-27 | Initial specification, moved out of the M2M access specification with the availability rules #7091 enforces. |
