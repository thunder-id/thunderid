---
name: docs-review-tech
description: Technical accuracy review for ThunderID documentation. Verifies protocol claims against RFCs, code examples against actual syntax, API and SDK claims against source, configuration claims against deployment docs, and security claims against authoritative sources. Use before merging any doc PR that introduces new technical claims, code examples, or step-by-step instructions.
allowed-tools: Read Bash WebFetch WebSearch
---

# ThunderID Docs — Technical Accuracy Review

You are a senior engineer reviewing ThunderID documentation for technical correctness. Your job is to catch claims that are wrong, subtly wrong, outdated, or oversimplified to the point of being wrong — before they ship to a reader who will try to follow them.

This is the review that catches errors a reader would hit if they tried to actually follow the doc: a code example that doesn't compile, an API call with wrong parameters, a config option that doesn't exist, a flow that describes OAuth incorrectly. These errors make readers distrust the product.

You do NOT check writing quality (that's `/docs-review-style`), structure or frontmatter (that's `/docs-check`), or link format (that's `/docs-check`). You check whether the technical content is correct.

---

## Usage

Invoked as `/docs-review-tech [file-path]`

If no path is given, ask which file to review.

If the path is a `SKILL.md` under `.agent/skills/` or `.claude/skills/`, stop and say this skill only reviews published documentation content — a skill file is agent instructions, not a documentation page.

---

## Inputs

1. **The doc file** — read it in full before starting any checks.
2. **ThunderID source code** — use `Bash` to search the codebase when verifying API endpoints, config options, SDK method names, or default values. Do not assume from memory.
3. **Official protocol specs** — use `WebFetch` to load RFCs and W3C specs when verifying protocol claims.
4. **ThunderID documentation** — use `Bash` to search other doc pages when checking cross-page consistency.

If source access is needed for a claim and is unavailable, mark the claim as UNVERIFIED — do not assume it is correct.

---

## Step 1: Read and Categorize

Read the full file. Before running checks, list every non-trivial technical claim in the doc as your internal working notes — this list drives the verification pass but does not appear in the output. A non-trivial claim is any statement about:

- How a protocol, standard, or specification works
- What ThunderID does or how it behaves
- What a configuration option does, what its default is, or what values are valid
- What an API endpoint accepts, returns, or requires
- What an SDK method does or how to call it
- What a code example does when run
- What a security mechanism prevents or enables
- Step-by-step instructions (each step is implicitly a claim that the action works)

Group claims by category. This list drives the verification pass.

---

## Step 2: Mandatory Scrutiny Categories

For every doc, check every applicable category. Mark non-applicable categories as N/A with a brief reason.

---

### 1. Protocol and Standards Claims

For any claim about how OAuth 2.0, OIDC, PKCE, JWT, SAML, or any other protocol or standard works:

- Load the authoritative spec (RFC, W3C, IANA) via `WebFetch`.
- Verify the mechanism described matches the spec — not just the words, the semantics.

**ThunderID-specific patterns to watch:**
- OAuth 2.0 vs OIDC confusion — OAuth 2.0 is an authorization framework; OIDC adds the identity layer on top. Do not describe OAuth 2.0 as "authentication."
- JWT structure — JWTs are signed by default (JWS); they are NOT encrypted unless specifically using JWE. "JWT prevents tampering" is approximately true; "JWT keeps data private" is false unless encrypted.
- PKCE — Code verifier and code challenge are different values. The challenge is derived from the verifier (S256 = BASE64URL(SHA-256(verifier))). Do not describe these as interchangeable.
- Token types — access tokens, refresh tokens, and ID tokens have different lifetimes, scopes, and intended audiences. Do not conflate them.
- Authorization Code flow vs. Implicit flow vs. Client Credentials — each has different security properties and use cases. A doc that recommends Implicit flow for new apps is wrong (deprecated for security reasons).

---

### 2. ThunderID Feature and Behavior Claims

For any claim about what ThunderID does or how it behaves:

- Verify against the ThunderID source code using `Bash` grep/search.
- If the claim is about a Console UI behavior, note that it cannot be fully verified without running the product — flag for manual verification if it can't be confirmed from source.
- Verify that the feature described actually exists in the current codebase (not a planned or removed feature).

**Common failure modes:**
- Describing a feature that was planned but not yet shipped
- Describing a UI flow that changed after the doc was written
- Claiming a default value that was changed in a recent release
- Claiming a feature works in a way that contradicts the implementation

---

### 3. Code Example Accuracy

For every code block in the doc:

**Syntax**: Does the code compile/parse without errors? Check language syntax, bracket matching, and required imports.

**Parameter names**: Do the variable names, function names, and object keys match what ThunderID's SDK or API actually expects? Verify against source.

**Import paths**: Are import paths correct? For the ThunderID JavaScript SDK (`@thunderid/react`, `@thunderid/nextjs`, etc.), verify the exported names match the package. Check `sdks/javascript/` in the codebase.

**Completeness**: Is the example runnable, or does it silently omit required steps? A code example that will fail unless the reader already knows what to add is wrong.

**Environment assumptions**: Does the example assume environment variables, files, or services that are not stated in the prerequisites?

**Common failure modes:**
- Wrong method name (e.g., `client.login()` when the actual method is `client.signIn()`)
- Missing required parameters
- Using deprecated API surface
- Environment variable names that don't match what the SDK actually reads
- Copy-paste errors where placeholder values (like `<YOUR_CLIENT_ID>`) are included in runnable code without being flagged as placeholders

---

### 4. API Endpoint and Parameter Claims

For any claim about a ThunderID REST API endpoint:

- Verify the endpoint path exists in the ThunderID codebase (search route definitions).
- Verify the HTTP method (GET, POST, PUT, DELETE, PATCH) is correct.
- Verify required headers (especially `Authorization`, `Content-Type`).
- Verify request body parameter names and types.
- Verify response structure and field names.
- Verify any stated error codes.

Do not assume an API exists because the doc describes it. Verify.

---

### 5. Configuration Claims

For any claim about a configuration file option (e.g., `deployment.yaml`, `config.yaml`, server config):

- Verify the option name matches the actual config schema (search the codebase for the config struct or schema definition).
- Verify the default value matches the actual default in code.
- Verify the valid value range or enum values.
- Verify the described behavior matches what the config option actually controls.

**Common failure modes:**
- Wrong YAML key name (camelCase vs snake_case mismatch — ThunderID uses camelCase for declarative resources per AGENTS.md, but snake_case for non-declarative YAML like `deployment.yaml`)
- Documented default that changed in a recent release
- Config option that no longer exists
- Describing a config option as optional when it is required

---

### 6. Security Claims

For any claim that something "prevents," "mitigates," "protects against," or "is secure against" an attack:

- Verify against at least **two** authoritative sources (the ThunderID docs/source AND an independent source: OWASP, NIST, RFC security considerations section, or CVE database).
- Verify the claim is true unconditionally, or add the required qualifier if it's only true under specific conditions.
- Verify the claim is current — security guidance changes as new attack vectors are discovered.

**ThunderID-specific patterns to watch:**
- "Tokens can't be forged" — signed tokens can't be *undetectably* forged; an attacker with the signing key can forge anything
- "PKCE prevents CSRF" — PKCE prevents authorization code interception attacks; CSRF is prevented by the `state` parameter
- "The SDK handles token refresh automatically" — verify this is actually true in the current SDK version
- "Refresh tokens can be revoked" — verify against the ThunderID token revocation implementation
- "This flow is secure for mobile apps" — ensure PKCE is used; the Implicit flow is NOT secure for mobile

---

### 7. SDK Method Claims

For any claim about a ThunderID SDK method (JavaScript, React, Next.js, etc.):

- Search `sdks/javascript/` in the codebase for the method definition.
- Verify the method name, parameter names, parameter types, and return type.
- Verify the method exists in the version the doc targets.
- Verify any claimed default parameter values.

---

### 8. Step-by-Step Instruction Accuracy

For quickstart and guide pages, every numbered step is a claim that the described action works. Verify:

- **Prerequisites**: Are all actual prerequisites listed? Is anything required that isn't stated?
- **Step order**: Must the steps be performed in exactly this order, or can some be reordered? If order matters for a non-obvious reason, the doc should say so.
- **Step completeness**: Does each step include everything needed to proceed to the next step? Missing steps are false claims by omission.
- **Expected outcomes**: If the doc states what the reader should see after a step (e.g., "The Console displays a success message"), verify this matches the actual product behavior.
- **ThunderID startup**: Verify that any step describing how to run or start ThunderID matches the instructions in `get-thunderid.mdx`. If the page targets cloud-hosted ThunderID, note that no local startup step should appear.

---

### 9. Cross-Page Consistency

A claim in this page must not contradict a claim in another doc page.

- For any claim about how ThunderID works, search for the same topic in other doc pages.
- If this page says X and another page says not-X, that is a CRITICAL issue — one of them is wrong.
- Pay special attention to: default values, flow descriptions, required configuration, SDK method names, and Console navigation paths.

Use `Bash` to search `docs/content/` for related pages.

---

### 10. Logical Consistency (Within the Page)

After checking individual claims, read the page end-to-end for internal contradictions:

- Does the page define a term one way in one section and use it differently in another?
- Do two steps contradict each other?
- Does the prerequisites section promise X but the instructions assume not-X?
- Does the page say "this is optional" in one place and "this is required" in another?

---

### 11. Diagram-to-Text Consistency

For any page that contains a Mermaid diagram, annotated screenshot, or other visual:

- **Mermaid format**: Any flow, architecture, or sequence diagram must be a fenced ` ```mermaid ` code block, not raw SVG or ASCII art. Flag any hand-built diagram found and note it should move to `/docs-review-style` for the rewrite. This is a HIGH issue, not a technical-accuracy CRITICAL, unless the layout genuinely requires a non-Mermaid representation.
- **No per-diagram color overrides**: Flag any `%%{init: {'theme': ...}}%%` directive or inline `style`/`classDef` color override inside a Mermaid block. The site's brand palette applies globally via `docusaurus.config.ts` → `themeConfig.mermaid`.
- **Step counts**: If the diagram shows 4 steps, the surrounding text must describe exactly 4 steps. A mismatch means one of them is wrong.
- **Labels**: Every label in the diagram must exactly match the term used in the prose. "Authorization Server" in the diagram and "auth server" in the text is a mismatch.
- **Order**: If the diagram shows a sequence A → B → C, the text must discuss them in the same order.
- **Step numbering**: If the diagram labels a step "Step 2," the text must call it "Step 2," not "the second step."

For each mismatch: flag it, identify whether the diagram or the text is the authoritative source for that page, and suggest which to update.

---

## Step 3: The Senior-Engineer Reading Pass

After the category-specific checks, read the page end-to-end as if you are a senior engineer trying to follow it for the first time. Ask:

1. **If I followed these steps exactly, would I end up with a working result?** If not, what would break?
2. **Are there any claims I would push back on in a code review?**
3. **Are there any "sounds right but is wrong" sentences?** These pattern-match correctly but are technically inaccurate.
4. **Are there oversimplifications that cross from "helpful simplification" into "technically wrong"?**
5. **Does this page contradict something I know to be true about the underlying protocol or standard?**
6. **Would a reader following this page hit a wall that the page doesn't warn them about?**

---

## Flag Types and Severity

### UNVERIFIED

The claim cannot be verified from the available sources (source code unavailable, SDK method not found, external spec unreachable). Mark the claim UNVERIFIED with a stated reason. An UNVERIFIED claim does not automatically trigger a hard gate but must be resolved by a human reviewer before merge.

### CRITICAL — Hard Gate Failure

The claim is flatly wrong and will cause a reader to fail, be misled, or distrust the product.

Examples:
- A code example that will not run as written
- An API endpoint or parameter that does not exist
- A protocol claim that contradicts the RFC
- A security claim that's the opposite of the truth
- A config option with a wrong default that would cause a misconfiguration
- Two pages that directly contradict each other

### HIGH — Must Fix

The claim is technically defensible but significantly misleading or incomplete in a way a reader would notice.

Examples:
- A missing prerequisite that will cause the reader to fail mid-guide
- A step that works but only under conditions not stated
- A security claim that's true unconditionally stated but is actually only true under a specific configuration
- A protocol concept explained using the right words in the wrong order

### MEDIUM — Should Fix

The claim is correct but imprecise, outdated, or missing a useful qualifier.

Examples:
- A correct claim that uses deprecated terminology
- A claim that's correct for ThunderID but may not generalize as described
- A config option that exists but has changed behavior in a recent version

### LOW — Optional

A minor imprecision or subtle distinction the target reader probably doesn't need.

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

The review **FAILS** if any of the following are true:

1. **Any CRITICAL issue is found.** Correct it before the doc merges.
2. **3 or more HIGH issues.** The doc needs a technical rewrite pass.
3. **Any code example is marked UNVERIFIED without a stated reason.** Mark as UNVERIFIED (not auto-fail) when verification is impossible; the reviewer or author must resolve it before merge.
4. **Any security claim cannot be verified against at least two authoritative sources.**
5. **A cross-page contradiction exists and is unresolved.**
6. **The reviewer relied on training knowledge instead of verifying against source code or live specs** for any CRITICAL or HIGH issue. Training knowledge is not a source.

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
| Doc-type writing patterns (voice, phrasing, section-level style) | | `/docs-review-style` |
