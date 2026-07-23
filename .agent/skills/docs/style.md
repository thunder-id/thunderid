# ThunderID Docs Writing Quality Review

Review an existing `.mdx` file for writing quality, linguistic consistency, and AI-writing-pattern detection. Report every issue with the exact quote and a suggested rewrite; never fix silently. Structure, frontmatter, and links are `check.md`'s job; technical accuracy is `tech.md`'s; new content is `edit.md`'s.

## Usage

Read when the user asks to review, polish, improve, or fix the writing or tone of a doc, or make it sound less AI-generated. If no path is given, ask which file.

---

## Step 1: Identify the Doc Type

Read the full file. Read the `docType` frontmatter field — every page has one (`quickstart`, `guide`, `concept`, `reference`, `use-case`, or `community`) and `check.md` gates on it being present and valid, so this is the source of truth, not a guess from structure.

- **quickstart** — step-by-step guide connecting a technology to ThunderID; `<Stepper>` with imperative H2 steps
- **guide** — task-oriented how-to, no Stepper
- **concept** — explains an idea/pattern, no step instructions
- **reference** — factual, tables/lists over prose
- **use-case** — scenario overview
- **community** — project-contribution docs, not end-user product docs; apply the same universal writing checks, skip the per-type checks in Step 4

State the type before running checks. If `docType` is missing (an older page not yet migrated) or the content clearly doesn't match its stated type, flag it and fall back to inferring from structure.

---

## Step 2: Calibrate Against the Established Voice

Rule-compliant prose can still fail to sound like the rest of the docs. Before the itemized checks, read 2-3 sibling pages of the same type (same section/category preferred) and compare:

- **Sentence rhythm** — short and declarative, or a mix with longer explanatory sentences? Does the target match?
- **Intro/transition framing** — how do siblings open a section or move between steps, vs. the target?
- **Formality/directness** — same directness and lack of filler, or more casual, hedging, or clipped?
- **Structural habits** — similar density of examples, admonitions, tables?

Flag deviations concretely: quote the target next to a comparable sibling phrase and name the difference. "The tone feels off" is not a finding.

If no sibling pages exist yet, skip this check and say so.

---

## Step 3: Universal Checks (All Doc Types)

### Active Voice
Instructions must use active voice.
- ❌ "The configuration can be set by navigating to..." → ✅ "Navigate to... and set the configuration."

### Address the Reader as "You"
Use second person; never label the audience.
- ❌ "Developers can use this flow to..." → ✅ "Use this flow to..."

### No Condescension Markers
Never imply a task is easy: cut "Simply," "Just," "All you need to do is," "Easily," "Straightforward."

### No Hedging in Instructions
Be direct: "You may want to consider running..." → "Run...". "You can run..." → "Run..." unless the step is genuinely optional.

### One Action Per Sentence
Don't chain actions with "and" in steps.
- ❌ "Click **New Application** and fill in the name and select the type." → ✅ "Click **New Application**. Enter a name, then select an application type."

### AI Vocabulary — Cut or Replace
Flag in prose (outside code blocks):

**Always cut/replace:** additionally, align with, crucial, delve, emphasizing, enduring, enhance, foster, garner, highlight (verb), interplay, intricate/intricacies, key (filler), landscape (abstract), leveraging, navigate (abstract), pivotal, robust, showcase, tapestry (abstract), testament, underscore (verb), valuable, vibrant

**Cut in technical docs:** game-changer, cutting-edge, seamlessly, empower, powerful (filler), comprehensive (filler), streamline (abstract), next-generation, out-of-the-box (filler), intuitive (filler claim), scalable (state the scale/how instead), enable/allows you to (use the imperative), end-to-end (filler), holistic, synergy, paradigm, unlock (abstract)

For each instance: quote → suggest replacement or "cut". Complements the Vale style in CI (which catches most automatically); this confirms no context-dependent uses remain (e.g. "navigate" as abstract verb vs. UI instruction).

**Hard gate:** 5+ instances (counting occurrences, not unique words) → hard gate failure.

### Filler Phrases — Cut
"In order to" → "To" · "Due to the fact that" → "Because" · "At this point in time" → "Now"/cut · "Has the ability to" → "Can" · "It is important to note that" → delete, state the point · "Please note that" → cut · "Note that" as opener → restructure · "Essentially"/"Basically" → cut · "Ultimately" as empty closer → cut

### Superficial -ing Tails
Cut present-participle tails tacked on to fake depth.
- ❌ "...allowing you to manage users efficiently." → cut the tail; if it matters, make it its own sentence with real content.

### Over-Explaining the Obvious
Don't describe what's visible or state self-evident consequences.
- ❌ "Click the **Save** button to save your changes." → ✅ "Click **Save**."

### No Em Dashes or En Dashes
Em dashes (—) and en dashes (–) must not appear anywhere. Search: `grep -n '[—–]' <file>`. Pick the narrowest fix, usually a period or comma; reach for a colon only when it clearly fits (Colon Rules below).

| Usage | Replace with |
|---|---|
| Two complete sentences joined for effect | Period, split into two |
| Parenthetical aside | Comma, or remove the aside |
| Sentence introducing a list/explanation/result | Colon (see Colon Rules) |
| Contrast | ", but Y" / ", while Y" |
| Elaboration that isn't a full sentence | Comma ("X, including Y") |
| Range in prose | Spell out: "X to Y" |

**Hard gate:** any em/en dash remaining → hard gate failure.

### Comma Rules
Use commas to make structure unambiguous, not to imitate speech pauses.

1. **Lists**: comma before the final and/or (serial comma). ❌ "password, passkey and social login" → ✅ "...passkey, and social login."
2. **Introductory phrases**: comma after ("After you restart the server, verify..."). Short phrases don't strictly need one; stay consistent.
3. **Two complete sentences joined by a coordinating conjunction** (and/but/or/nor/for/so/yet): comma before it, only when both sides are complete. ✅ "...successfully, but the application cannot connect." No comma when the second half isn't a full sentence.
4. **Nonessential info** gets commas; essential (identifying) info doesn't.
5. **Coordinate adjectives** (test: insert "and"): ✅ "a valid, absolute URL." Not when they form one unit: "a public API endpoint."
6. **Direct address** takes a comma (rare here, since instructions address the reader implicitly).
7. **Transitional words** (however, therefore) take a comma after them at sentence start.

Flag: comma splices ("expired, request a new token" → period or "so"); a comma between subject and verb; a comma before "and" joining one subject with two actions; comma overload (split into sentences instead).

### Colon Rules
Use a colon to introduce an explanation, list, or result, only when the text before it could stand alone as a sentence (UI labels and compact reference lines are the exception).

1. List after a complete sentence: "Configure the following: hostname, port, public URL." Not "Configure: hostname..."
2. Explanation/result: "...failed for one reason: the token had expired."
3. Examples: prefer "such as" or "For example," over "for example:".
4. Introducing code, commands, or output.
5. Headings/labels ("Default value: localhost"); no trailing colon on a heading unless it's a label.
6. Never split a verb/preposition from its object: ❌ "are: password, passkey" → ✅ "are password, passkey, and social login."
7. Lowercase after a colon unless a proper noun, a quoted sentence, code, a UI label, or multiple complete sentences follow (prefer a bulleted list instead).
8. No filler lead-in before a list: if the sentence before the list already stands alone, colon it directly instead of inventing a transition sentence. ❌ "...so you need to allow your app's origin. To do so, do the following:" (redundant, repeats "do" twice) → ✅ "...so you need to allow your app's origin:"

Test: colon when the first part creates an expectation the second fulfills, not just because a list appears somewhere in the sentence.

### Language: US English
"organise"→organize, "colour"→color, "licence" (noun)→license, "cancelled"→canceled, "centre"→center.

### No Contractions
Flag all: "you'll"→you will, "it's"→it is, "don't"→do not, "can't"→cannot.

### Numbers
Spell out one-nine in prose; numerals for 10+. Always numerals for ports, versions, time values, and counts regardless of size ("port 8090", "3 redirect URIs"). Use `%` directly. Numerals + unit for measurements (`512 MB`, `30 seconds`).

### UI Element Formatting
**Bold** for UI labels, buttons, menu items, fields, exactly as shown. `Inline code` for file paths, CLI commands, config keys/values, identifiers, env vars, port numbers. No quotes around UI elements or code values.

### No Emojis
None in prose, headings, table cells, or as decorative icons (including tech-type icons like ⚛️🌐📱🤖 in tables — use the bold term alone). Exception: admonition/status characters that are the site's own tooling output (✅/❌ in a review report), or product branding metadata outside doc content.

### Admonition Usage
By type, not for variety:

| Admonition | When |
|---|---|
| `:::note` | Supplementary, non-blocking |
| `:::tip` | Shortcut or best practice |
| `:::warning` | Risk of data loss/misconfiguration |
| `:::danger` | Destructive/irreversible |
| `:::info` | Background/conceptual framing (use-case pages only) |

Flag any admonition restating the preceding paragraph, or where inline prose would be clearer.

### Diagrams Must Be Mermaid
Any diagram must use a fenced ` ```mermaid ` block. Flag raw SVG, ASCII/box-drawing art, or a screenshot standing in for a diagram with no source of truth. Exception only when the layout genuinely can't be Mermaid (e.g. pixel-level annotation over a real screenshot) — state why. Don't add `%%{init...}%%` or inline style/classDef overrides; the site applies brand colors globally (`docusaurus.config.ts` → `themeConfig.mermaid`) and per-diagram overrides drift and can clash between light/dark mode.

### No Informal Abbreviations
| Use | Never |
|-----|-----------|
| configuration(s) | configs |
| development | dev |
| production | prod |
| environment(s) | env, envs |
| repository/repositories | repo, repos |

Prose only — leave file paths, config file names, branch names, and code identifiers as-is (`.env`, `docker-compose.prod.yml`, the `dev` branch, etc.).

### Consistent Terminology
| Use | Never |
|-----|-----------|
| sign in | log in, login (verb) |
| sign out | log out, logout (verb) |
| sign-in (adj/noun) | login, log-in |
| application | app (in prose) |
| configure | setup (verb), set-up |
| navigate to | go to, head to |
| select | choose, pick (UI dropdowns/options) |
| click | press, tap (desktop UI) |
| run | execute, invoke (CLI) |
| create | add, make (Console resources) |
| delete | remove, destroy (Console resources) |
| redirect URI | callback URL, redirect URL |
| access token | auth token (unless a specific non-JWT token) |
| identity provider | IdP (spell out first use per page) |

### Inclusive Language
| Avoid | Use |
|---|---|
| whitelist/blacklist | allowlist/denylist |
| master/slave | primary/replica |
| sanity check | quick check, confidence check |
| dummy value | placeholder/example value |

Flag "he/him" for hypothetical users → they/them. Flag ableist metaphors ("crazy configuration") → describe the actual problem.

### Sentence Cadence
**Choppy**: in running prose (skip code/lists/headings/admonitions/table cells), flag 3+ consecutive sentences of 8 words or fewer. Fine inside numbered steps or `<Stepper>` — only flag running prose.

**Long**: flag any prose sentence over 30 words, or any paragraph over 5 sentences — split or listify.

### AI Structural Patterns
Survive word-level scans; check across the full page.

- **Sentence length uniformity**: flag 4+ consecutive sentences in the same 8-word band. Prose should vary.
- **Paragraph length uniformity**: flag 3+ consecutive paragraphs of identical length.
- **Template repetition**: flag 3+ sections following the identical structural pattern (e.g. definition → explanation → example every H2). 1-2 is fine.
- **Generic connector abuse** as paragraph starters: "Furthermore," "Moreover," "Additionally," "In addition to this," "It is also worth noting that," "This highlights the importance of." Cut and reference the previous paragraph's specific content instead.

### Rhetorical Scaffolding
The subtlest tell, and the most common reason technically correct docs still read as hollow.

**Templated pivots** — flag every occurrence: "Here's where it gets interesting," "Here's where things get [adj]," "But here's the thing," "At its core," "This is where it all comes together," "The bottom line:," "That's not the whole story," "This sounds like a minor distinction until...," "This is the core tension." Replace with the specific concept, step, or behavior under discussion.

**Symmetric contrast framing** — flag every "It's not X. It's Y." construction ("The question isn't whether X. It's whether Y."). One per page is fine; 2+ is a hard gate. Fix: break the symmetry, concede the first clause, or collapse into one sentence with uneven clause lengths.

**Exhaustive enumeration on repeated mention**: flag a list (prerequisites, steps, options) spelled out in full more than once on the same page — abbreviate on second mention ("the same three flags as above").

**Triplet paragraph rhythm**: flag 3+ consecutive paragraphs following [Statement]. [Qualifier]. [Why it matters]. — mix up where the conclusion lands.

**Hard gate:** any rhetorical scaffolding phrase, or 2+ symmetric contrast constructions.

### Interchangeability Test
For each section's first paragraph: could it appear unchanged in a different product's docs? If so it's generic scaffolding — it needs at least one ThunderID-specific element (a named concept, specific behavior, concrete step, or technical constraint). Flag failures and suggest what would make it specific.

### Promotional Tone
Docs are neutral and precise, not marketing copy. Flag: unsupported superiority claims ("simplifies your workflow"), adjective stacking ("flexible, powerful, enterprise-ready"), technical facts framed as selling points, "powerful"/"rich"/"best-in-class" with no specificity, "allows you to" instead of just describing the behavior.
- ❌ "ThunderID's flexible flow engine lets you build any authentication experience." → ✅ "The flow designer supports conditional branching, multi-factor steps, and external service calls within a single sign-in flow."

### Generic Writing
**Specificity test** (concept pages exempt): each section needs at least one of: a ThunderID-specific behavior/constraint, a concrete example with real values/steps/outcome, or a trade-off/limitation/condition. Flag failures with what detail would fix it.

**Section necessity test**: could the reader skip this H2 and still follow the page? Filler sections restate the intro, give generic background, or could belong to any product's docs unchanged. Flag and say whether to cut, merge, or replace with ThunderID-specific content.

### Instruction Precision
For quickstart/guide steps:

**Vague outcomes**: does the step say what to expect when the result is non-obvious? ❌ "Configure the OAuth settings." → ✅ "Set **Redirect URI** to `http://localhost:3000/callback`. <ProductName /> rejects requests that redirect elsewhere."

**Ambiguous UI references**: ❌ "Open the settings." (which settings?) → ✅ "In the Console, navigate to **Applications** → [App name] → **Protocol**."

### Step Count and Decomposition
Count steps in a single Stepper/numbered sequence.

Not a hard gate. 10+ steps:
1. Propose a specific restructuring: split into multiple pages (genuinely separate tasks), group into labeled phases (one continuous task with natural internal structure), or fold trivial steps ("click Save") into a neighbor.
2. Ask the user directly whether the flat structure or the restructuring is better (batch with Step Locality below if both apply).
3. If confirmed intentional: mark ✅ passed, tell them the exact marker to add (`steps={N}`) — write it via `edit.md` or a manual edit. If they want to restructure: mark `[needs writer input]`.

"Too many steps, shorten it" alone is not a finding — the count only triggers the conversation.

### Step Locality (Minimize Screen Switching)
Identify each step's UI location (tab/page/screen named or clearly implied).

Not a hard gate. 2+ steps at the same location are separated by an unrelated-location step with no dependency either way:
- ❌ Settings → Applications → back to Settings → ✅ Applications first, then both Settings actions merged.

Before flagging, check for a genuine dependency (an intervening step produces something the return step needs) — if so, skip the flag. When it applies: propose the reordered/regrouped sequence, ask the user directly (batch with Step Count if both fire). If confirmed intentional: mark ✅. Otherwise `[needs writer input]`.

### Batching Step Count and Step Locality Questions
If both need to ask on the same page, combine into one message (matching how `new-page.md` batches its questions):

> This page has two structural things worth confirming:
> 1. **Step count** — {N} steps. {proposal or "no restructuring needed"}
> 2. **Step order** — {screens}. {proposal or "no reorder needed"}
> Are the current structure and order intentional, or would you like either restructured?

Process both answers before finalizing; each resolves independently.

### Concept Bleed
Beyond the per-type rules in Step 4, flag a **guide** embedding a full parameter reference table instead of linking to the reference page. Name the correct doc type and suggest cut/summarize/link out.

---

## Hard Gate Rules
These four determine pass/fail; every other ❌ finding should still be fixed before merge but doesn't gate:

1. Any em/en dash present.
2. 5+ AI vocabulary instances remain.
3. Any rhetorical scaffolding phrase present.
4. 2+ symmetric contrast constructions.

Step Count/Decomposition and Step Locality are deliberately excluded — they resolve by asking the user, not by mechanical count/pattern match.

---

## Step 4: Doc-Type-Specific Checks

Apply after universal checks; these override Step 2/3 where they conflict. For reference pages, "no second-person" supersedes the universal "address the reader as you."

### Quickstart
**Intro**: must open "*Use this guide to [verb] [outcome].*" ❌ "This guide explains how to connect React..." → ✅ "Use this guide to add sign-in to a React app using <ProductName />."

**Step headings**: imperative verb phrases, not label nouns. ❌ `## Application Configuration` → ✅ `## Configure the Application`.

**No conceptual detours mid-step**: link out for background ("See [Flows](link)"); flag inline explanations longer than one sentence.

**Page ending**: must be `## What's Next`.

### Guide
**Intro**: leads with the task/outcome, not a page description. ❌ "This guide is about configuring X." / "This guide walks you through X." → ✅ "Configure X to enable Y in your application."

**No "This page/guide/document" openers in body** (`check.md` only checks this in frontmatter `description`).

**Page ending**: `## Next Steps` or `## Related Guides`. Flag `## Go Further`, `## What's Next?`, or others.

### Concept
**No imperative instructions** ("Click X", "Run Y") — link to a guide instead.

**Opening defines the concept** ("What is X?"); not a task, list, or historical claim.

**No "How to" in title/headings** — misplaced content, move to a guide or reframe.

**Page ending**: must be `## Related`.

### Reference
**No narrative prose** where a table fits better.

**No second-person**: imperative or third-person for parameter descriptions. ❌ "You can set this to `true`..." → ✅ "Set to `true` to enable caching."

**No "you need to"/"you should."**

**No page ending section required.**

### Use-Case
**Intro** frames scenario and audience, not a feature list.

**"When to Choose This Pattern"**: bullet criteria, not prose.

**Page ending**: `## Try It Out` or `## Next Steps`.

---

## Output Format
Group findings by check category; omit categories with no issues. Per finding: quote the text (with line number), state what's wrong in one line, suggest a rewrite or mark `[needs writer input]`.

```
Reviewing: docs/content/guides/applications/manage-applications.mdx
Doc type: guide
Sibling pages read: manage-users.mdx, manage-roles.mdx

CROSS-PAGE CONSISTENCY
  ❌  Steps open with a bare imperative ("Click Save."); siblings lead with a one-line rationale
      first (e.g., "To scope this role to a team, select an organization unit first.").
      → Add a short rationale before non-obvious steps, matching sibling pattern.

ACTIVE VOICE
  ❌  line 23: "The application can be deleted by navigating to Settings."
      → "Navigate to Settings and delete the application."

AI VOCABULARY
  ❌  line 41: "leverage the SDK" → "use the SDK"
  ❌  line 67: "streamline your workflow" → cut or state what it specifically does

FILLER PHRASES
  ❌  line 12: "In order to register" → "To register"

CONDESCENSION
  ❌  line 88: "Simply click Save" → "Click Save"

GUIDE: INTRO SENTENCE
  ❌  "This guide is about managing applications in the Console."
      → "This guide walks you through creating, configuring, and deleting applications in the <ProductName /> Console."

GUIDE: PAGE ENDING
  ✅  ## Next Steps — correct

─────────────────────────────────────
Result: FAIL
Hard gates: 0
Issues: 6 failures · 1 pass
```

`Result` is `FAIL` only when a Hard Gate Rule triggers; otherwise `PASS` even with non-gating ❌ issues remaining (still fix before merge, but judgment-based findings get human review, unlike hard gates). All-clean:
```
─────────────────────────────────────
Result: PASS
Hard gates: 0
Issues: 0 failures · N pass
```
