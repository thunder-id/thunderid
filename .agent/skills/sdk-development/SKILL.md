---
name: sdk-development
description: The contract for every ThunderID SDK and the workflow for changing one. Use whenever work touches a ThunderID client SDK or integration package in javascript-sdks, ios-sdks, android-sdks, or flutter-sdks. Covers writing a new SDK for a language or framework, adding or changing an operation, configuration key, error, or UI component, porting a capability between SDKs, writing and running SDK tests, validating a change against the specification and threat model, raising the cross-SDK pull requests, reviewing an SDK change, and editing the SDK development specification itself.
allowed-tools: Read Write Edit Bash WebFetch
---

# SDK Development

Every ThunderID SDK implements one contract, recorded in the SDK development specification.
Every SDK change goes through it: a new SDK, a capability added to an SDK that already exists,
an integration package, or a review of any of those.

A change is finished when it is implemented in every SDK where it applies, validated against
the specification and the threat model, covered by tests, and raised as linked pull requests
across the repositories it touches.

## 1. Load the specification

Read both documents before writing or reviewing SDK code. Do not work from another SDK's
source as the reference, and do not work from memory of this skill.

| Working in | Read |
|---|---|
| `thunder-id/thunderid` | `docs-internals/sdk-development/spec.md` and `docs-internals/sdk-development/threat-model.md` |
| An SDK repository | Fetch [spec.md](https://github.com/thunder-id/thunderid/blob/main/docs-internals/sdk-development/spec.md) and [threat-model.md](https://github.com/thunder-id/thunderid/blob/main/docs-internals/sdk-development/threat-model.md) |

The specification is the authority. This skill routes to the right part of it, holds the
checklist, and drives the workflow. Where the two disagree, the specification wins and this
file needs fixing.

This skill and the two documents live in `thunder-id/thunderid` and are maintained only here.
An SDK repository refers to them by link from its `AGENTS.md` rather than keeping a copy, so
there is one place to change when the contract moves. A copy of this file in an SDK repository
is a bug: replace it with a link.

## 2. Locate the SDK repositories

A cross-SDK change needs a local checkout of each repository. Never guess a path, and never
assume a path is correct because it was pre-populated as a working directory.

Read `~/.thunderid.dev.local/sdk-paths.json` if it exists:

```json
{
  "javascript": "/absolute/path/to/javascript-sdks",
  "ios": "/absolute/path/to/ios-sdks",
  "android": "/absolute/path/to/android-sdks",
  "flutter": "/absolute/path/to/flutter-sdks"
}
```

Ask the user for any ecosystem that is missing from the file, one question covering all of
them, and offer to record the answers. Ask again rather than reusing a recorded path that no
longer resolves.

**Verify every path before using it**, whether it came from the file or from the user:

```bash
git -C "<path>" remote -v
```

The origin or upstream must resolve to the matching repository name: `javascript-sdks`,
`ios-sdks`, `android-sdks`, or `flutter-sdks`. A path that resolves to some other repository,
or to an older single-language repository, is the wrong checkout. Stop and ask rather than
editing it. Write a path to `~/.thunderid.dev.local/sdk-paths.json` only after it verifies.

Also check each checkout before branching:

- `git -C "<path>" status --short` for uncommitted work. Leave it alone, and never stash
  without asking.
- `git -C "<path>" fetch upstream` and compare against the upstream default branch. A fork
  can be well behind, so branch from the upstream branch rather than from a stale local one.

## 3. Pick the path

### Writing a new SDK

1. Decide the layer and the parent, and whether the platform needs its own protocol
   implementation or bridges to existing Platform SDKs. See **Layers** and **Platform
   packaging and build**.
2. Confirm it is an SDK and not an integration. See **Integration packages**.
3. Propose it through a design discussion before implementation starts.
4. Implement configuration, client surface, error hierarchy, and the security floor.
5. Add UI components if the layer and platform call for them, with translations and flow
   element identifiers.
6. Add unit, component, and end-to-end tests.
7. Add pipelines, quickstart, reference, and at least one runnable sample.
8. Record the specification version in the SDK's `README.md`.

### Changing an existing SDK

1. Check whether the change touches anything the specification defines. A new operation, a
   configuration key, an error, a component, or a change to security behaviour all do.
2. If the specification does not cover the capability yet, update the specification in the
   same change set. An SDK must never be the only record of a contract.
3. Decide which sibling SDKs need the same change, and implement it in each of them.
4. Add or change the tests that cover the behaviour.
5. Validate, then raise the linked pull requests.

### Porting a capability between SDKs

Implement it against the specification, not by translating the source SDK line by line. Keep
the canonical name and the returned concept; let the signature follow the target platform.

### Reviewing an SDK change

Check the change against the specification's requirements, not against the habits of the
surrounding code. Where they disagree, the specification holds and the surrounding code is a
separate issue. Work the review checklist.

## 4. Review checklist

Apply the ones the change touches.

**Naming and surface**

- Operation names are the canonical ones, adapted only for the language's conventions.
- A returned concept matches what other SDKs return under the same name.
- Configuration keys use the canonical names. No second name for an existing concept.
- A key the platform cannot support is documented as unsupported, not silently dropped.
- Public surface exposes no parent-layer internals.

**Behaviour**

- The same call site works in both redirect and embedded mode.
- Any operation before initialization fails with a distinct documented error.
- `signOut()` with no session succeeds without raising.
- A UI framework SDK needs only its single primary entry point for a standard flow.

**Errors**

- Raised through the `ThunderIDError` hierarchy, with a stable code and the originating
  package as `origin`.
- No credential, token, or unmasked personal data in a message or a log line.
- Every code the SDK can raise is documented.

**UI**

- Components work with no configuration beyond the root provider.
- A component with non-trivial layout ships an unstyled `Base` variant.
- Flow steps render from the server response, never a hard-coded form.
- Every element rendered from a flow carries its identifier: `thunderid-field-<field id>` and
  `thunderid-action-<action ref>`, exposed through the platform's accessibility tree.
- No string literals in components. Strings come from a translation bundle with fallback
  resolution, and server-localized text is not translated again.

**Packaging**

- Minimum supported platform version declared, and not raised in a patch or minor release.
- The Core Lib product stays separable from the Platform product.
- A bridged SDK pins each native dependency to a version, implements no protocol logic of its
  own, and mirrors the client surface names across the channel.

## 5. Security and threat model verification

Run this whenever the change touches authentication, tokens, storage, credentials, logging,
or configuration that affects any of them. It is not optional for those changes.

**Check the security floor holds.** Each of these is mandatory and cannot be configured off:

- PKCE with S256 and a random state on every authorization request.
- State compared on callback; a mismatch fails authentication and stores nothing.
- Tokens in the platform's secure store. Never browser `localStorage` or `sessionStorage` as a
  default.
- ID token signature, issuer, audience, expiry, and nonce validated before acceptance.
- Refresh deduplicated to one in-flight request, refresh tokens rotated, stored tokens written
  atomically.
- Sign-out revokes server-side before clearing local state.
- Non-HTTPS `baseUrl` outside loopback rejected at initialization.
- Credentials submitted and discarded, never retained beyond the call.
- Tokens, credentials, and personal data masked or absent in logs, before any caller-supplied
  logger runs.

**Check the change against the threat model.** Read `threat-model.md` and answer:

1. Does the change touch one of the modelled interactions? Those are redirect authorization
   and callback, code exchange and token storage, credential submission in embedded mode,
   token attachment to outbound requests, and sign-out.
2. Does it weaken a mitigation the model relies on, or introduce a configuration key that
   could? If so, the model needs updating in the same change set, and the change needs a
   second look before it ships.
3. Does it add a new interaction, actor, or external dependency? Those need a new entry in the
   model.
4. Does it resolve or change one of the recorded residual risks?

**Update the documents.** A change that alters a trust boundary, an actor's entitlements, or
an interaction updates `threat-model.md`. A change that adds or alters a capability the
specification defines updates `spec.md` and adds a row to its change log. Both live in
`thunder-id/thunderid` and are reviewed as their own pull request.

**Never publish an exploitable, unmitigated finding.** If the review turns one up, route it
through a private GitHub Security Advisory on `thunder-id/thunderid` rather than describing it
in a public specification, issue, or pull request.

## 6. Validate

Run the checks locally before raising anything. Match what each repository's pull request
builder runs, so a local pass means a CI pass.

| Repository | Lint | Build | Unit tests |
|---|---|---|---|
| `javascript-sdks` | `pnpm lint`, `pnpm format:check`, `pnpm typecheck` | `pnpm build` | `pnpm test` |
| `ios-sdks` | `swiftlint lint --strict` | `swift build` | `swift test` |
| `android-sdks` | `./gradlew ktlintCheck` | `./gradlew build -x test` | `./gradlew test` |
| `flutter-sdks` | `flutter analyze` | `flutter pub get` | `flutter test` |

End-to-end suites, where the change affects authentication behaviour:

| Repository | Command |
|---|---|
| `javascript-sdks` | `pnpm test:e2e` from `tests/e2e` (Playwright) |
| `ios-sdks` | `Tests/e2e/run-e2e.sh` (Maestro) |
| `android-sdks` | `tests/e2e/run-e2e.sh` (Maestro) |
| `flutter-sdks` | `tests/e2e/run-e2e.sh` (Maestro) |

The end-to-end suites provision a real ThunderID server from the repository. They take time
and need a simulator or emulator on mobile, so run them when the change affects a flow, and
say so plainly when they were skipped rather than implying they passed.

**Test coverage the change needs before it is done:**

- Unit coverage for the success path and each error of every operation touched.
- A test that fails if a mandatory security behaviour is removed.
- Unit tests that pass with no network available.
- Reference fixtures, not self-generated output, where deterministic behaviour is shared
  across SDKs.
- End-to-end steps that select elements by flow identifier, never by visible text or view
  structure.

Report results honestly. A failing test is reported with its output, and a skipped step is
reported as skipped.

## 7. Raise the pull requests

A capability counts as delivered once it exists in every SDK where it applies, or once its
absence is recorded with a reason. Merging it in one repository is the start of that.

Only raise pull requests when the user has asked for them. For each repository the change
touches:

1. Branch from the upstream default branch, not from a stale fork branch.
2. Commit in the style the repository's own history and contributing guide use.
3. Fill that repository's own `.github/pull_request_template.md`. Do not substitute a template
   from another repository.
4. Open the pull request against the upstream repository.

Include a parity section in every body:

```markdown
## SDK parity

- [x] thunder-id/ios-sdks#412
- [x] thunder-id/android-sdks#233
- [ ] flutter-sdks: not applicable, the capability has no Dart surface
```

Because the numbers only exist once the pull requests do, open them all first, then go back
and fill in each body with the sibling links. Leaving that pass undone is the usual way a set
of pull requests ends up unlinked.

The parity check on each repository reads that section. It passes and applies
`sdk-parity-reviewed` when it finds a linked sibling pull request or a written reason, and
fails with a comment when it finds neither.

Valid reasons for stopping at one platform: the capability has no meaning on the others, the
platform primitive does not exist yet, the fix is confined to one platform's implementation of
shared behaviour, or the change does not reach the public surface. Lack of time is a deferral,
not a reason, and is recorded as a tracked issue in each repository that lacks the capability,
linked from the body.

Exempt from the question entirely: dependency updates, CI and tooling, documentation, tests,
and internal refactors.

## 8. Before calling it done

- Implemented in every SDK where it applies, or the omission is recorded with a reason.
- Specification and threat model updated if the change reached either.
- Lint, build, and unit tests pass in every repository touched.
- End-to-end run for changes that affect a flow, or an explicit statement that it was skipped.
- Quickstart, reference, and sample updated where the change is visible to an adopter.
- Pull requests opened, cross-linked, and each carrying its parity section.
