# Resource sharing across OUs

- **Status:** Draft
- **Version:** 0.2
- **Related documents:** [Discussion #3310: Architectural First Principles for Resource Modeling in B2B Context](https://github.com/thunder-id/thunderid/discussions/3310), [Issue #3346: Build core modules to support the common resource architecture in B2B context](https://github.com/thunder-id/thunderid/issues/3346)

## Summary

ThunderID models organizations as a forest of OU trees. [Discussion #2719](https://github.com/thunder-id/thunderid/discussions/2719) lists the patterns that forest has to carry: B2B SaaS sold to many customers, an enterprise with several divisions, third-party partners, resellers, and multiple brands in one company.

Each pattern holds most resources per OU. Some resources are not local to one OU though: they are common to a tree, or to several trees. A resource is owned by one OU and still has to be usable from others, and which OUs those are is a decision somebody makes, not a consequence of where the resource was created.

Copying the resource into each OU duplicates the owner's configuration and lets the copies drift. What the use cases need is one resource made available to other OUs, where each of those OUs can change some of its data inside bounds set by the owner or by an OU between the owner and the consumer.

Sharing gives a resource **reach** instead of a copy: one resource, one owner, and a set of policies recording which other OUs can see it and on what terms. A consuming OU never receives the owner's core configuration. It receives permission to use the resource, the templated fields it may set for itself, and the bounds it must stay inside. Read-only access is not enough, because an OU that cannot adapt a resource to its own division copies it anyway.

Nothing here is specific to one kind of resource, so sharing is built once as a resource-type-agnostic framework.

## Architecture

```mermaid
flowchart TB
    RT["Resource type<br/>core config and templated fields"] --> REG["Registry"]
    REG --> SVC
    SVC["Sharing service<br/>orchestrates, does not decide"] --> PE["Policy engine<br/>pure functions, no I/O"]
    SVC --> CS["Composite policy store<br/>one store to ask"]
    CS --> ST["Database store<br/>relational, deployment scoped"]
    CS --> DS["File-declared store<br/>in memory"]
    SVC --> RES["OU hierarchy resolver<br/>upward traversal"]
    SVC --> ENU["OU hierarchy enumerator<br/>downward traversal"]
    SVC --> CA["Visibility and overlay caches"]
    SVC --> HK["Resource type hooks<br/>ownership, members, cleanup"]
```

| Component | Responsibility |
|---|---|
| Policy engine | Every decision: can an OU see a resource, what do its terms work out to, is a proposed edit allowed. Plain functions over plain values, with no database, clock or request context, so the narrowing rules and chain evaluation are testable from a table of inputs. Kept as its own layer with its own vocabulary, so that a decision cannot reach for a store, a logger or a request while it is being made |
| Sharing service | Fetches the policies bearing on a question, asks the hierarchy where an OU sits, translates types, opens transactions, clears caches, calls the resource type's hooks. It decides nothing itself; every judgement is a call into the policy engine |
| Registry | The list of onboarded resource types, one declaration each. Optional capabilities are not registered separately: the framework checks whether the declaration implements them |
| Composite policy store | Puts the two halves behind one interface, so the service asks one store rather than merging two. Reads consult the database first and fall back to the declarations; writes go to the database alone; an OU's own field values are database rows whichever half the governing policy came from |
| Database store | Policies, targets, exclusions, overlay rules, and each OU's own values for templated fields. Keeps no rows for the resources themselves; what a resource *is* stays the type's business |
| File-declared store | The same policies for resources defined in a file, held in memory for the life of the process. The file is their source of truth, so persisting them would strand rows when the file changes and duplicate them on restart |
| OU hierarchy resolver | Upward questions: this OU's ancestors, whether one OU sits above another. The same instance OU initialization already builds |
| OU hierarchy enumerator | Downward questions: what sits beneath an OU, what OUs exist. Separate on purpose, because walking a subtree is not an access decision and should not be reachable from the object that makes them |

The composite store is where the two halves meet, rather than the service. Every read otherwise carries its own "and also ask the declarations" branch, and the rules for merging them are subtle enough to want one home: the database answers first, so that a row left behind by a declaration since withdrawn from its file still applies; a declared policy is dropped from a listing when a stored row governs the same OU, so one OU is never answered with two policies; and a failure to read the database is a failure, never a silent fallback to the declared half alone, which would resolve to a narrower answer than the truth and read as a resource somebody has lost access to.

## Detailed design

Three ideas account for most of the design.

- **Sharing is chained, not matched.** Nothing is matched against a pattern. Coverage starts at an OU something named outright (the owner, a root the owner reached, or an OU the owner pointed a share at) and travels down a branch hop by hop through targets that reach into a subtree. That is what makes revocation clean: remove whatever covered a position and everything reached through it goes too, while an OU named in its own right is untouched.
- **Reach is granted one step at a time.** A policy may only name OUs directly beneath the OU issuing it. To go deeper, either say "and its subtree" or let the named OU issue its own policy. An ancestor never grants reach over the head of the OU that holds the resource.
- **Terms only narrow, and only the frontier OU may narrow them.** Nobody passes on more than they hold, and an OU may set terms for its own children only if what it received named that OU alone. Where the policy above already reaches its children, those children are the granter's to decide. That is what keeps exactly one policy covering any OU, so resolving terms is a lookup rather than a reconciliation.

#### Two ways to make a policy smaller

A policy can be made smaller along either of two independent axes, and the document keeps them apart because they answer different questions and are checked by different rules.

| Term | Axis | What it changes | How | Who may set it |
|---|---|---|---|---|
| **Rule narrowing** | terms | *what* an OU reached by a target may do with a field | lower `editable`, or shrink the values on offer | the policy's issuer, and a frontier OU for its own children |
| **OU Exclusion** | reach | *which* OUs a target reaches at all | name an OU in that target's `excludedOuIds`, which removes it and its subtree | the policy's issuer alone |

**Rule narrowing** is the only thing "narrowing a policy" means in this document unless reach is named explicitly. It has exactly two moves, and both only ever reduce:

- **Lock the field.** Turn `editable` from true to false. The OU can still read the field and can no longer write it.
- **Restrict the options.** Shrink `allowedValues`, pin `value`, or add to `excludedValues`. The OU may still write the field, but from a smaller menu.

Neither move can go the other way. A policy cannot raise `editable`, widen `allowedValues` past what its issuer holds, or drop one of its issuer's `excludedValues`, and an attempt is refused rather than quietly trimmed.

**OU Exclusion** is not a kind of rule narrowing and does not touch the rules at all. It is a targeted removal: the named OU and everything beneath it stop being reached by every target on the policy, so they hold nothing rather than holding less. That is why exclusion is what cleanup keys off, and rule narrowing is not: an OU whose rule got smaller still has the resource, while an excluded OU has lost it.

#### Frontier OUs, and who may decide what

An exclusion belongs to the target carrying it and narrows that target's reach alone. **An OU cannot withhold the resource from a child that a policy above it already reaches.** Reach flows down from whoever granted it, so withdrawing it is the granter's to do.

Whether an OU has anything to decide at all therefore comes down to one question about the policy that reached it: does that policy stop at the OU, or does it carry on past it?

| The OU was reached by | The policy | The OU is |
|---|---|---|
| `root`, `allRoots`, or a `child` target | stops at the OU it names | a **frontier OU** |
| a peer target (future) | stops at the OU it names, its children included | a **frontier OU** |
| `childSubtree` or `allChildren` | carries on to every depth below the anchor | inside someone else's reach |
| `allOus` | carries on across the whole deployment | inside someone else's reach |

A **frontier OU** holds the resource with nothing below it granted. It may issue its own policy: name its own children, set tighter terms for them, and exclude whichever of them it likes, because that reach is its to give. This is the only way the resource travels deeper than one step without a subtree target, and it is how a chain of single-OU shares hands a resource down a branch one OU at a time.

What put the OU on the frontier does not matter, only that the policy stopped there. An OU reached as a peer from another tree is a frontier OU on exactly the same terms as one named by its own parent: the peer target reached it and nothing below it, so its subtree is its to hand on, one policy at a time. That is what R5 means by a peer being able to share down its own branch.

An OU inside someone else's reach holds nothing to pass on. Its children already see the resource on the granter's terms, so it may neither exclude them nor retighten them, and a policy it writes is refused rather than accepted with no effect. Everything about those OUs, exclusions and rules alike, is set where the covering policy was defined. Under a deployment-wide `allOus` policy that means one policy governs every OU in the deployment, and only its issuer can change anything.

The cost is deliberate: delegation now requires the granter to name OUs rather than reach for a subtree. A resource shared with `allChildren` is uniform across the subtree by construction; an owner who wants each OU to set its own terms downward names them with `child` targets instead and accepts the enumeration. The gain is that **exactly one policy covers any OU**, which is what makes reach, terms, revocation and export agree with one another.

### Policies, stages and targets

A **policy** is one OU's standing decision about one resource: who it reaches and on what terms. Each OU gets exactly one policy per resource. A second create is refused, naming the existing policy to edit instead. That constraint is what makes an edit well defined and stops policies piling up.

Every policy records a **stage**: `share` when the owner issued it, `reshare` when the issuer is an OU the resource had already reached. A reshare requires the issuing OU to see the resource at the moment it writes the policy.

**Targets** are not a list of OUs. They are a list of anchors, each with a scope; the actual set of OUs is worked out when somebody asks. A subtree target written today therefore still covers an OU created under that subtree tomorrow.

A policy is a **flat list of targets**. Each target carries its own scope, its own exclusions and its own terms. Nothing sits outside a target.

```yaml
targets:
  - scope: allChildren
    excludedOuIds: [ acme-customer-a ]
    overlayRules:
      isRecoveryFlowEnabled: { editable: false }
  - scope: child
    ouId: acme-customer-a
    overlayRules:
      isRecoveryFlowEnabled: { editable: true }
```

| Scope | Anchor | Who may issue | Reach |
|---|---|---|---|
| `child` | a direct child of the issuer | owner or sharee | That OU alone |
| `childSubtree` | a direct child of the issuer | owner or sharee | That OU and everything beneath it |
| `allChildren` | the issuer itself, implied | owner or sharee | Every OU beneath the issuer, the issuer excluded |
| `root` | a tree root | owner only, cross-tree gated | That root alone. Does not cascade |
| `allRoots` | none | owner only, declarative only | Every root alone. Does not cascade |
| `allOus` | none | owner only, declarative only | Every OU in the deployment, as a fallback |

An explicit target can only name a direct child, so it is called `child` rather than `ou`. The names `ou` and `ouSubtree` are reserved for peer targets, which reach an OU sideways.

A request with no target is malformed. So is a target whose scope and OU disagree: `root`, `child` and `childSubtree` each need one, the other three must not carry one.

#### Targets of one policy must not overlap

An OU two targets both reach has two sets of terms and no way to choose. Such a policy is refused.

To give one OU its own terms, the broader target excludes it. That is the carve-out above.

| Broader | Swallows | Cleared by |
|---|---|---|
| `allOus` | every other scope | excluding the narrower target's anchor |
| `allRoots` | `root` | excluding that root |
| `allChildren` | `child`, `childSubtree` | excluding that child |

Nothing else can overlap: a selective target names a direct child or a tree root, and neither sits inside the other. An exclusion takes the OU's subtree with it, so excluding the anchor is always enough. `allOus` beside `allRoots` has no cure, because neither anchors on an OU.

Naming one OU in two targets is refused too, for the same reason. `child` and `childSubtree` on the same OU is the same clash.

The one-hop rule applies per target, wherever it sits in the list.

**Exclusions** carve an OU, and its subtree, out of **one target**. Another target of the same policy may still reach it, which is what makes the carve-out work. Only the policy's issuer may set one; see [Frontier OUs](#frontier-ous-and-who-may-decide-what).

An exclusion its own target does not reach is refused, since it withholds nothing. A sibling target reaching that OU does not count.

```mermaid
flowchart LR
    subgraph A["Tree A"]
        OW["Owning OU"]
    end
    subgraph B["Tree B"]
        RB["Root of B"]
        B1["OU inside B"]
        B2["OU inside B"]
    end
    OW -->|"1. owner reaches the root"| RB
    RB -->|"2. B decides who inside B sees it"| B1
    RB -.->|"hidden until B decides"| B2
```

Landing on a root reaches the root and nothing beneath it, so entering another tree takes two decisions and the receiving root stays in charge of its own interior.

### Peer-to-peer sharing (future requirement)

Everything above follows the tree: a policy reaches downward from its issuer, or lands on a root. Two OUs that are not parent and child cannot share at all. That misses the partner pattern from Discussion #2719, where two independent businesses are wired together sideways, and something more ordinary too: one division handing a resource to a sibling division without pushing it up through their common parent.

A **peer** target names exactly one OU anywhere in the forest, with scope `ou`, or `ouSubtree` to bring its subtree. A sideways target is the only one that may name an OU which is not a direct child, which is why those two names are held for it.

```mermaid
flowchart LR
    subgraph T1["Tree 1"]
        R1["Root 1<br/>not reached"]
        A1["Owning OU"]
        A2["Sibling OU"]
        G2["Beneath the sibling<br/>not reached"]
    end
    subgraph T2["Tree 2"]
        R2["Root 2<br/>not reached"]
        B1["OU inside tree 2"]
    end
    A1 -->|"reaches this OU alone"| A2
    A1 -->|"cross tree, gated"| B1
```

Four constraints keep it from becoming a way around everything else:

| Constraint | Reason |
|---|---|
| Owner only | An OU that merely received a resource can pass it down its own branch but cannot hand it sideways, so lateral spread is always the owner's decision. It also keeps every ceiling dependent on a policy issued above it, which is what lets re-materialization stay a walk in depth order |
| One OU, and it stops there | It reaches the named OU and nothing else. If that OU wants its subtree to see the resource it issues its own policy, exactly as another tree's root would. There is no "all peers" form |
| Never a descendant | `child`, `childSubtree` and `allChildren` already cover that ground, and two ways to say one thing become two code paths that eventually disagree |
| Off by default | It has to be switched on for the deployment, and a target in a different tree also has to clear the cross-tree setting |

In visibility resolution a peer target acts as an **entry point**: it covers the named OU's position directly, without needing anything above it covered, and coverage does not leak upward, so the OU's parent and root are judged on their own. Otherwise it behaves like any named target, so an OU reached both by a peer target and by its own ancestor holds what both permit.

### Visibility resolution

Deciding whether an OU can see a resource is a walk, not a lookup. Take the branch the OU sits on, start at the top, and step down one position at a time, deciding for each whether anything covers it. A position is covered either in its own right, or by a position above it that is itself covered and reaches down. Only the second kind depends on the walk so far.

```mermaid
flowchart TD
    S{"Is this OU the owner?"} -->|yes| V["Visible"]
    S -->|no| T["Start at the top of the branch"]
    T --> C{"Does a policy reach this position?"}
    C -->|no| H["Hidden, and so is everything beneath it"]
    C -->|yes| N{"Is this the OU in question?"}
    N -->|no| D["Step down one position"]
    D --> C
    N -->|yes| V
```

Four ways a position can be covered:

- **Root coverage** happens only at the top of the branch, from an owner's `root` or `allRoots` target.
- **Descendant coverage** looks upward from the position just above, ignoring any position not itself covered. An `allChildren` or `childSubtree` target anchored on a covered position reaches any depth below it, provided nothing between anchor and position is excluded. A `child` target covers only the position immediately below the OU that named it, which is why naming a grandchild directly never reaches it.
- **Peer coverage** (future) applies wherever the named OU sits, whether or not anything above it is covered, and stops there.
- **Deployment-wide coverage**, from `allOus`, fills in where nothing more specific reached, either at the top of the branch or below a position that is already covered. Being available at the top is what lets it reach a root, and therefore any tree: without that it could never establish coverage anywhere, because a root has no position above it and root coverage comes only from a `root` or `allRoots` target. Below the top it behaves like any fallback and needs the position above covered, so it fills a gap in a chain rather than starting one mid-branch.

A target's exclusions are consulted only for the target doing the covering. That is what makes reach the granter's to withdraw, and what lets one target carve an OU out while another target of the same policy still reaches it.

Because only a frontier OU may issue a policy, an OU holds at most one policy per resource, and the targets of one policy may not overlap, exactly one target ever covers an OU, and the walk returns that one. The mechanism carries a list rather than a single value so that "no policy covered this" is the same shape as the others rather than a special case, and so a scope added later cannot silently pick a winner. The fetch behind it is narrowed to the branch being walked: a policy can only bear on the branch through a blanket scope or an anchor on it, so the query grows with branch length rather than with how widely the resource is shared.

### Overlay rules and rule narrowing

An **overlay rule** answers "what may this OU do with this templated field?". It has four parts.

| Field | Meaning |
|---|---|
| `editable` | Whether the receiving OU may write the field |
| `value` | What the receiving OU starts with, or is pinned to when the field is not editable |
| `allowedValues` | What it may choose from. Absent means the field's whole value space |
| `excludedValues` | Subtracted from both of the above, last |

For the three list fields, omitting one and setting it to an empty list mean opposite things: omitted means "whatever my parent allowed", empty means "nothing at all". Collapsing them would turn the most restrictive rule into the least restrictive one, so they stay distinct all the way into storage.

A rule is worked out when it is written, against what the issuing OU holds, so reading it back later is one lookup instead of another walk. Asking for more than you hold is **refused, not quietly reduced**, because a policy that was quietly cut back looks like it worked, and the author finds out much later:

- `editable: true` when the issuer holds `editable: false`
- an `allowedValues` or `value` set its own does not cover

Two more are refused on their own terms, with no parent involved: a `value` outside its own `allowedValues`, and `editable: false` combined with `allowedValues`, which is a menu nobody may choose from.

`excludedValues` is the one part that composes instead of being checked. A request's carve-outs are **added** to the parent's rather than replacing them, so every value the parent withheld stays withheld whatever the request says. Un-carving is therefore not something to refuse; it is not expressible. A request naming one carve-out of its own and not restating the parent's is a narrowing, and must be accepted: requiring it to repeat every ancestor's exclusions would make a legal request fail for forgetting something it cannot change anyway.

How two members compare depends on what a member is, which only the resource type knows. A declaration picks one of three kinds:

| Kind | Members are | Compared by |
|---|---|---|
| `scalar` | one opaque value | equality |
| `referenceSet` | a set of ids | set membership |
| `hierarchy` | delimiter-joined paths, where a path names everything beneath it | prefix containment in the type's own separator |

A hierarchy field needs the type to supply its separator, and the framework refuses to resolve the field without one: a type that declares a hierarchy field but implements no `FieldDelimiterResolver`, or resolves an empty delimiter, fails the request rather than comparing raw strings.

Both the original request and the resolved result are stored. When an ancestor policy changes and a descendant is recalculated, the recalculation starts from the original request; otherwise each narrowing would stack on the last, and an OU would never get anything back when the ancestor later loosened.

Recalculation **clamps where a write refuses**, and the asymmetry is deliberate. A request arriving from a caller is refused when it exceeds what the issuer holds, because a request quietly cut back looks like it worked. A request being replayed was accepted once already, and the thing that changed is above it: cutting it back to fit is the whole point, and refusing instead would let a sharee's old request veto an owner tightening its own resource.

### Rule resolution

One target covers an OU, so its terms are that target's rule, worked out when the policy was written. There is nothing to reconcile and no policy-level rule to fall back through.

The fold below is a guard, not a mechanism. It receives one rule and returns it, and exists so that a future scope which breaks the one-target invariant degrades to the safe answer rather than to whichever rule happened to be read last. Peer sharing does not break it: a peer target is owner-only, an owner holds one policy, and a peer cannot name an OU the owner already reaches downward.

Were it ever to receive two, the fold is per field: `editable` must be true in all of them, `allowedValues` and `value` intersect, and `excludedValues` union so every withheld value survives. One case needs care: a pinned rule carries no menu, so folding a bounded editable rule with a pinned one moves the bound onto the value rather than producing a rule that would fail its own validation.

If no policy mentions a field the answer is the type's declared default, and the result says which of the two it came from, because "the owner decided this" and "nobody said anything" are different statements. An OU that cannot see the resource is different again: it gets **empty** rules rather than defaults, since the defaults describe what an OU holding the resource may do when nobody said otherwise, not "this OU may do nothing".

### Editing a policy

Every edit carries the version the caller believes it is editing. If the policy moved since it was read the update matches no row and the caller is told, rather than silently flattening somebody else's write.

A policy counts as **blanket** when any of its targets is `allOus`, `allRoots` or `allChildren`, and **selective** otherwise. An edit cannot move it between families.

A blanket target already reaches everything in its family, so there is nothing for it to grow toward. A selective target is bounded by the one-hop rule, so growing it gives its issuer nothing new. The restrictions therefore apply per target:

- **blanket targets** cannot change, and their rules cannot widen. **Omitting a rule counts as widening**, because the field then falls back to the declared default, which is what the rule was overriding
- **selective targets** may be added or dropped, and their rules edited either way. Editing a carve-out means exactly that, so freezing them would freeze the pattern it exists for

Exclusions may be edited in both directions on any target. Adding one carves an OU out; removing one hands the resource back to an OU the issuer had carved out itself, and that is the issuer's own decision to reverse. What a scope family protects is other people's reach, not the issuer's earlier opinion of its own.

The two refusals answer differently, because they describe different mistakes. An edit that changes how broadly a policy reaches, in either direction, is refused as a change of scope family; an edit that widens or drops a blanket target's rule is refused as a widening. Telling the author of a policy that names OUs individually that "a blanket policy may only be narrowed" would describe a policy they are not editing.

An edit rebuilds the target rows with fresh ids, so rules are matched across versions by what their target selects, not by the id they point at.

An edit can move the limit other policies were cut back to, so every reshare of that resource is recalculated in the same transaction. Walking down the parent links would now reach the same set, since each policy has exactly one parent, but recalculating every reshare of the resource stays the simpler statement and is robust to a frontier OU whose parent was declarative and so unlinked. The recalculation runs shallowest issuer first: a covering policy is always issued from above, so working top down rebuilds each limit before anything cut back to it. Reshares that come out unchanged are left alone, otherwise their versions would bump and a concurrent edit of an untouched policy would start failing.

An edit can take reach away as well as tighten terms. A new exclusion, or a target the policy no longer names, takes the resource away from OUs that had it, so such an edit runs the same cleanup a delete does.

### Deletion and cleanup

Cleanup runs when a policy is deleted, or when an edit takes reach away from it: a new exclusion, or a target it no longer names. Rule narrowing alone never triggers it, because an OU whose terms got smaller still holds the resource. Each OU that has just lost sight of the resource is then cleaned up:

1. The OU's own values for the resource's templated fields are deleted. The framework deletes from its own overlay table unless the type implements `OverlayCleaner`, in which case that is called instead, because only the type knows where it keeps them.
2. The type's `PolicyHooks` cleanup fires for that OU and that resource, naming no fields.

The hook names no fields because it fires only on total loss: the OU can see nothing of the resource, so every field goes. A field list derived from the removed policy could not be complete anyway. A field the policy never named can still hold state, either because the type declared an editable default for it, which an OU may write with no policy mentioning it, or because the field uses dynamic keys, where a rule on the prefix does not enumerate the instances beneath it. Both clearing paths beside the hook already take no field list, so the hook matches them.

Deleting a policy also deletes the policies its sharees wrote, and that is the intended depth rather than collateral damage. The framework removes them itself, walking the parent links inside the delete's own transaction, because the link is a plain value rather than a cascading foreign key and because the OUs about to lose the resource have to be read off those policies before they are gone. A policy's parent is the one policy covering its issuer, because only a frontier OU may write one. If the parent goes, its issuer can no longer see the resource, so nothing it granted onward can stand. Cleanup follows the same path: an OU that loses sight of the resource puts everything its own policies reached back on the queue, so the OUs below a removed dependent are cleaned rather than silently abandoned.

Which OUs those are is decided by asking about visibility **after** the change, not by reading the removed policy's target list. Deleting a policy outright takes every OU it reached, since no second policy covers any of them. An edit is the case the re-check earns its keep on: dropping one target or adding an exclusion leaves the policy's other targets standing, and an OU those still reach keeps what it had.

Getting the candidate list needs a downward walk, because a target records an anchor and not the set beneath it: a subtree or root target reached everything below its anchor, `allChildren` everything below its issuer, and a blanket scope the whole deployment. If no downward enumerator is available the service logs loudly rather than continuing quietly, since per-OU state left under a policy that no longer exists is a data-retention problem nobody would notice.

A declared policy cannot be deleted. Its existence belongs to the file that declares it, so it goes away by being removed from the file.

### Declarative policies

A resource defined in a file declares its own policies in that file. They pass exactly the same validation as any other policy and then sit in memory alongside the stored ones for the life of the process. `allOus` and `allRoots` are available only this way, because a reach that is not bounded by the issuer's own position in the tree is a deployment decision rather than a request.

**A declaration carries its own id.** The file supplies it, the way a declared role supplies its own, and a declaration without one is refused. The id is what a reshare beneath the policy points at, so minting one at load time would move that anchor on every restart and orphan everything written beneath it.

**A declared policy is never written to the database.** The file is its only source of truth. Persisting it would strand rows the moment the file changed and duplicate them on the next start, and it would leave two authorities disagreeing about one policy.

**Re-applying a file replaces what it declared.** The replacement is keyed on the resource and the initiating OU, not on the id, so a file that rewrites its policy under a new id still replaces the old one rather than adding a second. That is what makes replaying every file on every start idempotent: nothing accumulates, because nothing survives a restart to accumulate onto.

**A declared policy cannot be edited or deleted through the API.** The file says what it says; an operator who wants different terms edits the file. What an OU reached by a declaration may do instead is issue a policy of its own, which is an ordinary reshare and is bounded by the declaration exactly as it would be by a stored policy.

Two consequences follow, and are intended:

- A declaration conflicts only with a **stored** row for the same resource and initiating OU, never with another declaration of that pair. Two files naming one pair is an authoring mistake that resolves to whichever loaded last, and the framework has no way to tell which the author meant; a stored row, on the other hand, means the API and a file both claim the same policy, and that is reported rather than silently resolved.
- Removing a declaration from its file removes the policy on the next start, and nothing prunes what it left behind before then. An OU's own values for the resource stay until the loss is noticed, which is the same behaviour a declared role's assignments have.

### Resource type onboarding

Onboarding means registering one declaration: the type's identifier and its templated fields, which are the fields a policy may carry rules for. Each field gives its key, the kind of its members, and the rule that applies when no policy mentions it. It may also name a coarser fallback key, used when no rule mentions the field itself, and a dynamic-key prefix so a map-shaped field is declared once instead of enumerating every member.

Five capabilities beside the declaration itself. None are registered separately; the framework checks whether the declaration implements each. Two of them are not optional, and the difference matters at onboarding time:

| Capability | Required | Purpose |
|---|---|---|
| `OwnerResolver` | Always | Resolves which OU owns a resource. The framework stores no resource rows and reads ownership nowhere else |
| `FieldDelimiterResolver` | When any field is of kind `hierarchy` | Supplies that field's separator |
| `MemberValidator` | Optional | Decides whether the values a rule lists are ones the issuing OU may legitimately name |
| `PolicyHooks` | Optional | Clears the type's own per-OU state for the whole resource when an OU loses sight of it |
| `OverlayCleaner` | Optional | Deletes an OU's own field values on the same event, for a type that keeps them outside the framework's table |

**Ownership is required, and a type without it must not onboard.** Registration refuses such a type rather than accepting it and failing later. The reason is that nothing else can answer the question: the fetch that serves a visibility question is scoped to the asking OU's chain, and an owner's own policies point at *other* OUs, so an owner asking about its own resource fetches nothing to read an owner from. A type that could not answer would have its owner told it cannot see its own resource, which is a worse failure than refusing to start.

**A hierarchy field's separator is required for the same reason, and an empty answer counts as no answer.** Comparing delimiter-joined paths without the delimiter is raw string prefixing, under which `billing` covers `billingx` and a policy hands over a sibling nobody named. There is deliberately no default to fall back on: a type declaring a hierarchy field and resolving no separator has every operation on that field refused, rather than quietly granted more than it wrote.

**`MemberValidator` answers with an error whose class decides the outcome.** A client error means the members are refused, and the caller is told so without the reason: "no such member" and "that member is not yours to name" have to be indistinguishable, or the API can be used to enumerate what exists in OUs the caller cannot see. Anything else means the check itself could not run, which is reported as an internal failure, because a store the type could not reach says nothing about whether the members were nameable.

It is worth spelling out why this capability exists at all. Suppose a rule pins a field to a permission string. The framework can check that string against the sets a parent policy allowed, but it does not know whether the permission exists, which resource server defines it, or whether the issuing OU has any business naming it. Only the type can answer that, which is also why a parent that names no value of its own bounds nothing: it stands for everything the owner holds, so a request naming members is a narrowing the framework has no set to check, and the type's answer is the only gate.

**Neither cleanup hook is field-scoped.** Both fire only when an OU has lost the resource outright, and at that point every value it holds for the resource is dead. A list derived from the removed policy could not be complete anyway: a field that policy never named can still hold state, either because the type declared an editable default for it or because the field uses dynamic keys, where a rule on the prefix does not enumerate the instances beneath it.

### Data model

Five tables in the configuration database. Every table carries `DEPLOYMENT_ID VARCHAR(255) NOT NULL`, and every child table cascades on delete from its policy. No existing table is reused or altered.

Two shapes are deliberate and worth stating before the tables. A policy row records only what the framework itself reasons about, so nothing that belongs to a resource type leaks into this schema. And a declared policy has no row at all: the file is its source of truth, so there is no column saying a row came from one.

#### `RESOURCE_SHARING_POLICY`

One row per OU decision about one resource.

| Column | Type | Notes |
|---|---|---|
| `DEPLOYMENT_ID` | `VARCHAR(255) NOT NULL` | Deployment isolation |
| `ID` | `VARCHAR(36) PRIMARY KEY` | UUID v7 |
| `RESOURCE_TYPE` | `VARCHAR(64) NOT NULL` | The registered type identifier |
| `RESOURCE_ID` | `VARCHAR(36) NOT NULL` | Not a foreign key: the framework stores no resource rows |
| `OWNING_OU_ID` | `VARCHAR(36) NOT NULL` | The OU that owns the resource |
| `INITIATING_OU_ID` | `VARCHAR(36) NOT NULL` | The OU whose decision this policy is |
| `POLICY_STAGE` | `VARCHAR(16) NOT NULL` | `CHECK (POLICY_STAGE IN ('share', 'reshare'))` |
| `PARENT_POLICY_ID` | `VARCHAR(36)` | The policy covering this one's issuer. A plain value, not a foreign key: see below |
| `VERSION` | `INTEGER NOT NULL DEFAULT 1` | Optimistic concurrency on edit |
| `CREATED_AT`, `UPDATED_AT` | `TIMESTAMPTZ DEFAULT NOW()` | |

`UNIQUE (DEPLOYMENT_ID, RESOURCE_TYPE, RESOURCE_ID, INITIATING_OU_ID)`, one policy per OU per resource.

Index: `idx_rsp_parent (PARENT_POLICY_ID)`.

`PARENT_POLICY_ID` carries no foreign key and no cascade, which is a decision rather than an omission. A reshare's parent may be a declared policy, and a declared policy has no row for a constraint to point at; a foreign key would make the legal case unstorable. The reference is therefore a plain value, in the way a role assignment names its role, and removing what hangs off a deleted policy is the service's job inside the same transaction as the delete. That is where it has to be anyway: the OUs that lose the resource have to be worked out before the rows naming them are gone, so cascading would destroy the information cleanup needs.

#### `RESOURCE_SHARING_POLICY_TARGET`

One row per target anchor, so one policy can name several.

| Column | Type | Notes |
|---|---|---|
| `DEPLOYMENT_ID` | `VARCHAR(255) NOT NULL` | |
| `ID` | `VARCHAR(36) PRIMARY KEY` | Per-target rules point at this |
| `POLICY_ID` | `VARCHAR(36) NOT NULL` | `REFERENCES "RESOURCE_SHARING_POLICY" (ID) ON DELETE CASCADE` |
| `TARGET_SCOPE` | `VARCHAR(16) NOT NULL` | `CHECK (TARGET_SCOPE IN ('allOus', 'allRoots', 'root', 'allChildren', 'child', 'childSubtree'))` |
| `TARGET_OU_ID` | `VARCHAR(36)` | Null only for `allOus` and `allRoots`. Every other scope anchors on an OU, `allChildren` included, where it holds the issuing OU itself |

`UNIQUE (POLICY_ID, TARGET_SCOPE, TARGET_OU_ID)`, and `UNIQUE (POLICY_ID, ID)` for the exclusion and overlay rule composite keys to reference. Plus a `CHECK` tying `TARGET_OU_ID` to the scope:

```sql
CHECK ((TARGET_SCOPE IN ('allOus', 'allRoots') AND TARGET_OU_ID IS NULL)
    OR (TARGET_SCOPE NOT IN ('allOus', 'allRoots') AND TARGET_OU_ID IS NOT NULL))
```

Indexes: `idx_rspt_policy (POLICY_ID)`, `idx_rspt_target_ou (DEPLOYMENT_ID, TARGET_OU_ID)`, and `idx_rspt_blanket_once`, unique over `(POLICY_ID, TARGET_SCOPE)` where `TARGET_OU_ID IS NULL`, which the plain `UNIQUE` misses because nulls count as distinct.

#### `RESOURCE_SHARING_POLICY_TARGET_EXCLUSIONS`

OUs carved out of **one target**, each taking its subtree with it.

| Column | Type | Notes |
|---|---|---|
| `DEPLOYMENT_ID` | `VARCHAR(255) NOT NULL` | |
| `POLICY_ID` | `VARCHAR(36) NOT NULL` | `REFERENCES "RESOURCE_SHARING_POLICY" (ID) ON DELETE CASCADE` |
| `TARGET_ID` | `VARCHAR(36) NOT NULL` | The target the carve-out belongs to. Constrained with `POLICY_ID`, below |
| `EXCLUDED_OU_ID` | `VARCHAR(36) NOT NULL` | |

`PRIMARY KEY (POLICY_ID, TARGET_ID, EXCLUDED_OU_ID)`. Index: `idx_rspte_target (TARGET_ID)`.

`TARGET_ID` is constrained with `POLICY_ID`, so an exclusion cannot name another policy's target:

```sql
FOREIGN KEY (POLICY_ID, TARGET_ID)
    REFERENCES "RESOURCE_SHARING_POLICY_TARGET" (POLICY_ID, ID) ON DELETE CASCADE
```

#### `RESOURCE_SHARING_POLICY_OVERLAY_RULE`

What a policy says one target may do with one field.

| Column | Type | Notes |
|---|---|---|
| `DEPLOYMENT_ID` | `VARCHAR(255) NOT NULL` | |
| `ID` | `VARCHAR(36) PRIMARY KEY` | Rule members point at this |
| `POLICY_ID` | `VARCHAR(36) NOT NULL` | `REFERENCES "RESOURCE_SHARING_POLICY" (ID) ON DELETE CASCADE` |
| `TARGET_ID` | `VARCHAR(36) NOT NULL` | The target the rule belongs to. Every rule names one. Constrained with `POLICY_ID`, below |
| `FIELD_KEY` | `VARCHAR(255) NOT NULL` | |
| `RESOLVED` | `JSONB NOT NULL` | The rule as worked out against what the issuer holds |
| `REQUESTED` | `JSONB NOT NULL` | The rule as the issuer asked for it |

`UNIQUE (POLICY_ID, TARGET_ID, FIELD_KEY)`. No partial index beside it, since `TARGET_ID` is never null.

`TARGET_ID` is constrained together with `POLICY_ID`, so a rule cannot pair a policy with another policy's target:

```sql
FOREIGN KEY (POLICY_ID, TARGET_ID)
    REFERENCES "RESOURCE_SHARING_POLICY_TARGET" (POLICY_ID, ID) ON DELETE CASCADE
```

Indexes: `idx_rspor_policy (POLICY_ID)`, `idx_rspor_target (TARGET_ID)`.

Each of the two columns holds one rule as a document: `editable`, and the three member lists. A document rather than columns and a member table, for three reasons.

The distinction between an omitted list and an empty one survives on its own. Omitted means "whatever my parent allowed" and empty means "nothing at all", and in a member table zero rows spells both, which is why that shape needed a boolean per list to disambiguate. A document simply has the key or does not.

A member list is opaque to the framework and can be long. Members are the resource type's own vocabulary, compared through the kind the type declared, and nothing here filters, joins or orders by them; storing them as rows buys query power that no query uses, and costs a row per member per rule per policy. Keeping the whole rule as one value also makes a rule row atomic: it is written and read as the unit the algebra actually operates on.

And the requested form has to be kept beside the resolved one, because recalculation replays the request rather than re-narrowing an already narrowed result. Two documents keep that pairing obvious; two parallel sets of columns and member rows did not.

#### `RESOURCE_OVERLAY_VALUE`

A consuming OU's own value for one templated field of a shared resource. The only table not hung off a policy. Cleanup deletes an OU's rows here when it loses sight of the resource, unless the type implements `OverlayCleaner`.

| Column | Type | Notes |
|---|---|---|
| `DEPLOYMENT_ID` | `VARCHAR(255) NOT NULL` | |
| `RESOURCE_TYPE` | `VARCHAR(64) NOT NULL` | |
| `RESOURCE_ID` | `VARCHAR(36) NOT NULL` | |
| `OU_ID` | `VARCHAR(36) NOT NULL` | The OU whose value this is |
| `FIELD_KEY` | `VARCHAR(255) NOT NULL` | |
| `VALUE` | `JSONB NOT NULL` | |
| `CREATED_AT`, `UPDATED_AT` | `TIMESTAMPTZ DEFAULT NOW()` | |

`PRIMARY KEY (DEPLOYMENT_ID, RESOURCE_TYPE, RESOURCE_ID, OU_ID, FIELD_KEY)`, also the upsert conflict target.

The SQLite script defines the same five tables, columns, constraints and indexes, the partial unique index on blanket targets and the two composite foreign keys included. The only differences are dialect spellings: `JSONB` becomes `TEXT`, and `TIMESTAMPTZ DEFAULT NOW()` becomes `TIMESTAMP DEFAULT CURRENT_TIMESTAMP`.

### API

The framework introduces no REST APIs. It is a Go package that resource types embed, and it mounts no routes of its own.

Each resource type that adopts sharing exposes its own endpoints, nested under the resource the policy concerns, so a sharing decision is addressed through the thing being shared. For resource servers:

| Method | Path | Purpose |
|---|---|---|
| `POST` | `/resource-servers/{id}/sharing-policies` | Record a sharing policy |
| `GET` | `/resource-servers/{id}/sharing-policies` | List sharing policies |
| `GET` | `/resource-servers/{id}/sharing-policies/{policyId}` | Get a sharing policy |
| `PUT` | `/resource-servers/{id}/sharing-policies/{policyId}` | Replace a sharing policy |
| `DELETE` | `/resource-servers/{id}/sharing-policies/{policyId}` | Delete a sharing policy |
| `GET` | `/resource-servers/{id}/overlay-rules` | Resolve what an OU may do |

### Configuration

Deployment level, under `resource_sharing`. The key is absent from the shipped deployment file, so it takes the value shown here.

```yaml
resource_sharing:
  # Lets an OU that sits inside a tree reach outside its own tree.
  # A root OU reaching another root is ordinary B2B sharing and is never gated by this.
  # A peer target landing in a different tree is gated by it whatever the issuer's position.
  allow_child_ou_cross_tree_sharing: false
```

Peer sharing will bring a setting of its own, deliberately not named here. It will also be bounded by resource-specific rules: a resource declares which OUs may be named as its peers, so the candidates are a named set rather than every OU in the deployment.

## Requirements

### R1. Share a resource without duplicating it

**Requirement:** An OU can give another OU access to a resource it owns. Afterwards there is still one resource, its core configuration still belongs to the owner, and the receiving OU can find out exactly what it may do with it.

- **AC1.1:** Given a resource owned by OU A, when A creates a policy naming child OU B, then B can see the resource and no second copy exists.
- **AC1.2:** Given B can see the resource, when B reads its effective terms, then B receives the rule per declared templated field and the identity of every policy that contributed.
- **AC1.3:** Given a resource nobody has shared, when its owner asks whether it is visible, then the answer is yes without any policy existing.

### R2. Reach is granted one step at a time

**Requirement:** A policy may only name OUs directly below its issuer. Reaching further happens two ways: the named OU is marked as bringing its subtree, or that OU issues its own policy.

- **AC2.1:** Given OU A, its child B, and B's child C, when A writes a policy naming C, then the request is refused with a client error.
- **AC2.2:** Given a policy from A naming B alone, when C is checked, then C cannot see the resource.
- **AC2.3:** Given a policy from A naming B and marking it as bringing its subtree, when C and anything below C is checked, then all can see the resource.
- **AC2.4:** Given a policy from A naming B alone, when B writes its own policy naming C, then C can see the resource.
- **AC2.5:** Given a policy from A naming B and marking it as bringing its subtree, when B writes a policy naming C, then the request is refused with a client error, because C is already reached by A's policy.
- **AC2.6:** Given a deployment-wide policy, when any OU it reaches writes a policy of its own, then the request is refused with a client error.

### R3. Coverage starts somewhere and is carried down, and revocation follows it

**Requirement:** An OU sees a resource for one of two reasons: something named it outright, or an OU above it on its branch can see the resource and reaches down. Deleting a share takes away whatever depended on it, and nothing more.

- **AC3.1:** Given an OU that can only reach the resource through a position above it, when that position is excluded or covered by nothing, then the OU cannot see the resource.
- **AC3.2:** Given an OU named outright, whether because it owns the resource or the owner aimed a share at it, when nothing above it can see the resource, then it can see the resource all the same.
- **AC3.3:** Given C sees the resource only through a policy B issued, when the policy that let B see it is deleted, then C can no longer see it.
- **AC3.4:** Given an OU loses sight of the resource, when the delete finishes, then its own values for the resource are gone and the type's cleanup hook has run once for that OU and resource, with no field list.
- **AC3.9:** Given a type declares a field with an editable default, and an OU wrote a value for that field while no policy named it, when the OU loses sight of the resource, then the cleanup still covers that field.
- **AC3.5:** Given an edit that adds an exclusion or drops a target, when an OU loses sight of the resource as a result, then the same cleanup runs as for a delete.
- **AC3.6:** Given a frontier OU, when it excludes one of its own children or sets tighter terms for it, then that takes effect, because the reach in question is the frontier OU's to give.
- **AC3.7:** Given an OU inside a covering policy's reach, when it attempts to exclude or retighten one of its own children, then the request is refused with a client error rather than accepted with no effect.
- **AC3.8:** Given a policy is deleted, when cleanup runs, then every OU that policy reached loses sight of the resource, and no OU it did not reach is touched.

### R4. Crossing between organization trees is gated and lands on a root

**Requirement:** A resource leaves its own tree only by landing on another tree's root. A root OU may always do that; an OU inside a tree may do it only when `allow_child_ou_cross_tree_sharing` is on.

- **AC4.1:** Given a root OU owns the resource, when it writes a policy naming another root, then the policy is created whether or not the setting is on.
- **AC4.2:** Given the owner has an ancestor and the setting is off, when it writes a policy naming a root in another tree, then the request is refused with a client error.
- **AC4.3:** Given a policy reaching another tree's root, when any OU below that root is checked, then it cannot see the resource until that root issues its own policy.
- **AC4.4:** Given the owner has an ancestor, when it names the root of its own tree, then the policy is created even with the setting off.
- **AC4.5:** Given the owner has an ancestor and the setting is off, when a resource file declares `allRoots` or `allOus`, then the declaration is refused as the file loads, because both scopes span every tree and the own-root exemption of AC4.4 cannot apply. With the setting on, both are accepted from an owner at any depth.

### R5. Peer-to-peer sharing between OUs that are not parent and child (future requirement)

**Requirement:** A resource owner can share with one named OU anywhere in the forest, a sibling or an OU in another tree included. The share reaches that one OU and nothing else.

Peer sharing is gated in two layers, and both arrive with the requirement rather than before it. A new deployment setting decides whether peer targets may be written at all; off means the selector is refused outright, whatever `allow_child_ou_cross_tree_sharing` says. On top of that, which OUs count as acceptable peers is bounded by resource-specific rules, so a resource is reachable only by the peers named for it rather than by any OU in the forest. Both the setting's name and the shape of those bounds are settled when this requirement is implemented, not before.

- **AC5.1:** Given the deployment setting for peer sharing is off, when any OU writes such a target, then the request is refused with a client error, whatever `allow_child_ou_cross_tree_sharing` says.
- **AC5.2:** Given it is on and an owner names a sibling in its own tree, when both are checked, then the sibling can see the resource and their shared parent cannot.
- **AC5.3:** Given peer sharing is on and an owner names an OU in another tree, when `allow_child_ou_cross_tree_sharing` is off the request is refused with a client error, and when it is on the policy is created.
- **AC5.4:** Given a policy reaching an OU as a peer, when that OU's root, parent or children are checked, then none can see the resource.
- **AC5.5:** Given an OU that received the resource rather than owning it, when it writes a peer target, then the request is refused, because only the owner may issue one.
- **AC5.6:** Given an owner names an OU below itself as a peer, when the policy is written, then the request is refused, because `child` and `childSubtree` already cover that ground.
- **AC5.7:** Given an OU reached as a peer, when it issues a policy down its own branch, then its subtree can see the resource.
- **AC5.8:** Given a resource bounds which OUs may be named as its peers, when a peer target names an OU outside that set, then the request is refused with a client error, even with the deployment setting on.

### R6. Deployment-wide reach is declarative only

**Requirement:** `allOus` and `allRoots` cannot be set through an API. They are available only to a policy a resource file declares.

- **AC6.1:** Given any OU, when it writes a policy carrying `allOus` or `allRoots` through an API, then the request is refused with a client error saying which of the two was asked for.
- **AC6.2:** Given a resource file declaring a policy with either scope, when the file is loaded, then the policy is created, subject to the cross-tree gate of AC4.5.
- **AC6.3:** Given an existing policy of any other shape, when an edit tries to turn it into one of these two, then the edit is refused by the scope-family and blanket rules.
- **AC6.4:** Given a policy naming specific roots, when it is written through an API, then it is created, subject to the cross-tree gate.
- **AC6.5:** Given a resource file declaring a policy without an id of its own, when the file is loaded, then the declaration is refused with a client error naming the resource.
- **AC6.6:** Given a file whose declaration is loaded twice, when it is replayed, then the second application replaces the first for that resource and initiating OU rather than adding a second policy, and does so even when the two carry different ids.
- **AC6.7:** Given a declared policy, when an edit or a delete is attempted against it through an API, then the attempt is refused with a client error, and no stored row is created for it.
- **AC6.8:** Given a resource and initiating OU the API has already recorded a policy for, when a file declares one for the same pair, then the declaration is refused rather than silently shadowing or replacing the stored policy.

### R7. Rule narrowing is delegated within bounds and never widens

**Requirement:** An OU can be given room to change a field, but only inside the bounds held by the OU that shared it. No OU passes on more than it holds.

- **AC7.1:** Given an OU that holds a field as not editable, when it writes a policy making that field editable for somebody else, then the request is refused with a client error.
- **AC7.2:** Given an OU bounded to a set of values, when it writes a policy allowing a value outside that set, then the request is refused with a client error.
- **AC7.3:** Given an OU whose own rule carries `excludedValues`, when it writes a policy dropping one of them, then the request is refused with a client error.
- **AC7.4:** Given a rule whose value sits outside its own allowed values, when it is submitted, then it is refused whatever any policy above it says.
- **AC7.5:** Given a rule naming a value the issuing OU is not entitled to reference, when it is submitted, then it is refused with a client error.

### R8. Exactly one policy covers an OU

**Requirement:** No OU is ever reached by two policies, nor by two targets of one policy, so its terms come from exactly one target and there is nothing to reconcile. Three rules keep it that way: only an OU a policy named and stopped at may issue a policy of its own; one OU holds at most one policy per resource; and the targets of one policy may not overlap.

- **AC8.1:** Given any OU that can see the resource, when its terms are resolved, then exactly one policy is named as the contributor, and the terms are those of the single target that reached it.
- **AC8.2:** Given an OU inside a covering policy's reach, when that OU writes a policy of its own, then the request is refused, so no second covering policy can arise.
- **AC8.3:** Given peer sharing is on, when a peer-reached OU, or any OU it shares down to, has its terms resolved, then AC8.1 still holds.
- **AC8.4:** Given a field no policy mentions, when terms are resolved, then the type's declared default comes back, marked as coming from the default rather than a policy.
- **AC8.5:** Given an OU that cannot see the resource, when its terms are resolved, then it gets empty rules rather than the declared defaults.
- **AC8.6:** Given a policy carrying a broad target and a narrower one the broad target already reaches, when it is written, then the request is refused with a client error.
- **AC8.7:** Given the broad target excludes the OU the narrower one names, when the policy is written, then it is created, and that OU holds the resource on the narrower target's terms while the rest hold it on the broad target's.
- **AC8.8:** Given a policy naming the same OU in two targets, whatever their scopes, when it is written, then the request is refused with a client error.
- **AC8.9:** Given a target whose exclusion names an OU only a sibling target reaches, when the policy is written, then the request is refused, because the exclusion withholds nothing on the target carrying it.

### R9. A blanket policy can only be made smaller

**Requirement:** A policy already reaching a whole family cannot be widened or turned into another shape. What stays open to it is editing its exclusions, which only ever changes the issuer's own reach, and narrowing a rule, which limits editability or the values on offer.

- **AC9.1:** Given a blanket policy, when an edit changes one of its blanket targets, then the edit is refused with a client error.
- **AC9.2:** Given a policy of either family, when an edit moves it into the other scope family, then the edit is refused with a client error distinct from the one a widening receives, and the same error whichever direction the conversion goes.
- **AC9.3:** Given a blanket target carrying a rule, when an edit omits or widens that rule, then the edit is refused.
- **AC9.4:** Given a blanket target, when an edit tightens its rule, then the edit succeeds.
- **AC9.7:** Given a policy holding a blanket target and a selective one beside it, when an edit widens the selective target's rule, adds a selective target, or drops one, then the edit succeeds.
- **AC9.5:** Given a blanket policy that excludes an OU, when an edit adds another exclusion or removes the existing one, then the edit succeeds, and removing one runs the same cleanup a delete does for the OUs that keep the resource as a result.
- **AC9.6:** Given a policy read at one version, when an edit is submitted against an older version, then it is refused with a client error.

### R10. A resource type onboards without the framework knowing its shape

**Requirement:** A new type gets sharing by registering one declaration of its fields. It then uses the same engine, storage and resolution as every other type, with nothing written specially for it.

- **AC10.1:** Given a type has registered its declaration, when a policy names one of its declared fields then the rule is stored, and when it names a key the type never declared then the request is refused with a client error.
- **AC10.2:** Given a type declares a hierarchy field and supplies its separator, when containment is worked out, then a path covers itself and everything beneath it, but not a sibling that merely starts with the same text.
- **AC10.3:** Given a type declares a dynamic-key prefix, when a rule names a key under that prefix, then it resolves to the declared field.
- **AC10.4:** Given a type implementing none of the **optional** capabilities, when policies are created and resolved, then everything not requiring one still works.
- **AC10.7:** Given a declaration that resolves no ownership, when it is registered, then registration is refused and the type stays unknown to the framework, so the first call naming it says so rather than answering its owner that the resource is not visible.
- **AC10.8:** Given a type declaring a hierarchy field, when it supplies no separator for that field or supplies an empty one, then every operation touching the field is refused rather than compared as raw text.
- **AC10.9:** Given a type implementing `MemberValidator`, when the validator refuses members, then the caller is refused without being told why; and when the validator itself fails, then the caller receives an internal failure rather than a refusal.
- **AC10.5:** Given a resource type that never registered, when a policy is created for it, then the request is refused with a client error.
- **AC10.6:** Given a type that implements `OverlayCleaner`, when an OU loses sight of a resource, then the type is asked to delete that OU's values and the framework's own overlay table is left alone.

### R11. Policies export and replay deterministically

**Requirement:** A resource's sharing can be exported, reviewed, kept in version control, then replayed to reproduce the sharing it was taken from.

- **AC11.1:** Given a resource with an owner policy and reshares derived from it, when the policies are exported, then the policy that let an OU see the resource always comes before the policy that OU issued.
- **AC11.2:** Given an export of a single resource, when it is replayed on its own, then every step is valid, with any policy whose parent is missing treated as a starting point.
- **AC11.3:** Given the same set of policies, when they are exported twice, then the two exports come out in the same order.

## Change log

| Version | Date | Change |
|---|---|---|
| 0.1 | 2026-09-22 | Initial specification. |
| 0.2 | 2026-10-08 | Targets become a flat list, each carrying its own exclusions and terms. Drops the three selector groups, the policy-level overlay rules and the policy-wide exclusion list. Scope values are camelCase; `ou` and `ou_subtree` become `child` and `childSubtree`, the old names reserved for peer targets. Adds the no-overlap rule. `RESOURCE_SHARING_POLICY_EXCLUSION` becomes `RESOURCE_SHARING_POLICY_TARGET_EXCLUSIONS` with `TARGET_ID`; `..._OVERLAY_RULE.TARGET_ID` becomes `NOT NULL`. |
