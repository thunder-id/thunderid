# Account Locks and Suspensions Specification

- **Status:** Draft
- **Version:** 0.3
- **Related documents:** [Design discussion #5368](https://github.com/thunder-id/thunderid/discussions/5368), [Feature issue #5073](https://github.com/thunder-id/thunderid/issues/5073), [Threat model](threat-model.md)

## Summary

ThunderID needs to restrict account access without deleting identities or removing their credentials and assignments.

Two different situations require different controls:

- Repeated authentication failures require **automatic protection** against brute-force attempts.
- Suspected compromise requires **administrative containment** that stops account use while leaving the identity available for investigation and remediation.

This specification introduces automatic **locks**, administrative **suspension**, and the operations used to release them. It applies to users, agents, and applications, with automatic locking available for eligible user and agent sign-in methods.

The public account statuses are: `ACTIVE`, `LOCKED`, `SUSPENDED`.

Locks and suspensions are independent controls. A lock restricts new authentication and challenge dispatch according to the configured policy, leaves existing sessions and tokens alone, and permits recovery. A suspension restricts account use at every plane, and denies the identity in AuthZEN access decisions, until an authorized operator releases it. Its flow checks that the operator may suspend the target, revokes the covered tokens and terminates the sessions the identity already holds, and then commits the hold. Operators manage these restrictions through `suspend`, `unsuspend`, and `unlock` administration flows.

## Architecture

The identity governance service owns access decisions and lifecycle transitions. Authentication, session, token, and flow components enforce its decisions through shared interfaces.

```mermaid
flowchart TD
    subgraph ENGINE["Authentication and access enforcement"]
        AUTH["Default authentication provider"]
        GATES["Session, token, application,<br/>dispatch and recovery checks"]
        SDK["IdentityGovernanceProvider"]
        GATES --> SDK
    end

    subgraph ADMIN["Account administration"]
        FLOWS["Authorized administration flows"]
        EFFECTS["Revocation and<br/>session termination"]
        NOTICE["Holder notices"]
        FLOWS -->|"before the hold"| EFFECTS
        FLOWS -->|"after the hold"| NOTICE
    end

    AUTH -->|"check, count, reset"| GOV
    SDK --> GOV
    FLOWS -->|"lifecycle commands"| SDK

    subgraph GOVERNANCE["Shared governance authority"]
        GOV["Access decisions and lifecycle transitions"]
        POLICY["Trusted category and method policy"]
        GOV --> POLICY
    end

    POLICY --> CONFIG["Deployment defaults and<br/>accountAccess configuration"]
    GOV --> DATA["Resolved identity and runtime access"]

    subgraph STORAGE["Separate persistence ownership"]
        PROFILE["Entity sources<br/>Profiles and credentials"]
        RUNTIME["Runtime persistent database<br/>State, counters and lock history"]
    end

    DATA --> PROFILE
    DATA --> RUNTIME
```

### Component responsibilities

| Component | Responsibility |
|---|---|
| Identity governance service | Evaluate restrictions, count eligible failures, form and clear locks, apply suspension, and derive effective status. |
| Policy resolver | Resolve the category policy using trusted identity metadata. |
| Default authentication provider | Verify credentials, construct identity results from profile-only reads, report attributable failures once, and ask governance to admit the verified subject and clear its method history. |
| `IdentityGovernanceProvider` | Expose access checks and lifecycle operations to the embeddable engine. |
| Access enforcement components | Supply the target identity, operation, and applicable authentication method to governance; enforce the returned decision. |
| AuthZEN service | Ask governance whether the evaluated subject is suspended before evaluating its roles, from a read that writes nothing. |
| Entity sources | Own identity profiles and credentials. |
| Runtime store | Own lifecycle state, failure counters, lock history, and registered runtime attributes independently of profile write access and of the profile source. It lives in the runtime persistent database, beside revocation records and SSO sessions. |
| Administration flows | Authorize lifecycle commands and coordinate revocation, session termination, and notifications. |
| Management APIs and Console | Expose effective status, execute configured administration flows, and manage deployment-wide policy. |

The default authentication provider, the atomic authentication and passkey registration endpoints, the AuthZEN service, and the user and agent management reads call governance directly. Downstream engine components use `IdentityGovernanceProvider`. A completed recovery flow releases the subject's locks through that same port.

### Governance interfaces

The authority is reached through two interfaces. `IdentityGovernanceProvider` is the engine-facing port an embedded host implements, and it is what a flow executor calls. The internal governance service is what the full server composes, and it also owns authentication admission/reset and failure counting.

#### `IdentityGovernanceProvider`

| Method | Contract |
|---|---|
| `EvaluateAccess(entityID, plane, method)` | Report whether the identity may be used at that plane, for that authentication method. The method is ignored on planes that present none. A hold is reported **in the decision**, never as an error, so a definitive refusal is not mistaken for an operational failure. An empty `entityID`, or one that names no local identity, is admitted: a subject that is simply not governed is not the same as an authority that could not be read. An error means the authority could not answer, and the caller must refuse. |
| `AdmitSignIn(entityID)` | A session-plane check for a sign-in that is completing. When it admits, it attempts to clear a lapsed account-wide entry using the observed runtime revision. A live entry is never cleared. A conflict or failed clear does not reverse admission. |
| `RecordLogin(entityID)` | Record granted access when enabled for the identity category. A recording failure must not fail the grant. |
| `Unlock(entityID)` | Clear every lock, the one left by unsuspend included, and return the account to the bottom of the escalation ladder. Does not release a suspension. |
| `ValidateSuspend(entityID, operatorNote)` | Check, without writing anything, that the caller may suspend the identity with that note: the authority reads the identity and applies the same organization-unit boundary and self-target checks as `Suspend`. Those checks apply to users and agents; an application target is authorized only by the flow's permission check. An identity that is already suspended passes. A refusal or an undecidable check is returned as an error. The suspend flow calls it before any containment step, so nothing is revoked for a target it refuses. |
| `Suspend(entityID, operatorNote)` | Place the administrative hold on an active or locked identity. It authorizes again on the write. An authorized repeat on an already-suspended identity succeeds without rewriting its state, placement time or operator note, so a flow that re-runs containment on a suspended identity completes. The write is guarded on the identity not being suspended, so of two concurrent suspends the first placement and note are kept. `operatorNote` is optional operator text of at most 100 characters, the Console field's limit; an empty string records none, and a longer note is refused (`IGS-1004`) with nothing written. Revocation and session termination are composed by the flow, not performed here. |
| `Unsuspend(entityID)` | Release the hold, from `SUSPENDED` only. Lands a user on the non-expiring account-wide lock. An agent keeps any automatic lock still in force, so it lands on `LOCKED` or `ACTIVE`; an application, never locked, lands on `ACTIVE`. Revokes nothing. |

Counting is **not** on this port. A runtime caller able to increment another identity's counter could lock that identity without ever presenting a secret.

`Unlock` is on it, and a recovery flow reaches it through the same `IdentityGovernanceExecutor` an administration flow uses. The release is therefore a **node an author places**, not behavior welded into the credential write: a deployment decides where in its recovery flow the ladder is released, and a flow that omits the node keeps the lock. The flow-authoring documentation tells authors to place it after `CredentialSetter`; flow validation does not check this.

`Unlock` is also the only mode a non-administration flow may run. Flow validation accepts the executor only in administration and recovery flows, and `suspend` and `unsuspend` fault outside an administration flow, because a suspension reachable from an unauthenticated flow is a denial-of-service primitive.

#### Access planes

A caller names the moment it is checking at; the authority decides which holds apply there, so the rule lives in one place rather than being copied into each enforcement point.

| Plane | When it is checked | Holds that apply |
|---|---|---|
| `authentication` | Immediately after an authentication method has compared what it was given against an identity, whether the proof verified or failed | Administrative hold, and the lock on the presented authentication method. With `dispatch`, one of the two planes at which every automatic lock applies; at `recovery` only a password or client-secret lock does. |
| `dispatch` | Before a challenge is sent | Administrative hold, and the lock on the **challenge's own** authentication method. Never moves a counter. |
| `recovery` | Self-service recovery, including a proof verified in a recovery flow, and any challenge outside an authentication flow | Administrative hold, and the lock on a password or client secret presented here. Every other automatic lock gives way, in any configuration, because recovery is how a locked account gets out; a password can be guessed, so a recovery flow must not be an unlocked way to guess it. |
| `session` | A subject materializing from a session snapshot or a flow | Administrative hold only. An automatic lock never applies here. |
| `issuance` | A refresh grant minting new tokens from an existing grant | Administrative hold only. An automatic lock never applies here. The other grants that mint for a user subject rely on a suspension's revocation instead: authorization-code redemption matches the deny list at the code's creation time and CIBA redemption at the later of the authentication and authorization times, and token exchange validates the subject token it was handed. A code issued after the cutoff cannot exist for a held subject, because issuance refuses it. |
| `application` | An application resolving at a runtime funnel | Administrative hold. |

A challenge in a registration flow takes the recovery plane: it is addressed to someone who is not yet the account, so a lock on an existing identity has nothing to say about it.

#### Decision and refusal reason

The decision carries whether access is admitted and, on a refusal, the reason: the hold that refused it. **Admission is the only field a caller is required to read.** A refusal without a reason, which a custom governance provider may return, is refused exactly as for a wrong secret.

| Reason | Refused by | Stated as |
|---|---|---|
| `suspended` | A suspension, at every plane. | Account suspended. |
| `locked` | An automatic lock or the lock left by unsuspend, at the authentication and dispatch planes, and at the recovery plane for a password or client secret. | Account locked. |

Which planes a hold applies to is decided inside the authority, so a caller only maps a reason to its error. A reason a caller does not recognize is refused as a wrong secret, so a reason added later is not stated until each caller maps it.

#### Internal service and persistence

The governance service implements the port operations, a status reader the user and agent services use for management reads, and two internal authentication methods:

| Method | Sole caller, and why it is internal |
|---|---|
| `AdmitAuthenticationStep(entityID, method)` | The default provider after a successful proof outside a recovery flow; inside one the provider calls `EvaluateAccess` at the recovery plane, which clears nothing. Read one fresh runtime snapshot for authentication-plane admission and the optional method clear. Clear only the recorded method entry if its revision still matches. `AdmitSignIn` handles the entity entry at completion. A skipped or failed clear does not reverse admission; admission and clearing remain separate operations. |
| `RecordFailure(entityID, plane, method)` | The default authentication provider. Counting must follow attributable failure evidence that only the verifying layer holds. The plane is `recovery` inside a recovery flow and `authentication` otherwise, and a failure is counted only where the lock applies to its method. |

Governance reaches persistence through a narrow state interface: read the governed entity, read its profile without runtime state, increment a failure, form a lock, clear lock entries unconditionally or only if the revision is unchanged, record a login, set a suspension, and clear a suspension (writing the post-suspension lock in the same statement). Each is a targeted statement rather than a whole-document write, which is what stops a concurrent profile update from rolling a counter back.

The entity service does not reach governance at all. A credential write is a write: it records the new secret and its evidence, and it decides nothing about locks.

### Embedded engine and custom providers

The full server supplies the governance authority. Embedded hosts can supply their own provider or omit governance:

- **Provider omitted:** access checks are skipped and lifecycle commands are unavailable. The engine logs this at debug level at startup.
- **Provider configured:** access checks must succeed before access is admitted. An unavailable authority causes refusal.
- **No governed subject:** requests without a governed identity are distinct from failed identity reads. Application checks still apply where relevant.
- **Host actor provider:** with a governance provider configured, OAuth refuses a suspended application's or agent's client at client authentication, the authorization endpoint, the CIBA callback and UserInfo, whichever actor provider the host supplies.

Custom authentication providers govern their own identities during authentication. Shared downstream checks apply to the subjects and applications they produce when governance is configured.


## Detailed design

### Account states

| Status | Meaning | Persistence |
|---|---|---|
| `ACTIVE` | No applicable suspension or live lock. | Stored lifecycle state. |
| `LOCKED` | A live lock restricts one sign-in method or the whole account. | Derived from stored lock data and the current time. |
| `SUSPENDED` | An administrator has restricted account use. | Stored lifecycle state with suspension time and optional operator note. |

Effective status follows this precedence: **`SUSPENDED` > `LOCKED` > `ACTIVE`**. `LOCKED` is never written as a lifecycle state value.

A suspension applies when either the state is `SUSPENDED` or suspension circumstances are present. Partial updates must not release access. Any other stored state, including an empty or undefined one, is not a hold: other parties may use the column for their own purposes. Suspend writes `SUSPENDED` and unsuspend writes `ACTIVE`, replacing whatever value the column held. Unreadable required runtime data causes refusal, with the answer that path already gives when the entity itself cannot be read (see Refusal disclosure), never as a hold or a wrong credential. At OAuth client authentication that answer is `invalid_client` (401), as ThunderID already answers an unreadable client.

#### Access behavior

| Operation | `ACTIVE` | Sign-in-method lock | Account-wide lock | `SUSPENDED` |
|---|---|---|---|---|
| New authentication | Normal checks | Refuse the held method | Refuse every method | Refuse every method |
| SSO reuse | Normal checks | Allow | Allow | Refuse |
| Token issuance and refresh | Normal checks | Allow | Allow | Refuse |
| Self-service recovery | Allow | Allow, except a password or client secret, which meets its own lock | Allow, except a password or client secret | Refuse |
| Sign-in challenge dispatch | Allow | Refuse for the held method | Refuse | Refuse |
| Authorized profile and credential updates | Allow | Allow | Allow | Allow |
| Automatic expiry | Not applicable | At `unlockAt`, if finite | At `unlockAt`, if finite | Never |

Normal authentication and authorization requirements apply to every allowed operation. Locks neither terminate sessions and tokens nor refuse their continued use, in any configuration: a lock is produced by failed sign-ins, which an unauthenticated party can drive, so extending one to those planes would make it a denial-of-service primitive. Suspension refuses at every plane, and the suspend flow revokes the tokens and ends the sessions the identity already holds before it writes the hold.

### Automatic locking

#### Authentication methods

An **authentication method** identifies the service that verifies a secret. It is independent of a flow node or delivery channel. Operator-facing text uses **sign-in method**; configuration, API and event values use the identifiers below.

| Authentication method | Automatic behavior |
|---|---|
| `credential` | Lockable. Password, PIN, and secret-answer comparisons share this method. |
| `otp` | Lockable. Email and SMS one-time codes share this method. |
| `system_credential` | Not counted. A client-secret failure is attributable, but nothing is recorded and it can never form a lock. |
| `passkey`, `magiclink`, `federated`, `openid4vp` | Not automatically lockable because their failures do not provide attributable bad-secret evidence. |
| `entity` | Reserved key for the entity-wide counter and lock, which covers every method; not a credential type. |

Configuration cannot make an ineligible method lockable. An account-wide lock nevertheless refuses every sign-in method, including methods whose failures cannot create a lock. This includes a federated sign-in, which reaches a local account only through the link recorded for that external identity.

#### Failure attribution

Only a definite failed secret comparison against a resolved identity contributes to a counter. The authentication method returns evidence to the default authentication provider, which records the failure once.

The following do not increment a counter:

- Unknown or ambiguous identities.
- Missing credentials, expired codes, or malformed requests.
- Internal verification failures.
- Challenge generation or delivery.
- A secret presented while the account is locked or suspended. The failed attempt is refused with the hold instead.

A failed proof attributed to an identity is decided from the same read that would count it, wherever it can be counted: a password or one-time password at a plane where its lock applies. There, if the identity is suspended, or a lock that applies at that plane is live on its effective authentication method or account-wide, the attempt is refused with the hold and not counted, so a wrong secret and the correct one get the same answer, a live lock is never extended and its escalation history does not advance. Elsewhere a failed proof is neither read nor counted and keeps the plain failure: a wrong client secret, which never counts, and a wrong one-time code in a recovery flow, which the lock gives way to. A hold there would not change how many guesses are allowed. Both outcomes still run the comparison. A failure that cannot be written is logged and goes uncounted; it does not change the answer. A failure whose identity cannot be read is refused with a server error, as a correct secret is when that read fails. A locked password leaves the one-time-password counter running, because that is a different secret being guessed.

With account-wide granularity, lockable failures share one counter.

#### Threshold and escalation

A counted failure increments the effective authentication method's counter. The counter resets after the configured interval without a counted failure, measured from `lastFailedAt`. An interval of zero never resets it on time; a successful sign-in or a lock still does. Reaching the threshold forms a lock only when that method has no live lock.

Lock duration follows an ordered list:

1. The first lock uses the first duration.
2. Each subsequent lock uses the next duration.
3. The final duration repeats after the list is exhausted, until the consecutive-failure limit below forms a lock that does not expire.
4. A positive decay interval after a finite lock ends returns the next lock to the first duration.

A duration of `0` creates a non-expiring lock. A decay interval of `0` disables time-based decay.

A second count, `consecutiveFailures`, records every counted failure since the entry was last cleared. Neither the failure window nor a lapsed lock resets it; only clearing the entry does (see Expiry and reset), and a user's unsuspend, which replaces the lock state with the post-suspension hold. Under account-wide locking the count sits on the account-wide entry, so it counts every method's failures and a completed sign-in clears it. When it reaches the policy's `maxConsecutiveFailures`, the lock that forms is non-expiring, whatever the threshold and the duration list say, so slow guessing that stays under the threshold or waits out each lock still ends. A user then needs a completed recovery or an operator unlock, and an agent an operator unlock. A `maxConsecutiveFailures` of `0` sets no limit.

The atomic failure increment returns the counters, the consecutive count and the runtime revision used to calculate a lock episode. Formation matches that revision and the observed failure and lockout counts. If the guard loses, governance rereads state and reevaluates eligibility, with at most three formation attempts in total. Recorded failures remain if formation does not succeed.

#### Expiry and reset

A finite lock applies while the current time is earlier than `unlockAt`. Expiry changes the access decision without a cleanup write or timer job. Non-expiring locks use `9999-12-31T23:59:59Z` internally; public responses omit their expiry.

| Event | Entries cleared |
|---|---|
| Admitted authentication step outside a recovery flow | The presented sign-in method's recorded entry, including failure and escalation history and the consecutive count, if the observed revision still matches. |
| Completed sign-in (assertion node, atomic endpoints) | A lapsed account-wide entry if the observed revision still matches; never a live one. |
| Recovery flow release node | Every lock entry and its history. |
| Operator unlock | Every lock entry and its history. |

An admitted authentication clears the entry of the method it was admitted through, because presenting one correct secret proves control of one sign-in method. A recovery flow clears everything, because it proves control of the account: the flow decides what proof it demands, and the sign-in method the user happens to re-credential at the end of it says nothing about which locks they should still be serving.

Automatic method and completion clears match the runtime revision observed during admission. Any intervening runtime mutation, including unrelated activity recording, can skip the clear without retry or grant failure, preserving newer state; a later admitted operation can clear a remaining lapsed entry. Explicit recovery and operator unlock clear unconditionally. Admission and concurrent suspension remain separate operations.

A credential replacement outside a recovery flow clears nothing. An operator rotating a secret, a passkey enrolment and the holder changing their own password through the self-service credential update are all writes, and a write is not proof of control by the account holder. A profile update cannot carry a credential at all; it is refused.

**The recovery release is a node, and it consumes recorded proof rather than its position in the graph.** An executor that verifies control records the identity it proved on shared runtime data, which only an executor may write. `CredentialSetter` does this on a successful credential write, and any later executor that establishes proof may record the same value. The release node reads its subject from there and refuses when nothing has recorded one, which is what stops a release node placed before the proving step from admitting anyone who can name an account.

The default recovery flows carry the node. A deployment that authors its own and omits it keeps the ladder, and that is the cost of making placement an authoring decision.

Clearing entries preserves suspension and unrelated runtime attributes.

### Suspension and restoration

| Action | Users | Agents | Applications |
|---|---|---|---|
| `suspend` | Apply suspension | Apply suspension | Apply suspension |
| `unsuspend` | Replace suspension with a non-expiring account-wide lock | Remove suspension without adding a lock | Remove suspension without adding a lock |
| `unlock` | Clear all lock entries and history | Clear all lock entries and history | Not offered |

#### Suspension

Suspension records the time and an optional operator note of at most 100 characters. The note is explanatory text and does not affect access decisions. Releasing the suspension deletes both. Authorized operators can update the identity and its credentials during investigation. Neither a credential change nor a completed recovery removes suspension.

#### User restoration

Unsuspending a user atomically replaces suspension with a non-expiring account-wide lock. The lock entries recorded before it, with their escalation history, are replaced by that one entry. The user must complete credential recovery, or an operator must unlock the account, before sign-in resumes. The Console also offers both steps as one action, Unsuspend and unlock, which runs the unsuspend flow and then the unlock flow; they remain two operations with their own audit events, and if the unlock fails the account is left locked.

This transition applies even when automatic locking is disabled. It must not expose an interval in which neither restriction applies.

#### Agent and application restoration

Unsuspending an agent or application does not create a recovery lock. Any independent agent lock, and its history, remains in force.

Unlock does not remove suspension. Neither unsuspend nor unlock recreates revoked sessions, grants, or tokens. There is no administrative action to create an arbitrary lock.

### Access enforcement

| Enforcement point | Required check |
|---|---|
| Authentication admission | Check after credential verification and before admitting the identity, and check a failed proof attributed to an identity before counting it, where it can be counted, so a held account gets the same refusal for both. Count only eligible failure evidence. |
| Session persistence | Check before saving a fresh SSO checkpoint. Administrative hold only. |
| Authentication completion | Check before issuing a flow authentication assertion, including after SSO reuse, or returning a successful atomic authentication response. Administrative hold only. Both record `lastLoginAt` after a successful completion when the category enables it. A `client_credentials` token records it for the client's own identity after the token is built. |
| Token validation and redemption | Token builders perform no governance check. Refresh performs a fresh issuance-plane check after subject resolution. Token validation and code/CIBA redemption enforce suspension revocation when enabled and when the artifact can be bound to the subject. Redemption uses the OAuth criteria revocation enforcement (`excludeRevokedScopes` over `EnsureNotRevoked`): a code is checked at its creation time and a CIBA request at the later of its authentication and authorization times. Coverage limits are listed below. |
| Application admission | Check OAuth client resolution and new or continued application-bound flows. |
| Challenge dispatch | Check before a flow sends an OTP, magic link, or account-bound passkey challenge. The direct SMS OTP send endpoint is not checked; a code it sends to an account held for one-time codes is refused at verification. |
| Recovery | Refuse suspended accounts; permit locked accounts, their required recovery challenges, and every proof presented in a recovery flow except a password or client secret, which still meets its lock. |
| AuthZEN decision | Check the evaluated subject before its roles. A suspended subject is denied with the reason `Subject is suspended` in a single evaluation and in every otherwise valid batch evaluation for it; a batch item with a missing field or an unknown resource keeps that error context and is denied either way. An action search returns no actions for it. A lock is not a denial. The check reads the runtime state without writing, so a missing runtime row reads as the starting state; a failed read fails the request. Administrative hold only. |

A suspended application's refusal does not increment the user's failure counter. A withheld challenge is not sent and the step states the hold. Withholding never increments a counter.

#### Enforcement components

The authentication plane is enforced by the **default authentication provider**. A federated sign-in resolves its local account there, through the link recorded for that external identity, so it is admitted at the same point as every other method. When several local accounts hold the link, the flow binds the account the user picks, or the one a registration completes as, through the provider under the federated method, so it is admitted at the same point as a single holder. It is bound with the internal `provisionedEntityID` credential key, which also binds an account the flow has just created, and the provider admits both the same way: the federated method carries no lock of its own, so only a suspension or an account-wide lock refuses. A newly created account has neither, so its check always admits, at the cost of one governance read per created account. If that read fails, the provisioning step fails with its authentication error, as it does when the created account cannot be read, and the account stays created. An identity with no recorded link reaches a local account only by verifying that account through an ordinary sign-in step, which the provider admits like any other. The provider is the only component that holds attributable failure evidence, and it submits attributable failures and verified subjects to the governance service, of which it uses only failure recording, step admission and access evaluation. Governance owns admission and best-effort reset; the provider owns proof evidence and authentication error mapping. Both outcomes of a comparison are submitted: a verified proof for admission, where an admitted step clears its counter, and a failed proof attributed to an identity for counting, where governance decides the attempt from the same read and refuses a held identity without counting. A held account therefore gets the same answer whatever password or one-time password it presents where its lock applies. Unattributed failures (an unknown or ambiguous identifier, a fault, a malformed request) answer as before. A single method selection supplies both credential dispatch and the authentication method governance checks. Inside a recovery flow the provider submits every verified proof, and every failure, at the recovery plane instead of the authentication plane; governance decides which locks give way there. A one-time password, magic-link code, passkey, federated sign-in or verifiable presentation gets past a lock, so a holder can recover with any of them, while a suspension still refuses. A password or client secret still meets its lock, because it is reused at every sign-in and can be guessed. A failure is counted only where the lock applies, so a wrong recovery code is not counted, while a wrong password in a recovery flow is. Governance keeps this per method in one registry, and a method added later must be classified there. The flow engine marks a flow as recovery from its type, including a recovery flow another flow calls; a flow definition cannot claim it.

Flow executors enforce the remaining planes through one shared guard, so a held identity produces the same refusal at every site rather than eight independently chosen ones.

| Executor | Plane | Behavior when held |
|---|---|---|
| `IdentityGovernanceExecutor` | — | **Introduced by this feature.** Applies a lifecycle command; see Administration flows. |
| `PreSuspendExecutor` | — | **Introduced by this feature.** Checks a suspend target and publishes the containment plan before anything is revoked; see Administration flows. |
| `IdentifyingExecutor` | `recovery` | In a recovery flow's identify step only. Fails the step with the suspension stated; only a suspension reaches this gate. The credential, magic-link and provisioning executors reuse its lookup but not this check: their identities are checked by the default authentication provider, the dispatch guard or the credential write. |
| `CredentialSetter` | `recovery` | Refuses before a credential is written, in every flow type. On success it records the proven subject for a release node that follows; it releases nothing itself. |
| `OTPExecutor`, `MagicLinkExecutor`, `PasskeyExecutor` | `dispatch` in an authentication flow, `recovery` otherwise | Sends nothing and fails the step with the hold stated. A refusal that names no hold, which only a custom governance provider can return, sends nothing either but answers as a sent code or link does, so the step does not reveal the account. Nothing is counted. |
| `AuthAssertExecutor` | `session`, through `AdmitSignIn` | Refuses before an authentication assertion is issued. A server error while resolving the subject fails the step, and the assertion is signed for the same subject that was checked, so it is never signed for one governance was not asked about. When admitted, it attempts a revision-guarded clear of a lapsed account-wide entry and records `lastLoginAt` if the category enables it. |
| `SessionExecutor` | `session` | Refuses before a session checkpoint is saved. A subject that cannot be resolved is not refused here: the save is best effort, and every grant made from a saved session is checked again at the assertion step. |

A withheld dispatch is scoped to the **challenge's** sign-in method, not the one the caller is authenticating with, so a locked password does not withhold a one-time password. An unreadable authority refuses at every one of these sites with the answer the site gives when its own entity read fails, never as a withheld or held identity.

Every one of these sites states the hold: a holder who is told nothing arrived is not left waiting for a code that never comes. The one difference between the two holds is where the holder is left, and it is described under Refusal disclosure.

#### Refusal disclosure

A refusal of a held account states which hold applies, either that the account is temporarily locked or that it is suspended. Disclosure is fixed in code and not configurable. A stated hold leaves the holder on the sign-in step with the message, so it is shown where the holder is signing in rather than returned to the client as an error. For a lock, self-service recovery stays reachable, though a password checked inside a recovery flow still meets its lock; recovery is not available to a suspended account and states the suspension if it is tried. A federated sign-in, whose identity provider round trip is already spent, ends with the hold stated.

The answer is the same whatever password or one-time password was presented where its lock applies: a wrong secret and the correct one get the same stated hold, because a failed proof that can be counted is checked as a verified one is. A wrong client secret, or a wrong one-time code in a recovery flow, keeps the plain failure, as described under Failure attribution. A link holder the user picks among several accounts, which the flow binds without a secret, is refused the same way, and the flow ends with the hold stated, as a federated sign-in does. A stated refusal never identifies the sign-in method that is locked, the attempts remaining, the position in the lock progression, or when the lock lifts. It applies only to an identifier that resolves to an account: an unknown identifier gets the existing unknown-account answer, which this feature does not change. The answer does not depend on which sign-in method or endpoint was used. Token endpoint refusals never state a hold: refresh and code or CIBA redemption answer `invalid_grant`, and client authentication answers `invalid_client`. A suspended application is not an account holder's hold either: the authorization endpoint shows the application as unavailable; UserInfo refuses its tokens with `invalid_token`, the CIBA callback answers `unauthorized_client`, and token, PAR, introspection and revocation answer as for an unregistered client at client authentication; a flow refuses with `FES-1020` without saying why. OpenID4VCI and the flow metadata read do not check the hold: the suspension revokes the client's tokens, and the flow itself refuses. Logout is not refused, since it grants nothing.

Self-service recovery and passkey enrolment state the suspension too: a suspended account in a recovery flow is told it is suspended, and both passkey registration steps state the suspension, in a flow and on `/register/passkey/start` and `/finish`.

Stating holds is a deliberate trade. Sign-in and recovery already tell an unknown identifier apart from a known account, so stating the hold reveals no account that was hidden before; what it reveals is the hold state. Anyone who reaches a refusal, including a challenge dispatch that needs only the identifier, learns that the account is locked or suspended. The threat model records this as an accepted risk.

A refusal never exposes the operator note. Policy is selected from the trusted target category, never from a request parameter, flow input, or application setting. An unavailable configured authority refuses access with the answer the path gives for an unreadable entity: a server error at most steps, and that step's existing failure or no-delivery answer where that is what an unreadable entity gets. Every such outage is logged as an error. A subject with no local identity is not an outage: the check admits it, since a hold is stored on the local record.

### Administration flows

Lifecycle actions use the `ADMINISTRATION` flow type and `IdentityGovernanceExecutor`, which applies the lifecycle command. It is a single executor selected by node mode rather than three executors, because the three commands differ only in which port method they call. A suspend flow also uses `PreSuspendExecutor`, which prepares the suspension before any containment step runs. These are the two executors this feature introduces.

| `IdentityGovernanceExecutor` input | Contract |
|---|---|
| Mode | Explicitly `suspend`, `unsuspend`, or `unlock`. **No default mode is declared**, so flow validation rejects a node that omits it; none of the three is a safe thing to run by accident. |
| `subject` | Resolved target identity for `unsuspend` and `unlock`, **in an administration flow only**. Outside one the input is ignored and the subject comes from recorded proof; see below. In `suspend` mode the input is consumed and ignored: the target is the subject of the trusted plan `PreSuspendExecutor` published, and the node faults if no such plan, or no subject in it, is present. |
| `operatorNote` | Optional operator text of at most 100 characters. Consumed on every mode and recorded only by `suspend`, so a flow that collects it is not re-prompted; nothing branches on it, which is what makes it safe to accept from a flow input. The suspend pre-check reads it too, so a longer note is refused before anything is revoked. |

| `PreSuspendExecutor` input | Contract |
|---|---|
| `subject` | Required resolved target identity. The executor has no modes and runs in administration flows only. |

`PreSuspendExecutor` calls `ValidateSuspend` for the target. A refusal fails the flow with the governance failure and publishes nothing, so no token or session is touched for a target the operator may not govern. When the check passes, it publishes the revocation plan on executor-only shared runtime data; the containment nodes and the suspend node consume that plan. `IdentityGovernanceExecutor` publishes no plan in any mode. `unsuspend` and `unlock` have no plan at all, because releasing a hold is not a containment event, and that distinction is why suspend and unsuspend are separate flows rather than one flow with a mode.

The administration entry gate requires an authenticated caller with the root system permission. Default flows also validate permissions. Account ownership alone does not authorize lifecycle actions. The actor and automatic trigger are derived from trusted execution context.

#### Where the subject comes from, and what authorizes the operation

The same executor serves an operator and an account holder, and the two differ in both respects. Conflating them is the failure this split exists to prevent: an unauthenticated caller who could name a target would be able to release any account, and an operator whose context were elevated would escape the boundary check.

| | Administration flow | Recovery flow |
|---|---|---|
| Modes reachable | `suspend`, `unsuspend`, `unlock` | `unlock` only |
| Subject | The `subject` input; for `suspend`, the subject of the plan `PreSuspendExecutor` built from that input | The identity recorded as proven during this execution |
| Caller | Authenticated, holding the scope the flow's `PermissionValidator` declares | Unauthenticated by definition |
| Authorization | The flow's gate, plus the authority's organization-unit boundary check | The recorded proof, established before the release node runs |
| Context to the authority | The operator's own | Runtime privilege |

Runtime privilege on the recovery path is not an exemption from the boundary check but the only correct answer to it: there is no operator to check, and a boundary evaluated against an absent caller refuses every release.

#### Organization-unit boundary

`suspend`, `unsuspend`, and `unlock` check the caller against the subject's organization unit before they write. A suspension is checked twice: `ValidateSuspend` checks it before any token or session is revoked, and `Suspend` checks it again on the write. The subject's unit and category come from the authority's own read of the entity, never from the request, so the answer to the question cannot be supplied by whoever is asking. A check that cannot be decided refuses.

**The boundary applies to users and agents.** Governance checks user updates with the user resource/action and agent updates with the agent resource/action already used by their services. It refuses operator self-targeting before the authorization service can apply its owner shortcut. Applications have no system authorization resource or update action, so their lifecycle operations remain under the administration-flow gate.

The administration entry gate admits only a caller holding the root system permission, and such a caller is outside every boundary, so the check refuses no caller the gate admits.

#### Suspension sequence

The default user, agent and application suspend flows compose existing executors around the two new nodes, in the same shape as user deletion (a preparatory node, then token and session revocation, then the write):

| Step | Node | Executor | Purpose |
|---|---|---|---|
| 1 | `permission_validator` | `PermissionValidator` | Authorize the operator. |
| 2 | `pre_suspend` | `PreSuspendExecutor` | Check that the operator may suspend the target, then publish the revocation plan. |
| 3 | `revoke_tokens` | `CriteriaRevocationExecutor` | Revoke what the subject's tokens carry, established before the cutoff. |
| 4 | `revoke_sessions` | `SessionRevocationExecutor` | Terminate the subject's sessions. |
| 5 | `access_governance` | `IdentityGovernanceExecutor` (`suspend`) | Apply the hold to the plan's subject. |
| 6 | `notify_holder` | `EmailExecutor` | Send the suspension notification, best effort. |

The unsuspend and unlock flows run `IdentityGovernanceExecutor` under the same node id, `access_governance`. The user unsuspend and unlock flows then run `notify_holder`, best effort.

Suspend places a new hold on an active or locked identity. An authorized repeat on a suspended identity passes the pre-check, re-runs containment with a new cutoff, and then succeeds without rewriting the hold. Unsuspend requires a suspension; otherwise it is refused before a write. A subject that names no identity is refused as a client error, not reported as a server fault; a request that supplies no subject is prompted for one at the pre-check. Every refusal reaches the flow as one generic failure; a server error fails the flow as a server error. The plan is a boundary revocation (`revoke_before_action`, reason `identity_suspended`), the same shape as a client secret regeneration, with its cutoff at the time `PreSuspendExecutor` runs plus the JWT leeway (`jwt.leeway`, 30 seconds by default). The leeway covers the time from that node to the hold write, which is normally the few milliseconds of the two revocation writes; a grant checked just before the hold and signed just after it; and node clocks that run ahead. A token issued between the pre-check and the hold write carries an `iat` before the cutoff while the hold is written within the leeway, so it is revoked with the rest; a session created in that interval is refused at the session plane once the hold exists. A flow that reaches the hold write later than the leeway leaves tokens signed in between uncovered, and with a leeway of zero that interval is the flow's own duration; see Known limitations. Its criterion is the subject's entity ID, which every locally issued token carries as `sub`. Suspending an application, or an agent that owns an OAuth client, also revokes its client identifier, as deleting the application does, so the tokens obtained through that client are covered. Every suspension record lives for at least one week, or the refresh-token validity period when that is longer; suspending an application, or an agent with an OAuth client, keeps its records at least as long as the longest artifact that client can be issued. See Revocation record retention under Known limitations. A record's lifetime counts from the later of its write and its cutoff, plus the JWT leeway, so it outlasts the last token it covers.

Unsuspend writes no revocation: grants issued after release are newer than the cutoff, except within the leeway, where a sign-in is refused until it passes, and grants issued before it stay refused while the revocation record lasts. Unlock does not change revocation.

Token containment depends on two deployment settings. `oauth.token_revocation.enabled` must be `true`: it enables the authorization server's revocation enforcement as a whole (validation, introspection, refresh and exchange), not only the revocation endpoint. When it is `false`, the flow's revocation step can still store its criteria, but OAuth does not enforce them; only the refresh live check and application admission still refuse. ThunderID's own APIs use the separate `server.security.token_revocation.enabled` setting, which has its own synchronization delay. A successful revocation step is therefore not evidence that every consumer rejects the tokens.

#### Failure behavior and containment scope

The steps of the suspend flow are separate operations, not one transaction, and containment runs before the hold. A target refused at the pre-check is refused before anything is revoked. `Suspend` authorizes again on the hold write; if the caller's authority changes between the two checks, the write is refused after containment has run, and the account is left active with its covered tokens revoked and sessions ended, as for a failed hold write. If token revocation or session termination fails, the flow stops and reports the failure, and the account is not suspended. Some tokens or sessions may already have been withdrawn, which costs the holder no more than signing in again. The operator retries Suspend, which the Console offers on an active account. If the hold write itself fails after containment, the covered tokens are revoked and the sessions ended, the account is still active, and the operator retries in the same way. Once the hold is written, a failed notification does not undo it: the notification is best effort and last.

The hold therefore depends on containment succeeding: while the revocation store or the session store is unavailable, an operator cannot suspend an account at all. The runtime state that carries the hold lives in the same runtime persistent database as revocation records and SSO sessions, so these failures are largely correlated: when that database is unavailable the hold could not be written either.

Running suspend again on an already-suspended identity re-runs containment without rewriting its state, placement time or operator note; the revocation and session steps run again with a new cutoff and the suspend node writes nothing. This is needed only when containment is to be repeated on an account that is already suspended. It is available through the administration flow API; the Console offers no Suspend action on a suspended identity. A repeat can also fail, and it then leaves the hold and the containment already completed for it in place. Release does not verify containment completion. Unsuspending and suspending again is not a safe way to repeat containment, because it releases the hold in between. Repeated operations do not guarantee exactly-once notification delivery. Concurrent issuance remains a separate containment limitation.

Suspending an application revokes tokens whose subject is that entity and tokens bound to its client ID, including user tokens issued through that client. Suspending an agent revokes tokens whose subject is that agent and, when it has an OAuth client, tokens bound to that client's ID, including tokens the agent obtained as a client on behalf of users. A token that names the agent only as its actor (`act`) while bound to another client is not revoked. Tokens issued to another client, including downstream exchange results, are not revoked recursively. Downstream APIs that validate only token signatures and expiry can continue accepting issued tokens until they expire; immediate rejection requires validation against current revocation information. The remaining token gaps are listed under Known limitations.

### Notifications and operational records

#### Suspension, unsuspend and unlock notifications

Default suspend flows use `EmailExecutor` with the `account-suspended` template, as the last node and best effort, so a failed notice never undoes a committed hold: any failure to prepare or send it, a missing template or recipient included, completes the node without sending. A node that sends nothing reports `emailSent` as `false` in the flow's response, and the Console then tells the operator the operation succeeded but no notification email was sent. A server-side failure, such as a template that cannot be rendered or a send the server cannot complete, is logged as a warning; a missing recipient or email provider is not logged, since it is a configuration state the shipped flows start in. The node declares a required `email` input, as every default email node does. The executor never prompts for it: the input names the attribute, and the recipient is that attribute on the record of the identity the suspend node publishes after writing the hold, unless the execution already carries a value under the input's name, in which case that value is used. Deployments customize notifications through the administration flow. The shipped suspension notice states that an administrator suspended the account, sign-in is unavailable and existing sessions have ended; it directs the holder to an administrator, who alone can lift the suspension.

The default user unsuspend and unlock flows end with the same best-effort `notify_holder` node, using the `account-unsuspended` and `account-unlocked` templates. The unsuspend notice says the suspension is lifted and the account is still locked, and directs the holder to account recovery or an administrator. The unlock notice says the holder can sign in again. Unlock succeeds when nothing is locked, so it sends the unlock notice then as well. The agent and application suspend flows carry the same best-effort suspension node, which sends only when the record carries the named attribute; the agent and application unsuspend flows and the agent unlock flow have no notification node. The Console's Unsuspend and unlock action runs both flows, so the holder receives both notices. Each of these flow notices, suspension included, is sent through the email sender its node names (`senderId`). The shipped nodes name none, because no sender exists at install time, so an operator sets one on each flow before these notices are delivered; until then the flows complete and send nothing.

#### Automatic-lock notifications

Automatic user lock email is configurable and enabled in product defaults; delivery still requires a valid recipient and a default email sender in the `notification` server-config section. Governance schedules the notice after committing a new lock, renders a notification template, and sends it through that sender. The templates are named in the `accountAccess` section and are chosen by whether the lock expires and, where the section sets one, by the authentication method that locked; see [Automatic-lock email](#automatic-lock-email). The bootstrap names one pair for every lock on one sign-in method, `account-locked` and `account-locked-permanent`, and `account-locked-entity` and `account-locked-entity-permanent` for an entity-wide lock. The notice's wording lives in the template, so an operator can edit it; governance supplies only facts: the authentication method that locked (`authenticationMethod`, `credential` or `otp`, empty for an entity-wide lock) and, for an expiring lock, the committed episode duration (`lockDuration`) as a short English value and its expiry to minute precision (`unlockTime`), which the template states is UTC. The method is an identifier, not translated text; a deployment that words each method in its own language sets a template per method. The credential method covers passwords, PINs and other stored secrets. The shipped templates describe an expiring lock as temporary, say a permanent lock does not expire automatically, offer account recovery or administrator help to regain access, and ask a holder who did not try to sign in to report it to their administrator, then change their credentials and check their contact details once access returns.

Delivery follows these rules:

- Only the successful lock-formation writer schedules a notice.
- Delivery is asynchronous, through the default email sender of the `notification` section. At most 256 notices wait for delivery, two are sent at a time and each attempt is bounded at 30 seconds. A notice that finds the queue full is dropped and logged. Shutdown waits up to two seconds for the queue, then cancels the sends in flight and logs each notice still queued as dropped.
- The recipient is the user type's primary email on the trusted current user record. A user type without one, a record with no value for it, or a user type that cannot be read sends nothing, and no other attribute is tried.
- Missing or invalid recipients, an unreadable profile or user type, no configured or unavailable templates and delivery failures are recorded without affecting the lock or authentication response. A missing primary email mapping or value is recorded at debug level; a lock with no configured template is recorded as a warning; a failed profile or user type read is recorded as an error, and nothing is sent.

None of the following is part of this contract: a notification when recovery releases a lock, an expiry notification, an agent lock notification, an external HTTP callback, a per-notice choice of email provider, or a per-account limit on notices. Notice volume is bounded by the lock durations and by the deployment's rate limits on sign-in.

#### Operational records

Lock formation is logged at debug level with the target, its category, the authentication method, the lock count and the unlock time, and, when observability is enabled, published once as `IDENTITY_LOCKED` with the target, its category, the locked authentication method, the lock count, whether the lock is permanent and, for a lock that expires, when it lifts. Only the writer that commits the lock records it, and it names no actor, because failed sign-ins trigger it. Suspend, unsuspend and unlock are logged at debug level with the operator, target, category and outcome, refusals included, and published as administration events: `IDENTITY_SUSPENDED`, `IDENTITY_UNSUSPENDED` and `IDENTITY_UNLOCKED` on success, and `IDENTITY_SUSPENSION_FAILED`, `IDENTITY_UNSUSPENSION_FAILED` and `IDENTITY_UNLOCK_FAILED` on failure, each failure carrying a failure class (`not_authorized`, `not_applicable`, `no_subject`, `not_found`, `note_too_long` or `operation_failed`). A suspension refused at the pre-check is a failed suspension. A self-service release from a recovery flow is published instead as `IDENTITY_SELF_UNLOCKED` or `IDENTITY_SELF_UNLOCK_FAILED`, in `observability.authentication`, without an operator. The operator note is never logged or published, only whether one was given. Every server-config write, including the `accountAccess` section, is published as `SERVER_CONFIG_UPDATED` with the operator and the section, or `SERVER_CONFIG_UPDATE_FAILED` with the error code. The value is never recorded, since a section may hold secrets. A revocation record that cannot be stored is published as `TOKEN_REVOCATION_FAILED` with its reason and target, so a suspension blocked by a store outage can be alerted on. Events use the `observability.administration` category (the revocation failure, the self-service unlock and `IDENTITY_LOCKED` use `observability.authentication`) and are published when observability is enabled; retention and retrieval are the deployment's. Secrets and tokens must not be logged. Identifiers are masked in log lines. The events are the record of locks and lifecycle operations, so a deployment that needs one enables observability; the log lines are only for troubleshooting. These records do not introduce an audit-history API.

### Scope exclusions

This specification does not define:

- Additional lifecycle states or soft deletion.
- Contact verification.
- Administrator-initiated credential recovery.
- Organization-owned policies or delegated helpdesk permissions.
- Administrator-defined background lock-event flows or an audit-history API.
- A per-notice choice of email provider. Automatic-lock email uses the `notification` section's default email sender, and each flow notice uses the sender its node names.
- Normalization of unknown-account responses. Sign-in, recovery, and registration keep their existing answer for an identifier that names no account.

### Data model

#### Runtime record

ThunderID stores lifecycle state and server-owned runtime attributes in a dedicated `ENTITY_RUNTIME_DATA` table, one row per identity. The table lives in the runtime persistent database, the SQL store that also holds revocation records, revocation criteria, SSO sessions and consents, and not in the entity database. It is operational state that ThunderID owns, independent of where profiles come from. Database-backed and declarative identities use the same table, and nothing joins it to `ENTITY`, so an identity defined only in a declarative file still has a writable runtime row. The runtime persistent database's cleanup of expired revocation and session rows never touches this table.

Entity tables and declarative files keep ownership of profiles and credentials. Runtime writes work when the profile source is read-only.

| Column | Contract |
|---|---|
| `DEPLOYMENT_ID`, `ENTITY_ID` | Primary key, bound from trusted deployment context and the identity's stable identifier. |
| `STATE` | Runtime state, normally `ACTIVE` or `SUSPENDED`. Unknown state values are preserved; only `SUSPENDED` withholds access. `LOCKED` is derived from lock data. |
| `RUNTIME_ATTRIBUTES` | A JSON object holding `accessState`, whose members are the lock state, the suspension and `lastLoginAt`. It never carries the profile's own attributes. The credential-change marker stays in the entity's system attributes. |
| `REVISION` | Incremented by runtime mutations that change a row; lock formation and automatic method/completion clears match the observed value. |
| `CREATED_AT`, `UPDATED_AT` | Describe the runtime row. Runtime writes never change profile timestamps. |

Only resolved identities acquire runtime rows; an unknown username allocates nothing.

#### Access-state document

The `accessState` document stores failure history, lock episodes, and suspension circumstances:

```json
{
  "accessState": {
    "lock": {
      "authenticationMethods": {
        "credential": {
          "failureCount": 0,
          "consecutiveFailures": 5,
          "lastFailedAt": "2026-09-16T10:00:00Z",
          "lockCount": 1,
          "unlockAt": "2026-09-16T10:05:00Z"
        }
      }
    },
    "suspend": {
      "suspendedAt": "2026-09-16T10:01:00Z",
      "operatorNote": "Account under investigation"
    }
  }
}
```

Account-wide locking uses `lock.entity` with the same counter and expiry fields. Unsuspending a user replaces the lock member with an entity entry containing the non-expiring timestamp and internal reason `POST_SUSPENDED`.

A live entity entry takes precedence over method entries. Both can be present after a granularity change: a live entity failure lock is enforced under either granularity while the policy for the entity-wide entry is enabled, the post-suspension hold always is, and method entries are ignored under entity granularity. Suspension state and circumstances participate together in enforcement.

#### Write consistency

Runtime writes must preserve these invariants:

- Generic profile updates cannot overwrite runtime data or restore stale counters.
- Declarative resources cannot supply live failure counters, lock episodes, or caller-owned suspension evidence.
- Increments, lock formation, resets, and lifecycle transitions use atomic or guarded updates with bounded conflict handling.
- Concurrent runtime-row creation preserves the same guarantees as updates.
- User unsuspension replaces suspension with its recovery lock atomically.
- Credential replacement records the credential-change marker with the new credential, in one transaction. The lock clear a recovery earns is a separate operation, and a clear that fails on a server error fails the recovery step rather than being reported as complete.

Missing, corrupt, and unavailable runtime data require distinct handling. Initialization never overwrites an existing row; corrupt or unavailable data fails the read. A missing row cannot reconstruct a database identity’s prior hold.

#### Row initialization

The profile table `ENTITY` has no `STATE` column. New database identities start `ACTIVE`. A read that finds no runtime row reports the starting state: `ACTIVE` for a database identity, or a declarative file's optional top-level `state` (`ACTIVE` when missing or empty), with an empty attribute document. The file value is not otherwise validated: any string of at most 50 characters is stored as given and matched exactly, so only `SUSPENDED` seeds a hold, and that hold has no suspension time or note. A value that differs from `SUSPENDED` only by case or surrounding spaces is logged as a warning, with the resource ID, when the file loads, and a longer value fails the load, naming the resource. Management reads (single reads, lists, searches and exports) write nothing. Exports carry no state, and the import API ignores the top-level `state` of a user, agent or application document and says so in that document's result message: an imported identity starts `ACTIVE`, and an import that updates an existing one leaves its runtime row unchanged. A suspended account moved to another environment by export and import therefore arrives `ACTIVE`. The row is created with that starting state by the governance read behind every access check and every runtime write (sign-in admission, failure counting, the session, issuance, dispatch, recovery and application checks, and the lifecycle operations), and then read back; a concurrent seed does not overwrite a row another request created. The AuthZEN suspension check is the exception: it reads the starting state as a management read does and creates no row. Nothing else is read from the profile to seed the row. An `accessState` key in a profile's system attributes is ordinary profile data that nothing reads: governance reads lock and suspension state only from the runtime row.

Creating an identity creates its row, and deleting the identity deletes it. Because the entity and the runtime row live in two databases, these are two ordered writes rather than one transaction:

- **Create:** the entity is inserted and committed in the entity-database transaction, and its runtime row is then reset to an empty `ACTIVE` row. A runtime failure is logged and the create stands: a missing row reads as the starting state, `ACTIVE`, and the first governance read creates it. A duplicate identifier fails at the entity insert first, and an identifier a declarative identity holds is refused before the insert. These checks prevent an ordinary duplicate create from resetting an existing identity’s holds.
- **Delete:** the entity delete commits first, and the runtime row is deleted after it. A failure to delete the runtime row is logged and not returned, because the identity is already gone; the orphan row is reset by the runtime write of any later create at the same identifier. The runtime row is never deleted first, because a runtime delete followed by a failed entity delete would re-seed a held identity as `ACTIVE`.

#### Schema upgrade

Existing entity databases must drop their previous `STATE` column before running the current profile store, because profile inserts no longer supply that required column: `ALTER TABLE "ENTITY" DROP COLUMN STATE;` (PostgreSQL, and SQLite 3.35 or later). Existing runtime persistent databases must create `ENTITY_RUNTIME_DATA` from the shipped runtime persistent database script. Existing runtime rows are retained; initialization defaults do not migrate or overwrite them.

This release supports a stop-the-world upgrade: stop every node, run both statements, then start the new build. Nodes of the previous build and of this one must not run against the same databases at once. No rollback to a build that predates the runtime row is defined; to return to it, restore the entity and runtime persistent databases together from backups taken before the upgrade.

#### Backup and restore

The entity database and the runtime persistent database must be backed up and restored to consistent points. Restoring the runtime persistent database to an earlier point than the entity database can drop a suspension or lock, because a missing database runtime row is re-seeded as `ACTIVE`, while a declarative row uses its initial file state, or can bring back one that was since released. Deployments already need to restore the runtime persistent database consistently for revocation records, and the production backup set includes it.

#### Reads and caching

Enforcement and management-status reads load runtime rows from the runtime persistent database, in batches of 100 for lists, outside the profile and credential caches. Profile-only identity and attribute reads omit runtime state and cannot supply an access decision. A cached profile therefore never supplies stale runtime state, and runtime writes do not invalidate profile caches. The database accepts only a JSON object in `RUNTIME_ATTRIBUTES`, and the store refuses any other value, which fails the read and any list batch that contains it. An object whose access-state members have the wrong types fails enforcement with a server error, and a management read reports the identity as `SUSPENDED` with no suspension time. A user, agent or application update returns the updated record with its status, in the shape that category's read returns, so it reads the runtime row before it commits; while the runtime persistent database is unavailable, those updates through the management API fail and change nothing.

Runtime data and access decisions are not cached across requests or flow steps. A fresh read means a query to the configured runtime store; database isolation and topology still govern visibility. A clean successful provider authentication and its completion use one runtime read each. The authority reuses its admission snapshot only for that operation's optional clear. Profile-only reads do not initialize missing runtime rows. Declarative file state is initialization data only; an existing runtime row, including an explicit release, takes precedence over later file loads. Increment and formation statements are built once at startup for each registered authentication method, and any other method is refused; entity identifiers, timestamps, deployment identifiers and runtime values remain execution parameters.

### API

#### Management status

Application reads and updates expose only read-only `status.value`, either `ACTIVE` or `SUSPENDED`, with no details. User and agent reads and updates also expose status details, following the same precedence as the status value. Their `reason` is a system classification; arbitrary operator text is returned separately as `operatorNote`:

| Status | Details |
|---|---|
| `ACTIVE` | No additional details. |
| `LOCKED` | Reason `FAILED_ATTEMPTS` or `SUSPENSION_RELEASE_LOCK`, with `lockedMethods`. |
| `SUSPENDED` | Reason `ADMINISTRATIVE_SUSPENSION`, suspension time, and optional `operatorNote`. Suspension refuses new sign-in, and release deletes the note. |

```json
{
  "status": {
    "value": "SUSPENDED",
    "details": {
      "reason": "ADMINISTRATIVE_SUSPENSION",
      "since": "2026-09-16T10:01:00Z",
      "operatorNote": "Account under investigation"
    }
  }
}
```

Each `lockedMethods` entry contains a `method` and, for a finite lock, `expiresAt`. Account-wide locks use `method: entity`. An omitted expiry means no automatic expiry.

Lock circumstances are returned on authorized reads of the record, including the holder's own profile read (`GET /users/me`). `operatorNote` is returned only on authorized management reads and updates, never on the holder's own reads (`GET` and `PUT /users/me`) or on a refusal. Status reports live restrictions under the effective policy. Disabled method holds and method entries ignored by entity granularity do not contribute `LOCKED`. A live post-suspension hold is always reported, and a live entity-wide failure lock is reported under either granularity while its policy is enabled. User and agent reads and updates also carry a read-only `lastLoginAt`, the instant access was last granted, session reuse included. Application reads do not carry one.

#### Errors

Lifecycle operations run as flows, so a governance error never reaches the client by its own code. The `IGS` and `IGP` codes are client errors: an administration flow fails with `FET-1112`, and the code is logged at debug level. A recovery release that governance refuses completes the step and releases nothing, because the credential is already written and running recovery again would meet the same refusal. It runs with runtime privilege, so the only refusals it can meet are `IGP-1001` and an account removed after its credential was set (`IGS-1003`). A server fault fails the flow as a server error in both flow types.

| Code | Surface | Returned when |
|---|---|---|
| `IGS-1001` | Debug log | A lifecycle operation names no identity. |
| `IGS-1002` | Debug log | A lifecycle operation does not apply to the identity's current state, such as unsuspending an identity that is not suspended. |
| `IGS-1003` | Debug log | A lifecycle operation names an identity that does not exist. |
| `IGS-1004` | Debug log | A suspension's operator note is longer than 100 characters. |
| `IGP-1001` | Debug log | A lifecycle operation is requested while `identity_governance_provider.type` is `disabled`. |
| `FET-1112` | Flow failure | A governance node in an administration flow could not apply its operation, or a recovery release found no recorded proof. It names no cause. |
| `FET-1113`, `FET-1114` | Flow failure | A flow step was refused by a lock or a suspension. Passkey enrolment in a flow states only a suspension (`FET-1114`): with the shipped governance a lock does not stop an enrolment, and a lock refusal from a custom provider fails the step as a registration failure (`FET-1018`). |
| `AUTHN-1011`, `AUTHN-1012` | HTTP 401 | A direct authentication endpoint (credentials, one-time password, federated or passkey sign-in) was refused by a lock (`AUTHN-1011`) or a suspension (`AUTHN-1012`). `/register/passkey/start` and `/finish` state only a suspension with the shipped governance: a lock does not stop an enrolment, which already requires proof of the account. A lock refusal from a custom provider during enrolment is answered as an enrolment failure, `AUTHN-PSK-1002` with HTTP 400. The status is the one invalid credentials receive. |
| `FES-1020` | HTTP 403 | A flow was started or continued for a suspended application. It does not say why. |

#### Lifecycle operations

Lifecycle actions use existing flow resolution and execution APIs. There are no direct suspend, unsuspend, or unlock endpoints. Generic profile updates cannot set lifecycle state or runtime attributes, because those live only in the runtime row; an `accessState` key written into a profile's system attributes is stored as ordinary profile data and has no governance meaning.

#### Runtime configuration

| Endpoint | Behavior |
|---|---|
| `GET /server-config/accountAccess` | Return `readOnly`, `writable`, and `merged` configuration layers. `readOnly` holds only a declarative resource; the deployment base appears only in `merged`. |
| `PUT /server-config/accountAccess` | Replace the writable layer and return the recomputed layers. |

Existing management API authorization applies. `PUT` replaces the layer; omitted previous overrides are removed. The following request overrides only the user's default threshold, with all other values inherited:

```json
{
  "user": {
    "default": {
      "threshold": 6
    }
  }
}
```

The section validates supplied fields and the merged policy before persistence. Invalid input returns the configuration-validation error without changing stored configuration.

The resolver reads the effective section through the server-config service on every resolution, so a write applies on every node as soon as that node's configuration cache no longer serves the old value. Ordinary resolutions consult the section again; activity recording shares one resolved value within its operation. A failed read applies the deployment defaults for that resolution; there is no cached fallback. A configuration read fault does not itself refuse authentication.

### UI

#### Account status and actions

The Console displays effective status on user, agent, and application management views. Actions are available only when their configured administration flows resolve.

| Action | Confirmation behavior |
|---|---|
| Suspend | Explain the access restriction. Collect an optional operator note for users and agents only. |
| Unsuspend user | Ask the administrator to verify contact information and reset potentially compromised credentials first, and explain that account recovery or a separate unlock is required before sign-in resumes. |
| Unsuspend and unlock (users only) | Ask for the same checks first, and explain that the suspension and the lock are both removed and sign-in resumes at once. Runs the unsuspend flow, then the unlock flow. |
| Unsuspend agent or application | Ask the administrator to rotate potentially compromised secrets first, and release suspension without implying that independent locks are cleared. |
| Unlock user or agent | Runs without confirmation, since it only restores access; the menu states that it removes all locks. |

Active applications offer suspend; suspended applications offer unsuspend. Users and agents also offer unlock and suspend when locked. A suspended agent offers unsuspend only, and a suspended user offers unsuspend and the combined unsuspend and unlock; the plain unlock operation remains supported but is not offered by the Console in that state. There is no manual lock action or direct API fallback for unavailable flows.

#### Account security settings

**Settings > Account security** has two cards: **Automatic account lock** and **Lock and suspension flows**. Automatic account lock holds separate user and agent cards. Each opens with the effective policy stated as a sentence, and has these controls:

- Automatic locking enabled or disabled.
- **Only the affected sign-in method** or **All sign-in methods** (authentication method on the agent card).
- Failure threshold and inactivity window, including no time limit.
- Lock durations, including a rung held until the user completes account recovery or an administrator unlocks (an administrator only, for agents), and escalation decay.
- For users: whether to email the account holder when a lock forms.

The user card has one lock-email checkbox; it is the only lock email setting the Console shows, since the email goes to the user type's primary email and its templates are set through the server config API. Saves preserve untouched overrides and every stored value the cards do not show, the templates included, and do not pin inherited defaults.

There is no application lock card. When a declarative `accountAccess` resource exists, the server refuses every write, so the screen shows the settings disabled, hides Save, and says they are managed declaratively. Count and duration-unit controls have matching heights.

Saving preserves unrelated writable overrides and stores differences against lower layers. It must not copy the entire effective policy into the writable layer. Load and save failures remain visible and cannot be presented as successful changes.

**Lock and suspension flows** writes nothing. It lists the suspend, unsuspend and unlock administration flows configured for users, agents and applications in a table like the Flows list, with each flow's name, type and last update. A row offers view and edit, never delete, and opens the flow editor. The card says when an operation has no usable flow. Suspension and unlock themselves start from each record's page.

### Configuration

#### Scope and inheritance

Lockout configuration is deployment-wide and selected from the target's trusted category. Only `user` and `agent` can be configured: applications cannot lock, so an `application` block is rejected by the section.

The deployment key `identity_governance_provider.type` (default `default`) can be set to `disabled`, as upstream's entity and user management providers can. The flow engine and OAuth then get a governance port that admits every check and refuses lifecycle commands with `IGP-1001`, so the administration flows fail and a recovery flow's release step completes without releasing. The authentication provider, the direct authentication endpoints, AuthZEN and status reads still use governance, so locks still form and are enforced while no lock can be released early: an expiring lock lifts on its own, and a non-expiring one stays until the setting is reverted. A deployment that sets `disabled` must therefore also disable the lock policy of every category (`account_access.<category>.default.enabled: false`, kept off in the `accountAccess` section), which also stops enforcing the locks already recorded. Recording is partial too: only the direct authentication endpoints stamp `lastLoginAt`, while flow sign-ins, session reuse and `client_credentials` grants record nothing, so such a deployment should leave `recordLastLogin` off.

Configuration uses the camelCase `accountAccess` server-config section. Product defaults ship as the `account_access` block in `default.json`, which `deployment.yaml` can override; together they are the base. The lock email's template names are the one exception: they name templates the bootstrap creates, so they are not part of the deployment configuration (see [Automatic-lock email](#automatic-lock-email)). The base is checked at startup by the section's rules, on its own: an enabled policy in it must be able to form a lock without a later layer, and an invalid base stops the server. When a declarative `accountAccess` resource exists, it overlays the base field by field and locks the section: writes through the API are rejected. An invalid declarative `accountAccess` resource stops the server at startup. Otherwise the writable value overlays the base field by field. Removing the declarative or writable value returns to the base, which names no lock email template. In a split deployment the Control Plane serves the `accountAccess` and `notification` sections too, so the policy is authored there and reaches each Data Plane through export and import with the rest of the configuration. The Control Plane enforces nothing and holds no account state of its own, so a status it reports is the starting state, not the Data Plane's.

The Helm chart sets the base through the `configuration.accountAccess` value, in camelCase. The chart's values carry the product defaults; a key removed there is not rendered into `deployment.yaml`, so the product default applies. The API, the Console and a declarative `accountAccess` file mounted with the chart's declarative resources work as in any other deployment.

Within a category, `default` is the policy every sign-in method uses. Entity granularity counts it under the account-wide `entity` key; authentication method granularity counts it under the presented method's own key. Policy resolution can overlay a per-method policy on `default`, but no configuration layer carries one: a per-method block, such as a `scopes` key, is rejected as an unknown field.

#### Lockout settings

| Section key | Meaning and validation |
|---|---|
| `lockGranularity` | `authentication_method` or `entity`; default `authentication_method`. |
| `enabled` | Enables the automatic policy; explicit false is distinct from absence. A policy that leaves it out is enabled when it states any lock field (threshold, window, durations, decay or consecutive limit), and must then be able to form a lock or the write is refused; a policy that states none of them configures no lock. A policy that reaches enforcement unable to form a lock is treated as disabled. |
| `threshold` | A whole number. At least 1 for an enabled effective policy; 0 is accepted only with `enabled: false`. |
| `failureWindowSeconds` | Required inactivity interval for an enabled effective policy; zero means the counter never resets on time. |
| `lockDurationsSeconds` | Nonempty list for an enabled policy; each duration is nonnegative and within the bound below; zero means non-expiring. |
| `lockDecaySeconds` | Nonnegative interval after a finite lock ends; zero means no decay. |
| `maxConsecutiveFailures` | A whole number. Zero sets no limit; otherwise at least the threshold, since a lower value would make the first lock permanent. Counted failures since the entry was last cleared that form a non-expiring lock. |

Every key above is resolved per identity category, so a category may lock on a different threshold without changing its sibling. The policy fields sit under the category's `default` and apply to every sign-in method.

#### Activity settings

Recorded activity is configured beside the lockout policy rather than inside it: nothing branches on the value, and switching locking off must not silently stop recording. Both keys are per category, and product defaults state both for users and agents: recording off, with a 3600-second resolution. Omitting a key from an override inherits the lower layer; a key absent from every layer resolves to off and 3600 seconds. The Console does not offer these keys; they are set through the API, a declarative `accountAccess` resource, or the deployment configuration, which Helm renders.

| Section key | Meaning and validation |
|---|---|
| `recordLastLogin` | Whether granting access records `lastLoginAt`. Off in product defaults for users and agents. Omission in an override inherits the lower layer; when absent from every layer, it resolves to false. |
| `lastLoginResolutionSeconds` | Resolution window for changing the stored timestamp; default `3600`. Zero permits a change on every eligible call, except one storing the identical timestamp. |

The value is written by the flow's assertion node when access is granted, once per flow, by a subject verified in that run and by one restored from a session snapshot alike, so it means *access last granted* rather than *credentials last verified*. Each eligible call sends a conditional SQL update. The resolution predicate limits changed rows, not database round trips; a future timestamp is repaired on the next eligible call. A write failure never fails the grant. The atomic authentication endpoints (credentials authenticate, one-time password verify, federated finish and passkey finish) record it too, on every successful response, including one that skips the assertion, because each is a completed sign-in; a refused sign-in records nothing, and a failure to record never fails the response. A `client_credentials` grant records it for the client's own identity after the token is built, so an agent that only uses that grant is not reported as dormant; an application is never recorded, since recording is configured only for users and agents, and a refused token request records nothing.

`RecordLogin` resolves one activity-policy snapshot. If every category is off, it skips entity reads and activity SQL; configuration cache misses can still query the database. Otherwise it reads the trusted category from the profile cache/store and selects from that same snapshot. It never hydrates runtime state solely to record activity.

Validation rejects unknown fields, negative values, invalid enumerations, and enabled merged policies without a valid threshold, window, or duration list. Omitted values inherit. Explicit zero retains its documented meaning.

Every seconds value is also bounded above, at the largest whole second a duration can carry. Past that bound the conversion wraps negative, and a negative duration does not behave as a long one: it switches the window off, makes a rung non-expiring, and skips decay. A write past the bound is refused through the API and the declarative section, so the operator is told, and a deployment base past it stops the server at startup. Configuration given to the service without that check, as an embedding host may, is held at the bound when it is read.

The authentication method registry determines locking capability.

#### SSO reuse and token issuance

An automatic lock never refuses SSO reuse or token issuance, under either granularity. It is formed by failed authentication attempts, which anyone who knows an identifier can produce, so a lock that reached those planes would let an unauthenticated party end an account's live sessions and stop its clients refreshing. A lock refuses new authentication and challenge dispatch; it withholds nothing that was already granted.

Suspension is the control that reaches these planes, through revocation rather than a per-token state check. For artifacts covered by revocation enforcement, the token's `iat` is compared with the suspension cutoff. Redemption and exchange apply their respective revocation checks; coverage is limited by retention and consumer enforcement. The cutoff is the time the suspend flow's pre-check ran plus the JWT leeway, so a grant issued before the hold, or after it within what remains of that allowance, is revoked with it. No claim is added and no per-token record is kept.

Every locally issued token for a local identity carries the entity ID as `sub`; mapping the subject to another attribute is not allowed. Integration tests check this on the authorization code, refresh, token exchange, client credentials, introspection, UserInfo and native API paths, together with the rejection of each path's tokens after a suspension. A subject token from an external issuer is never matched against local revocation. The token issued in exchange for it is local and carries the external `sub`, so local subject revocation applies to it; see Known limitations.

No state check is added at signing. Each issuance path either checks the identity live shortly before signing (sign-in completion, refresh, client admission) or redeems a grant established before the hold (a code by its creation time, a CIBA request by the later of its authentication and authorization times, a subject token by its `iat`). A code issued after the cutoff cannot exist for a held subject, because issuance refuses it. A token signed between a passed check and a committed hold carries an `iat` at or before the cutoff when the hold is written within the leeway of the pre-check, so it is revoked with the rest.

Refresh resolves the subject, then makes one fresh governance read on the issuance plane, which admits automatic locks. Subject resolution can already have hydrated runtime state; the governance check does not reuse that snapshot. The built-in resource-server check matches the same `sub` and client identifier, with its snapshot synchronization delay. Offline JWT validation cannot see revocation.

#### Default policies

| Category | Enabled | Threshold | Failure window (seconds) | Lock durations (seconds) | Decay (seconds) | Consecutive failures |
|---|---|---|---|---|---|---|
| User | Yes | 5 | 3600 | 300, 900, 1800, 3600 | 86400 | 50 |
| Agent | Yes | 5 | 900 | 60, 300, 900 | 3600 | 50 |

Agents lock after 5 eligible failed proofs in a 15-minute inactivity window, with consecutive locks lasting 1 minute, 5 minutes, then 15 minutes, the last repeating (`[60, 300, 900]`), so no shipped agent rung is permanent; only the 50th failure with no success in between holds an agent until an operator unlocks it. For users the 50th such failure arrives after about seven hours of locks with the shipped rungs. Lock history decays one hour after a lock ends. Operator unlock clears it and its history. An admitted step attempts to clear its own method history using the observed revision. An uncontended successful clear resets that method's escalation history. Client-secret failures remain uncounted and never lock an agent or application.

Product defaults ship one policy per category, and that is the only configurable shape: every sign-in method uses its category's policy.

Applications have no automatic-lock capability. Disabling a policy bypasses the failure locks it governs, per-method or entity-wide, without rewriting stored history. It cannot remove suspension or bypass the post-suspension hold. Changing granularity must continue recognizing a live entity-wide entry while its policy is enabled.

#### Automatic-lock email

The notice is configured per category under `notifications.onLock.email` in the `accountAccess` section. Only the `user` category sends notices, through the template the section names for the lock and the default email sender of the `notification` section. Changes take effect without a restart: the notifier is built whenever its template, sender and profile dependencies exist, and reads the settings, the templates and the sender for each notice.

| Setting (section key) | Behavior |
|---|---|
| `enabled` | Master switch; enabled for users in product defaults. Explicit false disables delivery. |
| `templates.temporary` | Template for a lock that expires. The bootstrap sets `authentication-method-locked`. |
| `templates.permanent` | Template for a lock that does not expire. The bootstrap sets `authentication-method-locked-permanent`. |
| `templates.account` | `temporary` and `permanent` for a lock on every sign-in method. The bootstrap sets `account-locked` and `account-locked-permanent`. |
| `templates.methods` | `temporary` and `permanent` for a lock on one method, keyed by `credential` or `otp`. The bootstrap sets none, so both methods share the default pair, which names the method. |

An override field that is not set uses `templates.temporary` or `templates.permanent`. The templates are set only through the section, not the deployment configuration, and the bootstrap stores their defaults in the section's writable layer, as it stores the default flows. A template name defaults where its template is created: a deployment that skips the bootstrap has neither the templates nor their names, and sends no lock email. The lockout policy stays in the deployment configuration, so enforcement, the startup check and deployment overrides never depend on the bootstrap. A write refuses a template that is not a valid handle, or a `methods` key other than `credential` or `otp`, with the generic configuration-validation error `SCF-1003` (HTTP 400). The detailed validation message names the accepted method keys in the debug log; that detail is not included in the API error response. A `PUT` replaces the writable layer, so it must carry the templates to keep them; a declarative `accountAccess` resource is used in place of the writable layer, so it must name them too. An export carries them, and an import restores them. With no template for a lock, the notice is not sent. The recipient is not configurable: a write that carries a recipient key, such as `recipientAttribute`, is refused as an unknown field with the configuration-validation error (HTTP 400), and a declarative `accountAccess` resource that carries one stops the server at startup.

The Console edits the switch through the same configuration section, as a checkbox. The address is read from the stored record when the notice is sent; only the user type's primary email mapping is consulted, not its schema, so a value left on a record after its attribute leaves the schema is still used. The mapping does not verify that a stored address belongs to the account holder.

A missing default sender or failed delivery never prevents lock enforcement. With user notifications enabled by default, a usable recipient and a default email sender are still required to deliver a notice. Agents do not receive lock email.

The master switch controls delivery. The transport is the email sender named by `defaultEmailSenderId` in the `notification` server-config section, an `email-smtp` connection. A write that carries any other field is refused with `SCF-1003`, and a declarative `notification` resource that names a sender locks the section. The sender is not checked when the section is written or loaded: it is resolved for each notice, so setting or changing it needs no restart. A sender that does not exist, is not an email sender, or is one the notification service does not read (in composite mode declarative senders are read only in declarative mode) fails the delivery; with none set, the notice is recorded as not delivered. Each committed episode has one delivery attempt: dropped or failed notices are not replayed, and another attempt against an already-live lock creates no notice.

#### Lifecycle flow selection

Lifecycle flow handles are configured in the `flow` server-config section:

| Key under `flow` | Default handle |
|---|---|
| `userSuspendFlow.defaultHandle` | `default-user-suspend-flow` |
| `userUnsuspendFlow.defaultHandle` | `default-user-unsuspend-flow` |
| `userUnlockFlow.defaultHandle` | `default-user-unlock-flow` |
| `agentSuspendFlow.defaultHandle` | `default-agent-suspend-flow` |
| `agentUnsuspendFlow.defaultHandle` | `default-agent-unsuspend-flow` |
| `agentUnlockFlow.defaultHandle` | `default-agent-unlock-flow` |
| `applicationSuspendFlow.defaultHandle` | `default-application-suspend-flow` |
| `applicationUnsuspendFlow.defaultHandle` | `default-application-unsuspend-flow` |

A handle must resolve to an administration flow before the action is offered. Flow availability does not grant permission to execute it.

## Requirements

### R1. Independent locks and suspensions

**Requirement:** Account status and access decisions distinguish automatic protection from administrative containment.

**Acceptance criteria:**

- **AC1.1:** Given a suspended identity, when its status is read, then it reports `SUSPENDED`; authorized user and agent reads include suspension details, while application reads expose only the status value.
- **AC1.2:** Given a timed lock with no suspension, when its expiry passes, then that lock no longer restricts access or contributes `LOCKED` status, without a cleanup write.
- **AC1.3:** Given a suspension, when a credential changes, a recovery flow completes, a lock expires, or an operator unlocks, then the suspension remains.

### R2. Attributable automatic protection

**Requirement:** Only eligible, attributable failures form automatic locks, with one count per failed verification.

**Acceptance criteria:**

- **AC2.1:** Given an enabled lockable authentication method and a threshold of N, when N eligible failures occur without the inactivity reset, then a lock forms for the effective method.
- **AC2.2:** Given an unknown identity, expired OTP, malformed request, challenge-only operation, or verification fault, when authentication fails, then it creates no attributable failed-secret count.
- **AC2.3:** Given a live lock or a suspension, when a password or one-time password is supplied for the account where its lock applies (a sign-in flow, or a password in a recovery flow), correct or wrong, then it is refused with the same answer, nothing is counted, and the lock is not extended. A wrong client secret, or a wrong one-time code in a recovery flow, gets the plain failure and is not counted.
- **AC2.4:** Given a client-secret failure, when it is recorded, then nothing is counted and no lock can form from it.

### R3. Configurable granularity and escalation

**Requirement:** Lockout policy supports method and account-wide protection with bounded escalation behavior.

**Acceptance criteria:**

- **AC3.1:** Given authentication method granularity, when the credential method locks, then OTP and other methods are not locked by that episode, and sessions and issuance remain usable.
- **AC3.2:** Given entity granularity, when a lock forms, then every sign-in method is refused, and sessions and issuance remain usable. No configuration extends a lock to those planes.
- **AC3.3:** Given repeated lockouts without reset or decay, when new episodes form, then durations follow the configured order and repeat the final rung.
- **AC3.4:** Given a zero-duration rung, when it is selected, then time alone does not release the lock.
- **AC3.5:** Given an expired finite lock and an elapsed positive decay interval, when another lock forms, then it uses the first rung.
- **AC3.6:** Given a failure window of zero, when failures occur at any interval without a successful sign-in, then they keep counting and the threshold forms a lock.
- **AC3.7:** Given a `maxConsecutiveFailures` above zero, when that many counted failures at one sign-in method (under account-wide locking, at any method) occur with no success with that method, recovery or unlock in between, then the method is locked without expiry, even if a window passed or a lock lapsed between them; a success with that method, a completed recovery or an unlock resets the count, and a success with another method does not.
- **AC3.8:** Given a `maxConsecutiveFailures` that is negative, or above zero and below the threshold, when the section is written or the base loads, then it is refused.

### R4. Suspension and deliberate restoration

**Requirement:** Authorized operators can contain an identity and release restrictions through the defined lifecycle actions.

**Acceptance criteria:**

- **AC4.1:** Given an authorized suspend flow and an optional operator note, when it succeeds, then the identity is suspended, remains editable, and exposes any recorded note only on authorized user or agent management reads. Application status reads expose only the value.
- **AC4.2:** Given a suspended user, when unsuspend succeeds, then a non-expiring account-wide lock replaces the suspension without an unrestricted interval.
- **AC4.3:** Given a suspended agent or application, when unsuspend succeeds, then no new recovery lock is added and any independent agent lock remains.
- **AC4.4:** Given an authorized user or agent unlock, when it succeeds, then all lock entries and their history are cleared without changing suspension.
- **AC4.5:** Given an account owner without administration authority, when they attempt a lifecycle operation, then ownership does not authorize it.
- **AC4.6:** Given a user or agent subject outside the caller's organization-unit boundary, when suspend, unsuspend, or unlock is attempted, then it is refused and nothing is written; a suspend refused at its pre-check (`ValidateSuspend`) revokes no token and ends no session.
- **AC4.7:** Given an authorization decision that cannot be reached, when a lifecycle operation is attempted, then it is refused rather than permitted.
- **AC4.8:** Given a `suspend` or `unsuspend` node in a flow that is not an administration flow, when it runs, then the operation faults and no state changes.
- **AC4.9:** Given an already-suspended identity and an authorized caller, when suspend is repeated, then the original hold remains unchanged and the flow repeats containment with a new cutoff.
- **AC4.10:** Given a suspend, unsuspend or unlock, when it completes or is refused (a suspend also at its pre-check), then it is logged at debug level and, with observability enabled, published as the matching administration event with the operator, the target, its category and the outcome; neither carries the operator note.

### R5. Consistent access enforcement

**Requirement:** Restrictions apply at authentication, session, token, application, challenge, and recovery boundaries, and a suspension applies to AuthZEN decisions about the subject.

**Acceptance criteria:**

- **AC5.1:** Given a suspended subject, when it attempts authentication, session reuse, issuance, refresh, or self-service recovery, then access is refused.
- **AC5.2:** Given a suspended application, when a new or continued application-bound flow or OAuth client admission is attempted, then it is refused without incrementing a user's counter.
- **AC5.3:** Given a restriction covering a sign-in challenge, when a flow dispatches it, then no challenge is sent. The step states the hold. The direct SMS OTP send endpoint is not covered; see Known limitations.
- **AC5.4:** Given a configured authority that cannot read required state, when access is checked, then the request is refused rather than admitted, with the response the same path gives when the entity itself cannot be read: a server error on sign-in, flow, authorization, refresh and passkey enrolment paths, a failed lookup in recovery, the no-delivery answer at magic-link dispatch (the answer an unknown user gets there), the credential-write failure at the credential-setting step, a server fault at the lifecycle steps, and `invalid_client` at OAuth client authentication. It is never answered as a held account or a wrong credential. An identity with no local record is not an unreadable state: it has nothing to govern, so the check admits it and nothing is recorded against it, as for an identity an external authentication provider asserts without a local account.
- **AC5.5:** Given an embedded engine without governance, or a deployment with `identity_governance_provider.type: disabled`, when normal authentication runs, then the flow engine's governance checks are skipped and lifecycle commands remain unavailable; a recovery flow still completes, its release step releasing nothing. Startup logs this at debug level only.
- **AC5.6:** Given a federated identity linked to several local accounts, when the user picks one at sign-in or a registration completes as the one holder in the target organization unit, then that account is admitted at the authentication plane for the federated method as it is bound: a suspension, an account-wide lock and the lock an unsuspend leaves refuse it with the hold stated, and a lock on another sign-in method does not.
- **AC5.7:** Given a suspended subject whose roles allow an action, when a resource server asks the AuthZEN API, then a single evaluation and every batch evaluation for that subject are denied, with the reason `Subject is suspended` wherever the evaluation is otherwise valid, the other evaluations in the batch are decided as usual, and an action search returns no actions.
- **AC5.8:** Given a locked subject, or a subject with no runtime row, when a resource server asks the AuthZEN API, then the decision follows the subject's roles and no runtime row is created. If the runtime state cannot be read, the request fails with a server error.

### R6. Recovery and reset

**Requirement:** Proof of control clears relevant automatic protection without releasing administrative containment.

**Acceptance criteria:**

- **AC6.1:** Given a recorded method entry, when authentication is admitted and the observed runtime revision still matches at the clear, then that method's counter and escalation history reset. A clean account sends no clear statement.
- **AC6.2:** Given any live lock, including a post-suspension entity lock, when a recovery flow's release node runs after its credential write, then every lock entry and its escalation history clear while any suspension remains.
- **AC6.3:** Given a locked, unsuspended user, when self-service recovery is requested, then the lock does not prevent the recovery path or its required challenge.
- **AC6.4:** Given a live lock, when a credential is replaced outside a recovery flow, by an operator, by a passkey enrolment or by the holder's self-service credential update, then no lock entry clears.
- **AC6.5:** Given a recovery flow whose graph carries no release node, when it completes, then every lock entry remains.
- **AC6.6:** Given a release node reached before anything recorded proof of control, when it runs, then no lock entry clears and the step fails.
- **AC6.7:** Given a release node in a recovery flow, when the request names a different subject, then the named subject is ignored and only the proven identity is released.
- **AC6.8:** Given an automatic clear based on an older runtime revision, when a newer failure, lock, suspension or activity update commits first, then the clear changes nothing and does not retry or reverse admission.
- **AC6.9:** Given a completed sign-in, when the observed entity-wide entry is lapsed and its revision still matches, then the entry clears. Individual authentication steps never clear the shared entry; completion never clears a live entry.
- **AC6.10:** Given a locked user, when a wrong one-time password is submitted in a recovery flow, then nothing is counted; when a wrong password is submitted in a recovery flow, then it is counted as at sign-in.
- **AC6.11:** Given a locked password, when the correct password is presented in a recovery flow, including one a sign-in flow calls, then it is refused as locked.
- **AC6.12:** Given the account-wide lock an unsuspend leaves, when the holder proves control in a recovery flow with a passkey or a one-time password, then the proof is admitted.
- **AC6.13:** Given a locked user and a sign-in flow that calls a recovery flow, when the recovery proves control, writes the new credential and releases the locks, then the sign-in completes and the account is active.

### R7. Composed containment and notifications

**Requirement:** Lifecycle side effects follow committed restrictions without becoming the authority for access.

**Acceptance criteria:**

- **AC7.1:** Given the default suspend flow, when it completes, then the subject's sessions are terminated and covered grants/tokens are revoked through the existing services when revocation enforcement is enabled; retention limits still apply.
- **AC7.2:** Given a committed suspension and a later notification failure, when access is attempted, then the suspension still refuses access.
- **AC7.3:** Given an automatic lock, when it forms, then it neither terminates sessions nor revokes tokens, and it does not refuse their continued use.
- **AC7.4:** Given enabled user lock email and a valid recipient, when a new automatic episode commits, then only the successful formation schedules its best-effort notice; repeated failures schedule no new episode notice.
- **AC7.5:** Given a failed or skipped notice, when the authentication response is produced, then the lock remains and the response does not reveal the delivery outcome.
- **AC7.6:** Given an already-suspended identity, when an authorized suspend runs, then it succeeds, the placement time and operator note are unchanged, and tokens and sessions issued before it are revoked; given an identity that is not suspended, when unsuspend runs, then it is refused.
- **AC7.7:** Given a suspended application or agent and enabled revocation enforcement, when a token whose subject is the application or agent and that was issued before the suspension is validated, then it is refused.
- **AC7.8:** Given a suspension released later, when a grant established after the release is used, then it is accepted, and a grant established before the suspension stays refused while its revocation record is retained.
- **AC7.9:** Given a mail server that is misconfigured or unreachable, when a lock forms, then the lock is formed, enforced, and released exactly as it would be with notification disabled.
- **AC7.10:** Given a suspended application and enabled revocation enforcement, when a user access or refresh token issued through it before the suspension is validated, then it is refused, also after the application is unsuspended while the revocation record is retained; a token issued through it after the release and past the cutoff is accepted.
- **AC7.11:** Given an active identity, when token revocation or session termination fails in the suspend flow, then the account is not suspended and Suspend remains available for it.
- **AC7.12:** Given a suspend node with no plan published by `PreSuspendExecutor`, when it runs, then it faults and writes nothing; given a plan, the hold lands on the plan's subject and never on a `subject` input.
- **AC7.13:** Given the default suspend flow, when the hold is written, then the revocation cutoff is the time the pre-check ran plus the JWT leeway.
- **AC7.14:** Given an automatic lock, when its formation commits, then it is logged once at debug level and, with observability enabled, published once as `IDENTITY_LOCKED` with the target, its category, the locked authentication method, the lock count and when it lifts or that it does not, and no actor; a failure that forms no lock and a formation writer that loses the guard record nothing.
- **AC7.15:** Given the bootstrap's lock email templates, when a lock forms on one sign-in method, then the notice renders `authentication-method-locked` (or `authentication-method-locked-permanent`) with the method that locked; a lock on every sign-in method renders `account-locked` (or `account-locked-permanent`); a template set for the method under `templates.methods` replaces the default; and with no template for the lock nothing is sent and the lock is unaffected.
- **AC7.16:** Given stored lock email templates, when the Console saves the account security settings, or the section is exported and imported, then the templates are kept.

### R8. Dedicated runtime ownership

**Requirement:** Runtime restrictions remain writable independently of profile-source write access.

**Acceptance criteria:**

- **AC8.1:** Given a resolved database-backed or declarative identity and a read-only profile source, when runtime failures, locks, or resets are persisted, then only the dedicated runtime authority changes.
- **AC8.2:** Given concurrent eligible failures, when a threshold is reached, then no increments are lost and competing formation writes do not create duplicate episodes from one observation.
- **AC8.3:** Given a profile update concurrent with a runtime mutation, when both complete, then the profile update cannot overwrite runtime data or reset its counters.
- **AC8.4:** Given an unknown identifier, when authentication fails, then no runtime row is allocated for that identifier.
- **AC8.5:** Given an identity with no runtime row whose profile system attributes carry an `accessState` key, when its runtime row is first created, then the database row starts `ACTIVE`, or the declarative row takes only its initial file state with `ACTIVE` fallback, and the profile key is neither a lock nor a suspension.

### R9. Runtime policy and management visibility

**Requirement:** Operators can inspect restrictions and change lockout policy without replacing deployment defaults unintentionally.

**Acceptance criteria:**

- **AC9.1:** Given deployment settings and no section overrides, when `accountAccess` is read, then `merged` contains the effective deployment policy.
- **AC9.2:** Given a valid writable replacement, when it is saved, then the next policy resolution on every node uses it once that node's configuration cache no longer serves the previous value.
- **AC9.3:** Given an invalid effective policy, when it is submitted, then validation rejects it without changing persisted configuration.
- **AC9.4:** Given a locked account, when an authorized management read occurs, then it reports the live lock entries enforced by the effective policy and their finite expiries, omitting expiry for non-expiring entries.
- **AC9.5:** Given a held account, when it attempts sign-in with the correct secret, or a wrong password or one-time password, then its refusal states whether it is locked or suspended, the same for each of them, with the status a failed sign-in has, without revealing the operator note, the locked sign-in method or when the lock lifts; a wrong client secret gets the plain failure, and an unknown identifier keeps its existing answer.
- **AC9.6:** Given an `accountAccess` value with an `application` block, when it is submitted, then it is rejected; given one in a declarative account-access resource, then the server does not start.
- **AC9.7:** Given an accepted `accountAccess` write, when observability is enabled, then `SERVER_CONFIG_UPDATED` records the operator and the section, never the value; a rejected write records the error code.
- **AC9.8:** Given agent recording enabled, when an agent obtains a `client_credentials` token, then its `lastLoginAt` is recorded; a refused token request records nothing.
- **AC9.9:** Given a held user, agent or application, when an authorized update of its record succeeds, then the response reports the status a read would, in that category's shape, never one the request carried.

### R10. Console lifecycle controls

**Requirement:** The Console exposes the supported operations through configured flows and preserves configuration inheritance.

**Acceptance criteria:**

- **AC10.1:** Given an unavailable lifecycle flow, when an identity edit page loads, then that action is not offered and no direct lifecycle API fallback is used.
- **AC10.2:** Given a user unsuspend confirmation, when it is shown, then it explains that recovery or a separate unlock is required before sign-in resumes.
- **AC10.3:** Given the Account security settings page, when it loads, then it presents user and agent controls without an application-lock card.
- **AC10.4:** Given an edit to one policy field, when the Console saves, then unrelated writable overrides are preserved and inherited values are not copied wholesale into the writable layer.

## Known limitations

These are properties of the current design, recorded so that nobody relies on a guarantee it does not give.

| Limitation | Effect |
|---|---|
| Offline token validation | A resource server that validates JWT signatures and expiry itself never sees revocation. A suspended identity's access token works there until it expires. |
| ThunderID's own API check | Revocation reaches it after the snapshot synchronization interval, not immediately. |
| Revocation record retention | A suspension's records last at least one week, or the deployment refresh-token validity when that is longer, for users, applications and agents alike. An application's or agent's records also last at least its OAuth client's own artifact lifetime, as application deletion sizes its record. No maximum token lifetime is enforced, so a user's token issued through a client with a longer validity can outlive the user's suspension record; after release and record expiry such a token is accepted again. Keep token validities within a week where this matters. |
| Application and agent suspension | Suspending an application or an agent disables its client and the artifacts bound to it. Tokens that belong to another client, including ones obtained by exchange, are not revoked recursively. |
| Agent as actor | Revocation does not match the actor claim (`act`). A delegated token bound to another client that names a suspended agent only as its actor stays valid until it expires. Tokens the agent obtained through its own client are revoked by client ID, and refresh through that client is refused. |
| Partial containment | Containment runs before the hold, as separate operations. A revocation or session failure stops the flow with the account still active and some tokens or sessions possibly withdrawn; the operator suspends again. The hold therefore cannot be written while the revocation or session store is unavailable; the runtime state shares the runtime persistent database with both, so such outages are largely correlated. A repeat on an already-suspended identity re-runs containment through the administration flow API only, and release does not verify that a repeat completed. A failed revocation write is published as `TOKEN_REVOCATION_FAILED` and a failed session step as the flow's node failure, so a deployment can alert on a suspension that could not be placed. |
| Two-database create and delete | An identity and its runtime row are written in two databases, in a fixed order. Partial failure can leave an identity with no runtime row, which reads as its starting state, or an orphan row. If an identifier is recreated and acquires a hold before an earlier delete removes its runtime row, that delayed delete can erase the new hold; if it is recreated while an orphan row remains and the create's runtime write fails, the new identity reads the orphan's state. A missing database row then initializes as `ACTIVE`; a declarative row uses initial file state. |
| Backup and restore | Restoring the runtime persistent database to an earlier point than the entity database can drop a suspension or lock, or bring back one that was released. Back up and restore the two databases to consistent points. |
| Recovery release | The credential write and the lock release are two steps. If the release fails, the step fails and the holder runs recovery again; the new credential stays in place. |
| Recovery attempt limiting | A lock gives way to a recovery proof such as a one-time code by design, so it does not limit recovery attempts made with one, and a wrong recovery code is not counted toward the sign-in lock; a password in a recovery flow still meets its lock. ThunderID has no recovery attempt limit of its own: a one-time code is bounded only by its expiry and the flow's limit on codes sent. This feature neither introduces nor changes that; deployments limit recovery attempts with rate limiting in front of ThunderID, as the [production guidelines](https://thunderid.dev/docs/deployment/production-guidelines/#configure-rate-limiting) describe. |
| Concurrent token issuance | Parent admission, suspension and signing are not atomic. The cutoff includes the JWT leeway, so this is limited to a grant whose check and signing are separated by more than the leeway. |
| Cutoff leeway | The cutoff is taken when the pre-check runs, before containment, and the leeway must also cover the time to the hold write, normally milliseconds. A suspend flow that reaches the hold write later than the leeway leaves tokens signed in between uncovered; a leeway of zero puts the cutoff at the pre-check, so that interval is the flow's own duration. Until the cutoff passes, including after an unsuspend, a fresh sign-in is refused at code redemption and tokens issued then are rejected; the holder retries once it has passed. |
| External subject collision | A token issued by token exchange or a JWT bearer grant for an external subject carries that subject's `sub`. Validation matches subject revocation on `sub` alone, whatever the subject's origin (a JWT bearer token names its source in `idp`; an exchanged token records nothing), so if an external subject equals a local entity ID, suspending or deleting that local identity also refuses the tokens issued for the external subject until its revocation record expires. The effect is denial only, and it needs a trusted external issuer to assert a `sub` equal to a local entity ID. User deletion already behaves this way. |
| Direct SMS OTP send | `POST /auth/otp/sms/send` is not checked against governance, so a held account's phone can receive a code from it. The code cannot be used: the verify endpoint refuses an account held for one-time codes, as every direct authentication endpoint does. The endpoint sends to any well-formed number whether or not an account exists, so limiting delivery volume and cost is a rate-limiting concern for the deployment. |
| Concurrent policy edits | The last write wins. A save does not detect an edit made after its page loaded, so two operators editing the policy at once can overwrite each other. |
| Embedded engine | The engine gives its flows no revocation or session service. A suspend flow there that carries the revocation and session steps fails at them and writes no hold unless the host supplies both; a flow without those steps writes the hold but revokes nothing and ends no session. Its pre-suspend step resolves the OAuth client through the host's actor provider, so a suspension there revokes by client ID too. |

## Open decisions

| Area | Decision required |
|---|---|
| Administration delegation | Whether administration stays root-only. Delegation would first need owner shortcuts settled on deletion and an application resource and update action, so a boundary applies to application lifecycle operations. |

## Change log

| Version | Date | Change |
|---|---|---|
| 0.1 | 2026-09-17 | Initial specification. |
| 0.2 | 2026-10-19 | Aligned with the implementation after the review rounds, including the stated-hold codes `AUTHN-MGR-1013` and `AUTHN-MGR-1014` and the `identity_governance_provider.type` setting. |
| 0.3 | 2026-10-21 | Clarified template-setting validation responses and diagnostic detail. Renamed the lock email templates (`authentication-method-locked` for one sign-in method, `account-locked` for every method, set under `templates.account`), resolve the default email sender at delivery instead of on write, and report an ignored import `state` in the import result. Stated that a `disabled` governance port leaves locks forming with no early release, so a deployment that sets it must also disable the lock policy. A best-effort flow notice that sends nothing reports `emailSent` as `false` and the Console tells the operator; only its server-side failures are logged. The direct authentication endpoints state a hold with the authentication service's own codes (`AUTHN-1011`, `AUTHN-1012`), and passkey registration states only a suspension. Governance that is omitted or `disabled` is logged at debug level at startup, not as a warning. A recovery release that governance refuses completes without releasing, as it does under a `disabled` port; only a release with no recorded proof fails with `FET-1112`. |
