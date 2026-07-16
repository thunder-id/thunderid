---
name: docs-review-tech
description: Technical accuracy review for ThunderID docs: checks protocol/RFC claims, code syntax, API/SDK claims, config claims, and security claims against sources. Use before merging docs with new technical claims or code examples, or whenever asked to verify a doc's technical accuracy.
allowed-tools: Read Bash WebFetch WebSearch
---

# ThunderID Docs — Technical Accuracy Review

You are a senior engineer reviewing ThunderID documentation for technical correctness: catch claims that are wrong, subtly wrong, outdated, or oversimplified to the point of being wrong, before they ship to a reader who will try to follow them.

This catches errors a reader would hit trying to follow the doc: a code example that doesn't compile, an API call with wrong parameters, a config option that doesn't exist, an OAuth flow described incorrectly. These errors make readers distrust the product.

Not in scope: writing quality (`/docs-review-style`), structure/frontmatter/link format (`/docs-check`). Only whether the technical content is correct.

---

## Usage

Invoked as `/docs-review-tech [file-path]`. If no path is given, ask which file. If the path is a `SKILL.md` under `.agent/skills/` or `.claude/skills/`, stop: it's agent instructions, not a documentation page.

---

## Inputs

1. **The doc file** — read in full before checking.
2. **ThunderID source code** — `Bash` search for API endpoints, config options, SDK method names, default values. Never assume from memory.
3. **Official protocol specs** — `WebFetch` RFCs/W3C specs for protocol claims.
4. **ThunderID documentation** — `Bash` search other pages for cross-page consistency.

If source access is needed but unavailable, mark the claim UNVERIFIED rather than assuming correctness.

---

## Step 1: Read and Categorize

Read the full file. As internal working notes (not part of output), list every non-trivial technical claim: how a protocol/standard works; what ThunderID does or how it behaves; what a config option does/defaults to/accepts; what an API endpoint accepts/returns/requires; what an SDK method does; what a code example does when run; what a security mechanism prevents/enables; step-by-step instructions (each step implicitly claims the action works). Group by category — this list drives verification.

---

## Step 2: Mandatory Scrutiny Categories

Check every applicable category; mark non-applicable ones N/A with a brief reason.

### 1. Protocol and Standards Claims
For any claim about OAuth 2.0, OIDC, PKCE, JWT, SAML, or other protocols: load the authoritative spec (RFC/W3C/IANA) via `WebFetch` and verify the mechanism matches, semantically not just in wording.

ThunderID-specific watch-list:
- OAuth 2.0 is an authorization framework; OIDC adds identity on top. Don't call OAuth 2.0 "authentication."
- JWTs are signed (JWS) by default, not encrypted unless using JWE. "Prevents tampering" ≈ true; "keeps data private" is false unless encrypted.
- PKCE: verifier and challenge are different values (S256 = BASE64URL(SHA-256(verifier))) — not interchangeable.
- Access, refresh, and ID tokens have different lifetimes/scopes/audiences — don't conflate them.
- Authorization Code, Implicit, and Client Credentials flows have different security properties; recommending Implicit for new apps is wrong (deprecated).

### 2. ThunderID Feature and Behavior Claims
Verify against source via `Bash` grep/search. Console UI behavior can't be fully verified without running the product — flag for manual verification if source doesn't confirm it. Verify the feature exists in the current codebase, not planned or removed.

Common failures: a planned-but-unshipped feature; a UI flow that changed since the doc was written; a default value changed in a recent release; behavior contradicting the implementation.

### 3. Code Example Accuracy
For every code block, check: **syntax** (compiles/parses, brackets, imports); **parameter names** (match the SDK/API, verified against source); **import paths** (correct for `@thunderid/react`, `@thunderid/nextjs`, etc. — check `sdks/javascript/`); **completeness** (runnable, no silently-omitted required steps); **environment assumptions** (unstated env vars/files/services).

Common failures: wrong method name (`client.login()` vs. actual `client.signIn()`); missing required parameters; deprecated API surface; env var names that don't match what the SDK reads; unflagged placeholder values (`<YOUR_CLIENT_ID>`) left in runnable code.

### 4. API Endpoint and Parameter Claims
Verify against the codebase: endpoint path exists (search route definitions), HTTP method correct, required headers (`Authorization`, `Content-Type`), request body param names/types, response structure/field names, stated error codes. Never assume an endpoint exists because the doc describes it.

### 5. Configuration Claims
Verify against the config schema/struct: option name, default value, valid range/enum, and that the described behavior matches what the option actually controls.

Common failures: wrong YAML key casing (declarative resources use camelCase per AGENTS.md; non-declarative YAML like `deployment.yaml` uses snake_case); a default that changed in a recent release; a config option that no longer exists; describing a required option as optional.

### 6. Security Claims
For any "prevents/mitigates/protects against/secure against" claim: verify against at least **two** authoritative sources (ThunderID docs/source plus an independent one — OWASP, NIST, an RFC's security considerations, or CVE). Verify the claim holds unconditionally, or add the needed qualifier. Verify it's current — security guidance shifts as new attack vectors emerge.

Watch-list: "Tokens can't be forged" (signed tokens can't be *undetectably* forged; a key-holder can forge anything); "PKCE prevents CSRF" (PKCE prevents auth-code interception; CSRF is prevented by `state`); "The SDK handles token refresh automatically" (verify against the current SDK version); "Refresh tokens can be revoked" (verify against the actual revocation implementation); "This flow is secure for mobile" (only with PKCE — Implicit is not secure for mobile).

### 7. SDK Method Claims
Search `sdks/javascript/` for the method definition. Verify name, parameter names/types, return type, existence in the targeted version, and any claimed default parameter values.

### 8. Step-by-Step Instruction Accuracy
For quickstart/guide pages, every numbered step is a claim the action works. Verify: **prerequisites** (all actual ones listed, nothing unstated required); **step order** (must it be exact — if a non-obvious ordering matters, the doc should say so); **completeness** (each step gives everything needed for the next — a missing step is a false claim by omission); **expected outcomes** (stated results match actual product behavior); **ThunderID startup** (matches `get-thunderid.mdx`; a cloud-hosted-ThunderID page shouldn't include a local startup step).

### 9. Cross-Page Consistency
Search `docs/content/` via `Bash` for the same topic elsewhere. If this page says X and another says not-X, that's CRITICAL — one is wrong. Watch especially: default values, flow descriptions, required configuration, SDK method names, Console navigation paths.

### 10. Logical Consistency (Within the Page)
Read end-to-end for internal contradictions: a term defined one way then used differently; two steps that contradict; prerequisites promising X while instructions assume not-X; "optional" in one place and "required" in another.

### 11. Diagram-to-Text Consistency
For any Mermaid diagram, annotated screenshot, or other visual: **format** — must be a fenced ` ```mermaid ` block, not raw SVG/ASCII art (flag hand-built diagrams for `/docs-review-style` to rewrite; HIGH not CRITICAL unless the layout genuinely needs a non-Mermaid form). **No per-diagram color overrides** (`%%{init...}%%` or inline style/classDef — brand palette applies globally via `docusaurus.config.ts` → `themeConfig.mermaid`). **Step counts**, **labels**, **order**, and **step numbering** in the diagram must exactly match the prose (e.g. "Authorization Server" in the diagram vs. "auth server" in text is a mismatch; "Step 2" must be called "Step 2," not "the second step").

For each mismatch: flag it, identify whether the diagram or the text is authoritative, and suggest which to update.

---

## Step 3: The Senior-Engineer Reading Pass

Read the page end-to-end as if following it for the first time. Ask: Would following these steps exactly produce a working result — if not, what breaks? Any claim you'd push back on in code review? Any "sounds right but is wrong" sentences? Any oversimplification that crosses into technically wrong? Does anything contradict what you know about the underlying protocol? Would a reader hit an unwarned wall?

---

## Flag Types and Severity

**UNVERIFIED** — can't verify from available sources (source unavailable, method not found, spec unreachable). State the reason. Doesn't auto-gate, but a human must resolve it before merge.

**CRITICAL (hard gate)** — flatly wrong, will cause failure/misleading/distrust. E.g.: a code example that won't run; a nonexistent endpoint/parameter; a protocol claim contradicting the RFC; a security claim that's backwards; a wrong default causing misconfiguration; two pages directly contradicting each other.

**HIGH (must fix)** — technically defensible but significantly misleading/incomplete in a way a reader would notice. E.g.: a missing prerequisite that causes mid-guide failure; a step that only works under unstated conditions; an unconditionally-stated security claim that's actually conditional; a protocol concept in the wrong order.

**MEDIUM (should fix)** — correct but imprecise, outdated, or missing a qualifier. E.g.: correct claim with deprecated terminology; a ThunderID-correct claim that may not generalize as stated; a config option whose behavior changed in a recent version.

**LOW (optional)** — a minor imprecision the target reader probably doesn't need.

---

## Output Format

```
Reviewing: docs/content/guides/getting-started/connect-your-application/react.mdx
Doc type: quickstart

Claims identified: [N]

CRITICAL ISSUES

C1: [Short title]
  Location: [Section or line]
  Draft says: "[verbatim quote]"
  What's wrong: [clear technical explanation]
  Correct: [what it should say]
  Source: [URL or codebase path verified during review]
  Suggested rewrite: "[replacement text]"

HIGH ISSUES

H1: [Short title]
  [same format]

MEDIUM ISSUES

M1: [Short title]
  [same format]

LOW ISSUES

L1: [one-line description and optional suggested rewrite]

LOGICAL CONSISTENCY
  [internal contradictions found, or "none found"]

CATEGORY COVERAGE
  ✅  Protocol/standards claims: [summary or N/A]
  ✅  ThunderID feature claims: [summary or N/A]
  ✅  Code examples: [summary or N/A]
  ✅  API endpoint/parameter claims: [summary or N/A]
  ✅  Configuration claims: [summary or N/A]
  ✅  Security claims: [summary or N/A]
  ✅  SDK method claims: [summary or N/A]
  ✅  Step-by-step instruction accuracy: [summary or N/A]
  ✅  Cross-page consistency: [summary or N/A]
  ✅  Logical consistency: [summary or N/A]
  ✅  Diagram-to-text consistency: [summary or N/A]

─────────────────────────────────────
Result: PASS / FAIL
[C: N critical · H: N high · M: N medium · L: N low]
```

---

## Hard Gate Rules

The review **FAILS** if any of:
1. Any CRITICAL issue found — correct before merge.
2. 3+ HIGH issues — needs a technical rewrite pass.
3. Any code example marked UNVERIFIED without a stated reason (a stated reason is fine; the reviewer/author still resolves it before merge).
4. Any security claim not verifiable against at least two authoritative sources.
5. A cross-page contradiction exists and is unresolved.
6. The reviewer relied on training knowledge instead of verifying against source/live specs for any CRITICAL or HIGH issue — training knowledge is not a source.

---

## Scope Boundary

| Check | This skill | Other skill |
|---|---|---|
| Protocol/spec accuracy | ✅ | |
| Code example correctness | ✅ | |
| API/SDK accuracy | ✅ | |
| Config option accuracy | ✅ | |
| Security claim accuracy | ✅ | |
| Cross-page consistency | ✅ | |
| Writing quality, voice, AI vocab | | `/docs-review-style` |
| Frontmatter, heading hierarchy, links | | `/docs-check` |
| Doc-type writing patterns (voice, phrasing, section style) | | `/docs-review-style` |
