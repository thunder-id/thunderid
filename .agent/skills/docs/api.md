# ThunderID Docs — API Documentation Review

Reviews API documentation for consistency and technical accuracy. **Technical accuracy is a hard gate here** — not advisory, and not softened the way `style.md`'s judgment calls are.

ThunderID has two genuinely different kinds of "API documentation," each with its own ground truth and its own Part below:

- **Part 1 — REST API specs**: `api/*.yaml` (OpenAPI 3.0.3), rendered at `docs/content/apis.mdx` via a Scalar viewer. Contributors edit the YAML directly; `apis.mdx` itself is static scaffolding with nothing to review.
- **Part 2 — SDK API reference pages**: `docs/content/sdks/<sdk>/apis/**/*.mdx`, hand-written Markdown documenting one class/component/function per file.

## Usage

Read when the user asks to review, check, or verify API documentation, an OpenAPI spec, or an SDK reference page. Also read automatically as part of `review.md` when the target path matches `api/*.yaml` or `docs/content/sdks/*/apis/**` — there it hard-gates the same way `tech.md` does, not the way `seo.md`'s advisory pass does.

- **A specific file given** → review that one file (spec file, or one SDK reference page).
- **No file given, but the request names a domain** ("review the user API," "check the node SDK reference") → review every file under that scope.
- **No file, no domain** → ask which: the whole REST surface (`api/*.yaml`), the whole SDK reference tree, one SDK, or one file.

Not in scope: writing quality (`style.md`), generic frontmatter/heading/link structure (`check.md` — still applies to the `.mdx` side as normal). This is specifically about whether the API documentation is *complete and correct*.

---

# Part 1: REST API Specs (`api/*.yaml`)

## Ground Truth

The Go backend registers every real route via `mux.HandleFunc` in `backend/internal/<domain>/init.go` (91 `init.go` files exist; ~40 register HTTP routes). Example:

```go
mux.HandleFunc(middleware.WithCORS("GET /users", userHandler.HandleUserListRequest, opts1))
mux.HandleFunc(middleware.WithCORS("POST /users", userHandler.HandleUserPostRequest, opts1))
```

This is the only source of truth for whether a documented endpoint is real. Never verify from memory or from what "sounds like" a REST API should have.

## Checks

### 1. Endpoint Existence (CRITICAL — hard gate)

For every `paths.<path>.<method>` entry in the spec, confirm a matching route is registered:

```bash
grep -rn "HandleFunc.*\"<METHOD> " backend/internal/ | grep -i "<path-fragment>"
```

Path parameters need normalizing before comparing — OpenAPI writes `{userId}`, Go's `net/http` router (1.22+) may write `{userId}` too, or a wildcard like `{path...}` for prefix matches (seen in `GET /users/tree/{path...}`). Don't string-match blindly; read the surrounding handler registration to confirm the same logical route, and read the handler function itself if the match is ambiguous.

A documented path+method with no matching registration anywhere in `backend/internal/` → CRITICAL. The doc describes an endpoint that doesn't exist.

### 2. Reverse Coverage — Undocumented Public Endpoints (HIGH)

For the spec's domain, list every registered route in the corresponding `init.go` and cross-check against the spec's `paths`. Flag routes that look like public API surface (not an `OPTIONS` preflight handler, not an obviously internal/health-check route) with no spec entry. Judgment call, not mechanical — an intentionally-undocumented internal route isn't a finding.

### 3. Request/Response Schema Accuracy (CRITICAL for field names/types; verify against the handler's Go struct)

For each operation's request body and response schema:
- Read the handler function (or the struct it decodes into/encodes from) in the backend source.
- Confirm field names match the struct's JSON tags, required fields match, and types are compatible (a Go `int` documented as `string`, or vice versa, is CRITICAL).

If the handler's request/response struct isn't readable from the referenced function (e.g. it's assembled dynamically, or the trail goes cold), mark UNVERIFIED with the specific reason — don't assume the schema is correct because it looks plausible.

### 4. No Empty or Placeholder Descriptions (hard gate — completeness)

Scalar renders `summary`, `description`, and tag `description` directly with no separate prose layer to fall back on. FAIL on:
- Missing or empty `summary` on any operation.
- Missing or empty `description` on any tag.
- Placeholder text: "TODO", "TBD", "Description here", or a description that's just the endpoint path restated.

### 5. Valid YAML and Schema References (mechanical — hard gate)

- The file must parse as valid YAML.
- Every `$ref` must resolve to a component that actually exists in `components.schemas`/`components.parameters`/etc. — either in the same file or correctly referencing another `api/*.yaml` file. A dangling `$ref` breaks the merged spec at build time; treat it as CRITICAL, not a style nit.

### 6. Tag Declaration and Grouping

- Every tag referenced by an operation (`paths.*.*.tags`) must be declared in that file's own `tags:` list with a description (Check 4 already covers the description itself; this catches tags used but never declared, which lose their description entirely, not just have an empty one).
- If this is a new spec file, check `docs/api-groups.config.yaml`: per its own comments, an unlisted file auto-groups by filename in Title Case — that's fine as the default, but flag it as a note if the auto-derived name would read oddly, since silence here just means "using the default," not "verified as intentional."

### 7. Cross-File Style Consistency (WARN, not gate)

Compare against sibling `api/*.yaml` files:
- `summary` phrasing style (imperative "List users" vs. a noun phrase) — flag a file that's the outlier against the rest of the surface.
- Error response coverage — if sibling operations across the surface consistently document `401`/`403`/`404` where applicable, flag an operation missing them without an evident reason (e.g., a public unauthenticated endpoint legitimately has no `401`).

## Part 1 Output Format

```
Reviewing REST API spec: api/user.yaml

ENDPOINT EXISTENCE
  ✅  GET /users — backend/internal/user/init.go:97
  ❌  DELETE /users/{id}/bulk — no matching route found in backend/internal/user/init.go or elsewhere in backend/internal/

REVERSE COVERAGE
  ⚠️  GET /users/{id}/audit-log registered in init.go:203 but not documented in this spec

SCHEMA ACCURACY
  ❌  UserListResponse.totalResults documented as string; handler backend/internal/user/handler.go:88 encodes it as int

DESCRIPTIONS
  ❌  operation POST /users/{id}/roles — missing summary
  ✅  all tags have non-empty descriptions

YAML / REFS
  ✅  valid YAML, no dangling $ref

TAG GROUPING
  ✅  all tags declared

STYLE CONSISTENCY
  ⚠️  "users-by-path" tag description reads as a fragment; sibling tag descriptions are full sentences

─────────────────────────────────────
Result: FAIL
Hard gates: 2 (endpoint existence, schema accuracy)
Issues: 4 failures · 2 warnings
```

---

# Part 2: SDK API Reference Pages (`docs/content/sdks/<sdk>/apis/**/*.mdx`)

## Ground Truth, and Its Limits

**SDK source is no longer in this repo** (moved out in a prior restructuring). The `sdks/<name>/` directories that may exist in your working tree are untracked (`git ls-files sdks` returns nothing) — don't assume they're present; check before relying on them:

```bash
ls sdks/<sdk>/dist 2>/dev/null
```

- **If `dist/` exists**: it's a legitimate verification source, not just a type signature. It typically includes `.d.ts` declaration files (names, parameter types, return types) *and* a readable bundled `.js` (e.g. `dist/index.js`) that contains actual runtime defaults and logic — confirmed by cross-checking `CookieConfig`'s `DEFAULT_MAX_AGE = 3600` etc. directly against `cookie-options.mdx`'s documented defaults. Use both: `.d.ts` for signatures, the bundle for behavioral claims (defaults, conditionals, what a method actually does).
- **If absent**: mark the claim UNVERIFIED with the reason "SDK source and build artifacts not present in this working directory; the SDK lives in an external repo." Per the same convention as `tech.md`'s Hard Gate Rule 3, a *stated* reason doesn't itself hard-fail the review, but a human must still resolve it before merge — don't let an unstated assumption of correctness slip through instead.

This corrects `tech.md`'s existing §7 ("SDK Method Claims"), which still says to search `sdks/javascript/` for the method definition as if full source lives there — it doesn't anymore. Use the check above instead of that instruction whenever it's invoked for these pages.

## Checks

### 1. Signature-to-Table Consistency (CRITICAL — hard gate, fully mechanical)

When a page shows an explicit `interface`/type code block (class-style pages like `CookieOptions` do; component pages like `SignInButton` don't — they use the Props table as the only signature), every field in that block must appear as a row in the corresponding table, and every table row must appear in the block. This needs no external source — it's the page contradicting itself. A property in the interface but missing from the table (or vice versa) is CRITICAL.

### 2. Accuracy Against Build Artifacts (CRITICAL if verifiable, UNVERIFIED with reason if not)

Per "Ground Truth" above: when `dist/` is available, verify every claimed signature, default value, and stated behavior against it. Flag any mismatch as CRITICAL — a wrong default (e.g., documented `secure` default `false` when the bundle sets `true`) causes real misconfiguration for anyone who reads the doc and doesn't override it.

### 3. Error Claims Against the Error Registry (CRITICAL)

If a page's "Error Handling"/"Throws" section names a specific error class or code, confirm it exists in that SDK's `errors.mdx` (e.g. `javascript/apis/errors.mdx`'s "Error Hierarchy" section, or the equivalent for other SDKs). A named error class not in the hierarchy is either a typo or a fabricated claim — CRITICAL either way. If the SDK has no `errors.mdx`, mark UNVERIFIED and say so — don't assume the error name is right because it sounds plausible.

### 4. Cross-SDK Consistency for Shared Concepts (HIGH, judgment)

When multiple SDKs expose the same underlying concept (e.g. cookie options across the server-side JS SDKs), their documented defaults and behavior should agree unless the page states a reason for divergence. Flag disagreements — they usually mean one page is stale, not that the SDKs genuinely differ.

### 5. Coverage — Every Exported Member Has a Page (HIGH, needs `dist/`)

When `dist/` is available for the SDK, list its exported classes/functions/components (from `dist/index.d.ts` or equivalent) and confirm each has a corresponding page under that SDK's `apis/`. An exported member with no page is undocumented API surface — flag it. Without `dist/`, skip this check and say why (nothing to enumerate against).

### 6. Required Sections by Artifact Type (WARN — judgment, not a hard gate)

The page's own content tells you its type:
- **Client/class** (shows a `class`/constructor): needs a signature block, a method or property table, and an Error Handling section if any method can throw.
- **Component** (React/Vue, shows JSX/template usage): needs Usage, a Props table, and an Error Handling section if it can throw (as `SignInButton` does for `ThunderIDRuntimeError`).
- **Utility/middleware function**: needs a signature, a Parameters table, and a Returns description.

Missing the section its own type implies is worth flagging, but this is judgment about what the page's content implies it is — not a mechanical rule.

### 7. Cross-Reference Validity

Relative links between sibling pages in the same `apis/` tree follow the same rule as `check.md`'s Internal Links check — verify with `make build_docs`, don't hand-count `../` depth.

### 8. Table Column Consistency (WARN, style parity)

Flag a page whose parameter/property table omits a column (e.g. `Default`, `Required`) that a sibling page in the same SDK provides for an equally-applicable field, with no evident reason. This is about matching the SDK's own established convention, not imposing one column set on everything — a component's Props table using `Required` and a class's Properties table using `Default` can both be correct if that's each SDK's consistent pattern.

## Part 2 Output Format

```
Reviewing SDK reference: docs/content/sdks/node/apis/utilities/cookie-options.mdx
dist/ available: yes (sdks/node/dist)

SIGNATURE CONSISTENCY
  ✅  all 4 interface fields (httpOnly, maxAge, sameSite, secure) appear in the Properties table

BUILD ACCURACY
  ✅  maxAge default 3600 — matches sdks/node/dist/index.js:216
  ✅  httpOnly default true — matches sdks/node/dist/index.js:217
  ✅  sameSite default 'lax' — matches sdks/node/dist/index.js:218
  ✅  secure default true — matches sdks/node/dist/index.js:219

ERROR REGISTRY
  N/A — page has no Error Handling section, and none of its fields imply one

CROSS-SDK CONSISTENCY
  ✅  matches express SDK's cookie option defaults

COVERAGE
  ✅  all sdks/node/dist exports have a corresponding apis/ page

REQUIRED SECTIONS
  ✅  utility-function page has Signature, Parameters, Returns

CROSS-REFERENCES
  ✅  verified via make build_docs

TABLE COLUMNS
  ✅  consistent with sibling pages in sdks/node/apis/

─────────────────────────────────────
Result: PASS
Hard gates: 0
Issues: 0 failures · 0 warnings
```

---

## Hard Gate Summary

The review **FAILS** if any of:

**Part 1 (REST specs):**
1. Any documented endpoint has no matching backend route (Check 1).
2. Any request/response schema field name or type contradicts the backend struct (Check 3).
3. Any operation is missing a `summary`, or any tag is missing a `description` (Check 4).
4. The file fails to parse as YAML, or contains a dangling `$ref` (Check 5).

**Part 2 (SDK pages):**
1. A page's signature block and its table disagree (Check 1).
2. A claimed signature, default, or behavior contradicts `dist/` when `dist/` is available (Check 2).
3. A named error class/code doesn't exist in that SDK's error registry (Check 3).

**Both parts**: any claim marked UNVERIFIED *without* a stated reason — same convention as `tech.md`'s Hard Gate Rule 3.

---

## Scope Boundary

| Check | This reference | Other reference |
|---|---|---|
| Endpoint/schema accuracy against backend source | ✅ | (`tech.md` §4 covers this for API claims made *outside* dedicated reference pages, e.g. a guide mentioning an endpoint) |
| SDK signature/behavior accuracy against build artifacts | ✅ | (`tech.md` §7 covers this for SDK claims made outside dedicated reference pages, but see the correction above) |
| API reference page structure and coverage | ✅ | |
| Generic frontmatter, headings, code block tags, internal links | | `check.md` |
| Writing quality, tone, voice | | `style.md` |
| SEO / discoverability | | `seo.md` |
