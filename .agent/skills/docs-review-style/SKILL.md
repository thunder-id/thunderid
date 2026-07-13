---
name: docs-review-style
description: Reviews existing ThunderID documentation MDX content for writing quality, linguistic consistency, and AI-writing-pattern detection — flags AI vocabulary, passive voice, weak phrasing, condescension markers, em dashes, contractions, hand-built diagrams, rhetorical scaffolding, promotional tone, generic/interchangeable content, type-specific pattern violations (intro sentences, page-ending labels), oversized step sequences, and step ordering that bounces between UI screens with no dependency forcing it — the last two are proposed as a restructuring and confirmed with the user rather than auto-failed. Also calibrates the page against sibling pages to catch voice drift that's technically rule-compliant but still reads like a different author wrote it. Reports every issue with the exact quote and a suggested rewrite; never fixes silently. Use when polishing a draft, reviewing AI-generated content, improving a page before merging, or specifically chasing down why the docs don't read as one consistent voice. For writing new content into placeholders, use `/docs-edit` instead.
allowed-tools: Read Bash
---

# ThunderID Docs Writing Quality Review

Review an existing `.mdx` file for writing quality, linguistic consistency, and AI-writing-pattern detection. Report every issue with the exact quote and a suggested rewrite. Never fix silently — explain every change. Do not touch structure, frontmatter completeness, or link format — those are for `/docs-check`. Do not check technical accuracy — that is `/docs-review-tech`. For writing new content into placeholders, use `/docs-edit` instead.

This skill enforces all ThunderID writing quality and style rules, including detection of AI-sounding structural patterns: writing that is technically accurate but still generic, over-explained, promotional, or structurally hollow.

## Usage

Invoked as `/docs-review-style [file-path]`

If no path is given, ask which file to review.

If the path is a `SKILL.md` under `.agent/skills/` or `.claude/skills/`, stop and say this skill only reviews published documentation content — a skill file is agent instructions, not a documentation page, and these style rules do not apply to it.

---

## Step 1: Identify the Doc Type

Read the full file. Identify the doc type from frontmatter or structure:

- **quickstart** — has `toc_progress: quickstart` or `<Stepper>` with imperative H2 steps
- **guide** — task-oriented how-to page, no Stepper
- **concept** — explains a concept or pattern; no step instructions
- **reference** — factual, structured; tables and lists over prose
- **use-case** — scenario overview page

State the detected doc type before running any checks. If ambiguous, ask.

---

## Step 2: Calibrate Against the Established Voice

A page can pass every itemized rule below and still not sound like it belongs in the same body of work as the rest of the docs. Before running the rule-based checks, read 2–3 sibling pages of the same doc type, preferring the same section or category as the target file. Use them to notice what "sounding consistent" actually means beyond what the itemized rules cover:

- **Sentence rhythm**: are sibling pages' sentences short and declarative, or do they mix in longer explanatory sentences? Does the target page's rhythm match, or does it read noticeably choppier or more meandering?
- **Framing of intros and transitions**: how do sibling pages open a section, introduce an example, or move from one step to the next? Does the target page follow a similar pattern, or does it introduce its own idiosyncratic phrasing?
- **Level of formality and directness**: do sibling pages address the reader with the same degree of directness and the same absence of filler, or does the target page feel noticeably more casual, more hedging, or more clipped in comparison?
- **Structural habits**: do sibling pages use examples, admonitions, or tables at a similar density? A page that's unusually sparse or unusually example-heavy compared to its siblings reads as written by a different author, even when every individual sentence is rule-compliant.

Flag deviations concretely: quote the target page's phrasing next to a comparable phrase from a sibling page, and name the specific difference. "The tone feels off" is not a finding — a side-by-side quote and what's different about it is.

If no sibling pages exist yet (a genuinely new section with no precedent), skip this check and say so.

---

## Step 3: Universal Checks (All Doc Types)

Apply these regardless of doc type.

---

### Active Voice

Instructions must use active voice.

- ❌ "The configuration can be set by navigating to..."
- ✅ "Navigate to... and set the configuration."
- ❌ "An application will be created."
- ✅ "<ProductName /> creates an application."

Flag every passive voice instance in instructions. Suggest the active rewrite.

---

### Address the Reader as "You"

Use second-person "you" in instructions and guidance. Never label the audience in the text.

- ❌ "Developers can use this flow to..."
- ✅ "Use this flow to..."
- ❌ "For platform engineers evaluating..."
- ✅ (Write to them directly, not about them.)

---

### No Condescension Markers

Never imply a task is easy. Cut or rephrase:

- "Simply run..." → "Run..."
- "Just add..." → "Add..."
- "All you need to do is..." → state the instruction directly
- "Easily configure..." → "Configure..."
- "Straightforward setup" → remove the qualifier

---

### No Hedging in Instructions

Instructions must be direct.

- ❌ "You may want to consider running..."  → "Run..."
- ❌ "It might be worth checking..." → "Check..."
- ❌ "You will need to..." (future tense) → "You need to..." or just the imperative
- ❌ "You can run..." (when it's a required step) → "Run..."

"You can" is fine only when something is genuinely optional.

---

### One Action Per Sentence

In steps and instructions, each sentence performs one action. Do not chain actions with "and."

- ❌ "Click **New Application** and fill in the name and select the type."
- ✅ "Click **New Application**. Enter a name, then select an application type."

---

### AI Vocabulary — Cut or Replace

Flag any of the following in prose (outside code blocks):

**Always cut or replace:** additionally, align with, crucial, delve, emphasizing, enduring, enhance, foster, garner, highlight (verb), interplay, intricate/intricacies, key (filler adjective), landscape (abstract), leveraging, navigate (abstract), pivotal, robust, showcase, tapestry (abstract), testament, underscore (verb), valuable, vibrant

**Cut in technical docs:** game-changer, cutting-edge, seamlessly, empower, powerful (filler), comprehensive (filler), streamline (abstract), next-generation, out-of-the-box (filler), intuitive (as a filler claim), scalable (state the specific scale and how), enable/allows you to (use the imperative or describe the capability directly), end-to-end (filler), holistic, synergy, paradigm, unlock (abstract — "unlock potential")

For each instance: quote the original → suggest the replacement or "cut". This complements the ThunderID Vale style in CI, which catches most of these words automatically — this check confirms no context-dependent uses remain (e.g., "navigate" as an abstract verb vs. a UI navigation instruction) before merge.

**Hard gate:** if 5 or more instances remain (counting each occurrence separately, not unique words), treat this as a hard gate failure — see Hard Gate Rules below.

---

### Filler Phrases — Cut

- "In order to" → "To"
- "Due to the fact that" → "Because"
- "At this point in time" → "Now" or cut
- "Has the ability to" → "Can"
- "It is important to note that" → delete, then state the point
- "Please note that" → cut
- "Note that" as a sentence opener → restructure or use an admonition
- "Essentially" / "Basically" → cut
- "Ultimately" as an empty closer → cut

---

### Superficial -ing Tails

Flag present participle phrases tacked onto sentences to fake depth:

- ❌ "...allowing you to manage users efficiently."
- ✅ Cut the tail. If the point matters, make it its own sentence with real content.
- ❌ "...enabling seamless integration with your existing setup."
- ✅ State what the integration does specifically, or cut entirely.

---

### Over-Explaining the Obvious

Do not describe what the reader can see, or explain consequences that are self-evident.

- ❌ "Click the **Save** button to save your changes." → "Click **Save**."
- ❌ "This creates an application, which will now appear in the list." → "This creates the application."
- ❌ "Run the following command to run the server:" → "Run:"

---

### No Em Dashes or En Dashes

Em dashes (—) and en dashes (–) must not appear anywhere in the file. Rewrite the sentence instead.

Search with:
```bash
grep -n '[—–]' <file>
```

| Usage | Replace with |
|---|---|
| `X — Y` (parenthetical aside) | Two sentences, or a comma |
| `X — Y` (definition/explanation) | Colon |
| `X — Y` (contrast) | ", while Y" or ", but Y" |
| `X — Y` (elaboration) | ", including Y" or ": Y" |
| `X – Y` (range in prose) | Spell out: "X to Y" |

**Hard gate:** if any em dash or en dash remains, treat this as a hard gate failure — see Hard Gate Rules below.

---

### Language: US English

Flag non-US spellings. Common cases:
- "organise" → "organize"
- "colour" → "color"
- "licence" (noun) → "license"
- "cancelled" → "canceled"
- "centre" → "center"

---

### No Contractions

Documentation must be precise, not conversational. Flag all contractions.
- "you'll" → "you will" or rewrite as imperative
- "it's" → "it is"
- "don't" → "do not"
- "can't" → "cannot"

---

### Oxford Comma

Always use the Oxford comma in lists of three or more items.
- ❌ "Supports password, passkey and social login."
- ✅ "Supports password, passkey, and social login."

---

### Numbers

- Spell out one through nine in prose; use numerals for 10 and above.
- Always use numerals for: port numbers, version numbers, time values, and counts in technical context regardless of size.
  - ✅ "The server listens on port 8090."
  - ✅ "This creates 3 redirect URIs."
- Use `%` directly — never spell out "percent."
- Use numerals + unit for all measurements: `512 MB`, `30 seconds`.

---

### UI Element Formatting

**Bold** for UI labels, button names, menu items, and field names — exactly as they appear in the interface.
- ✅ "Click **Save**."  ❌ "Click the save button."  ❌ "Click 'Save'."

`Inline code` for: file paths, CLI commands, config keys and values, code identifiers, environment variables, port numbers in technical context.

No quotes around UI elements or code values. Use bold or code formatting instead.

---

### No Emojis

Do not use emojis anywhere in prose, headings, table cells, or as decorative icons in front of a term. This includes technology/type icons (⚛️, 🌐, 📱, 🤖) in tables — use the bold term alone, or a text label if a category needs distinguishing.

- ❌ "| ⚛️ **Browser App** | ..." → "| **Browser App** | ..."
- ❌ "✉️ Check your email" → "Check your email"
- ❌ "🎉 All checks passed!" → "All checks passed."

This does not apply to admonition/status characters that are part of the site's own tooling output (e.g., ✅/❌ in a `/docs-review-style` report) or to product branding metadata outside doc content (e.g., the site's emoji favicon in `docusaurus.product.config.ts`).

---

### Admonition Usage

Use Docusaurus admonitions by type, not for visual variety.

| Admonition | When to use |
|---|---|
| `:::note` | Supplementary information the reader should be aware of but is not blocking. |
| `:::tip` | A faster path, a shortcut, or a useful best practice. |
| `:::warning` | An action that could cause data loss, misconfiguration, or unexpected behavior. |
| `:::danger` | An action that is destructive or irreversible. |
| `:::info` | Background context or conceptual framing, used in use-case pages only. |

Flag any admonition that restates what the preceding paragraph already said, or where inline prose would be clearer.

---

### Diagrams Must Be Mermaid

Any flow diagram, architecture diagram, sequence diagram, or similar visual must use a fenced ` ```mermaid ` code block. Flag:

- Raw SVG elements (`<svg>`, `<rect>`, `<path>`, `<text>`, and similar) used to hand-build a diagram
- ASCII art or box-drawing characters (`┌`, `─`, `│`, `→`, and similar) used to represent a diagram
- A screenshot or image standing in for a diagram that has no source of truth

Only accept a non-Mermaid diagram when the layout genuinely cannot be expressed in Mermaid (e.g., precise pixel-level annotation over a real UI screenshot). State the reason when allowing an exception.

Do not add a `%%{init: {'theme': ...}}%%` directive or inline `style`/`classDef` color overrides to a Mermaid block. The site applies the brand color scheme globally (`docusaurus.config.ts` → `themeConfig.mermaid`); a per-diagram override will drift from the rest of the docs and may clash between light and dark mode.

---

### No Informal Abbreviations

Flag informal shorthand in prose and spell out the full word:

| Use | Never use |
|-----|-----------|
| configuration(s) | configs |
| development | dev |
| production | prod |
| environment(s) | env, envs |
| repository/repositories | repo, repos |

This applies to prose only — leave file paths, config file names, branch names, and code identifiers as-is (`.env`, `docker-compose.prod.yml`, the `dev` branch, `go.mod`'s `require` block, and similar are correct exactly as written).

---

### Consistent Terminology

Flag any use of the wrong term from this list:

| Use | Never use |
|-----|-----------|
| sign in | log in, login (verb) |
| sign out | log out, logout (verb) |
| sign-in (adjective/noun) | login, log-in |
| application | app (in prose) |
| configure | setup (verb), set-up |
| navigate to | go to, head to |
| select | choose, pick (for UI dropdowns/options) |
| click | press, tap (for desktop UI) |
| run | execute, invoke (for CLI commands) |
| create | add, make (for Console resources) |
| delete | remove, destroy (for Console resources) |
| redirect URI | callback URL, redirect URL |
| access token | auth token (unless specifically discussing a non-JWT token) |
| identity provider | IdP (spell out on first use per page; thereafter IdP is acceptable) |

---

### Inclusive Language

Flag any of the following:

| Avoid | Use |
|---|---|
| whitelist / blacklist | allowlist / denylist |
| master / slave | primary / replica |
| sanity check | quick check, confidence check |
| dummy value | placeholder value, example value |

Flag use of "he/him" for hypothetical or unnamed users → "they/them".

Flag ableist metaphors used for technical behavior (e.g., "crazy configuration") → describe the actual problem.

---

### Sentence Cadence

**Choppy cadence**: for each paragraph of running prose (skip code blocks, bullet lists, headings, admonition blocks, and table cells), flag any run of three or more consecutive sentences with eight or fewer words.

- ❌ "Configure the redirect URI. Save the application. Open the flow designer."
- ✅ "Configure the redirect URI and save the application, then open the flow designer to set up the sign-in steps."

This is acceptable in a numbered list of steps or inside `<Stepper>` steps — only flag it in running prose paragraphs.

**Long sentences and paragraphs**: flag any sentence in running prose that exceeds 30 words — if it requires re-reading to parse, rewrite as two shorter sentences. Flag any paragraph of running prose with more than five sentences — break it into smaller paragraphs or convert part of it to a list.

---

### AI Structural Patterns

These patterns survive word-level scans. Check each one across the full page.

**Sentence length uniformity**: flag any run of 4 or more consecutive sentences that fall within the same 8-word band (e.g., all 12–20 words). Docs prose should vary — a short sentence can make a technical point land hard; a longer sentence carries the context that explains it.

- ❌ "The SDK initializes on page load. It reads the stored token. It checks for expiry. It refreshes if needed."
- ✅ "On page load, the SDK initializes, reads the stored token, and checks for expiry. If the token has expired, it refreshes automatically before any API call is made."

**Paragraph length uniformity**: flag any run of 3 or more consecutive paragraphs that are the same length (all single sentences, or all exactly 3 sentences) — every paragraph weighing exactly the same reads as machine-authored.

**Template repetition**: flag any page where 3 or more sections follow the exact same structural pattern (e.g., definition → explanation → example, repeated in each H2). One or two sections sharing a structure is fine.

**Generic connector abuse**: flag these as paragraph starters — they create transitions that sound smooth but carry no meaning:
- "Furthermore,"
- "Moreover,"
- "Additionally,"
- "In addition to this,"
- "It is also worth noting that"
- "This highlights the importance of"

For each match: cut the connector and revise the sentence to reference the specific content of the previous paragraph instead.

---

### Rhetorical Scaffolding

The subtlest AI writing tells, and the most common reason a technically correct doc still reads as hollow.

**Templated section pivots** — flag every occurrence, they replace specific content with a generic transition:
- "Here's where it gets interesting"
- "Here's where things get [adjective]"
- "But here's the thing"
- "At its core"
- "This is where it all comes together"
- "The bottom line:"
- "That's not the whole story"
- "This sounds like a minor distinction until..."
- "This is the core tension"

For each match: replace the phrase with a sentence that names the specific concept, step, or product behavior under discussion. Scaffolding is universal; specificity is human.

**Symmetric contrast framing** — flag every "It's not X. It's Y." construction with artificially parallel clauses:
- "The question isn't whether X. It's whether Y."
- "It's not about what [X] does. It's about what [X] can't do."
- "The problem isn't [A]. The problem is [B]."

One such construction per page is acceptable. Two or more is a hard gate failure — see Hard Gate Rules below. For each duplicate: break the symmetry, concede the first clause, or collapse it into one sentence with uneven clause lengths.

**Exhaustive enumeration on repeated mention**: if a list of items (prerequisites, steps, config options) is spelled out in full more than once in the same page, flag the repeated instance. A human expert abbreviates on second mention ("the same three flags as above"); AI lists everything every time.

**Triplet paragraph rhythm**: flag sections where three or more consecutive paragraphs follow this three-beat pattern: [Statement]. [It does not do X / qualifier]. [That distinction matters / why it matters]. Mix it up — front-load the conclusion, bury the point mid-paragraph, or leave the reader to draw the inference.

**Hard gate:** if any rhetorical scaffolding phrase appears, or if more than one symmetric contrast construction appears, treat this as a hard gate failure — see Hard Gate Rules below.

---

### Interchangeability Test

For the first paragraph of each section (after the heading), ask: could this exact paragraph appear in the docs for a completely different product, unchanged? A section intro that passes this test is generic scaffolding — it must contain at least one element specific to ThunderID: a named concept (flows, applications, identity providers), a specific behavior, a concrete step, or a technical constraint.

Flag any section intro that fails this test. Suggest what specific element would make it non-interchangeable.

---

### Promotional Tone

Docs must be neutral and precise, not marketing copy. Flag any sentence that:
- Claims a feature is better, faster, simpler, or easier without a specific technical basis ("simplifies your workflow")
- Uses adjective stacking to describe ThunderID's capabilities ("flexible, powerful, enterprise-ready")
- Frames a technical fact as a selling point ("ThunderID's advanced flow engine lets you...")
- Uses "powerful," "rich," "best-in-class," or similar with no accompanying technical specificity
- Describes what the product "allows you to" do when it should just describe what the product does

Promotional tone hides technical meaning. Replace it with the specific technical fact.

- ❌ "ThunderID's flexible flow engine lets you build any authentication experience."
- ✅ "The flow designer supports conditional branching, multi-factor steps, and external service calls within a single sign-in flow."

---

### Generic Writing

A page that contains only definitions and category descriptions, with no technical specifics about how ThunderID implements the concept, is not useful documentation.

**Specificity test**: for each section (a concept page is exempt — it explains ideas, not behaviors), check whether it contains at least one of:
- A specific behavior or constraint unique to ThunderID (not just "this is how OAuth works")
- A concrete example with real values, real steps, or a real outcome
- A trade-off, limitation, or condition the reader needs to know

Flag each section that fails this test on a guide, quickstart, or reference page. Note what kind of concrete detail would make it pass.

**Section necessity test**: for each H2 section, ask whether a reader could skip it entirely and still follow the rest of the page. Filler sections typically restate the introduction, provide generic background the target reader already knows, or could appear in any software product's docs on the same topic unchanged. For each section that fails: flag it, and state whether it should be cut, merged with an adjacent section, or replaced with ThunderID-specific content.

---

### Instruction Precision

For quickstart and guide pages, scan each step instruction for:

**Vague outcomes**: does the step tell the reader what to expect after completing it, when the result is non-obvious?
- ❌ "Configure the OAuth settings."
- ✅ "Set the **Redirect URI** to `http://localhost:3000/callback`. <ProductName /> rejects auth requests that redirect to any other URI."

**Ambiguous UI references**: does the step reference a UI element that might not be clearly identifiable?
- ❌ "Open the settings." (Which settings — application settings, flow settings, server config?)
- ✅ "In the Console, navigate to **Applications** → [App name] → **Protocol**."

---

### Step Count and Decomposition

For quickstart Steppers and any numbered step list in a guide, count the steps in a single sequence.

**Check for a confirmation marker first.** If an HTML comment of the form `<!-- docs-review-style: step-count-confirmed steps=N -->` sits immediately above the Stepper (or the first item of the numbered list) and `N` matches the current step count exactly, skip straight to ✅ passed — this was already confirmed and nothing has changed since. If the marker is present but `N` no longer matches the current count, treat it as stale: run the check below as if no marker existed, since the structure changed after the last confirmation.

Not a hard gate — a long step sequence doesn't automatically fail the review, because sometimes the flat structure genuinely is the right call. When no valid marker is found and a single Stepper or numbered list reaches 10 or more steps:

1. Analyze the actual steps and propose a specific restructuring:
   - **Split into multiple pages** when the steps span genuinely separate tasks a reader might do independently (e.g., "set up the identity provider" vs. "connect it to your application" vs. "test the connection").
   - **Group into labeled phases** within the same page when the steps are one continuous task but have a natural internal structure (e.g., "Configure the basics" for steps 1-4, "Set up security" for steps 5-8, "Verify" for steps 9-10) — use sub-headings or a phase label above each cluster, not a flat renumbered list.
   - **Fold trivial steps into a neighbor** when a step is only "click Save" or similar and doesn't need to stand alone.
2. Present the proposal with your reasoning, and ask the user directly: *"Do you think the current flat structure is the right way to present this, or would restructuring it as proposed work better?"* (See Batching below if Step Locality also needs to ask on the same page.)
3. If the user confirms the current structure is intentional, mark this check ✅ passed in the report — do not leave it as an open ❌ finding just because the count crossed the threshold. Also tell them the exact marker to add so future reviews don't ask again: `<!-- docs-review-style: step-count-confirmed steps={N} -->` placed immediately above the Stepper or list. This skill cannot write it for you (read-only); `/docs-edit` can add it as part of the same edit if the user is about to make other changes to the page. If they agree to restructure instead, mark it `[needs writer input]` pending the actual rewrite.

"Too many steps, please shorten" is not a finding — the count is only the trigger for the conversation, not the verdict.

---

### Step Locality (Minimize Screen Switching)

For step sequences, identify which UI location each step operates in when stated or clearly implied — a tab, page, screen, or section named in the step (e.g., "Navigate to the **Applications** tab").

**Check for a confirmation marker first.** If an HTML comment of the form `<!-- docs-review-style: step-locality-confirmed screens=A,B,C -->` sits immediately above the sequence, and the comma-separated screen names match the current step-by-step screen order exactly (in order, including repeats), skip straight to ✅ passed. If the marker is present but the screen sequence no longer matches, treat it as stale and run the check below as if no marker existed.

Not a hard gate. When no valid marker is found, and two or more steps targeting the same UI location are separated by an intervening step targeting a different location, and the intervening step's outcome isn't a dependency for those steps (and they aren't a dependency for it):

- ❌ "Step 3: In **Settings**, enable SSO. Step 4: In **Applications**, create a new app. Step 5: Back in **Settings**, set the redirect URI."
- ✅ "Step 3: In **Applications**, create a new app. Step 4: In **Settings**, enable SSO and set the redirect URI." (both Settings actions merged and grouped together; Applications work happens first since nothing in Settings depends on it)

Before flagging, check for a genuine dependency: if the reader must return to a screen only because an intervening step produced something that screen's step needs (e.g., an app ID to paste into a Settings field), the reordering is not a violation — say so explicitly and skip the flag entirely. Only surface back-and-forth that exists for no dependency reason.

When it applies:
1. Propose the specific reordered or regrouped step sequence.
2. Ask the user directly: *"Do you think the current order is the right way to go, or should it be regrouped as proposed?"* (See Batching below if Step Count also needs to ask on the same page.)
3. If the user confirms the current order is intentional, mark it ✅ passed. Also tell them the exact marker to add: `<!-- docs-review-style: step-locality-confirmed screens={ordered list} -->` (e.g., `screens=Applications,Settings`), placed immediately above the sequence. Same as above, this skill can only report the line — `/docs-edit` or a manual edit adds it. Otherwise mark it `[needs writer input]` pending the reorder.

"Steps are out of order" alone is not a finding — the user's stated intent decides the outcome, not the detected pattern alone.

---

### Batching Step Count and Step Locality Questions

If both checks need to ask the user on the same page, do not ask twice. Run both checks' detection first, then combine into a single message covering both, the same way `/docs-new-page` batches its upfront questions:

> This page has two structural things worth confirming:
> 1. **Step count** — {N} steps in one sequence. {one-line restructuring proposal, or "no restructuring needed" if only Step Locality fired}
> 2. **Step order** — {screens involved}. {one-line reorder proposal, or "no reorder needed" if only Step Count fired}
>
> Are the current structure and order intentional, or would you like either restructured as proposed?

Process both answers before finalizing the report; each still resolves independently (one can be confirmed while the other gets restructured).

---

### Concept Bleed

Flag any page where content from the wrong doc type has crept in, beyond the specific rules already covered per doc type in Step 4 below (quickstart conceptual detours, concept imperative instructions, reference narrative prose):

- **Guide with reference tables**: a guide that embeds a full parameter reference table instead of linking to the reference page.

For each instance: flag it, identify the doc type it belongs to, and suggest whether to cut, summarize, or link out.

---

## Hard Gate Rules

The rules below are called out as hard gates because they were previously enforced as blocking failures by a standalone AI-pattern review. Every other ❌ finding in this skill should still be fixed before merge, but these four specifically determine the pass/fail `Result` in the Output Format below:

1. Any em dash or en dash is present.
2. 5 or more AI vocabulary instances (Phase 1 banned words) remain.
3. Any rhetorical scaffolding phrase is present.
4. More than one symmetric contrast construction appears.

Step Count and Decomposition and Step Locality (above) are deliberately **not** in this list — they resolve through asking the user whether their structure is intentional, not through a mechanical count or pattern match, so they never contribute to `Result: FAIL` on their own.

---

## Step 4: Doc-Type-Specific Checks

After universal checks, apply the rules for the detected doc type. Type-specific rules below override the universal rules in Step 2 where they conflict. For reference pages in particular, the "no second-person" rule supersedes the universal "address the reader as you" rule.

---

### Quickstart

**Intro sentence**
Must open with: *"Use this guide to [verb] [outcome]."*

- ❌ "This guide explains how to connect React to <ProductName />."
- ✅ "Use this guide to add sign-in to a React app using <ProductName />."

**Step headings (H2 inside Stepper)**
Must be imperative verb phrases, not label nouns:

- ❌ `## Application Configuration` → `## Configure the Application`
- ❌ `## Installation` → `## Install the SDK`
- ❌ `## Authentication Setup` → `## Set Up Authentication`

**No conceptual detours mid-step**
If background context is needed, link out: *"See [Flows](relative-link)."* Do not embed explanations inside a step. Flag any inline concept explanation longer than one sentence.

**Page ending label**
Must be `## What's Next`. Flag any other label.

---

### Guide

**Intro sentence**
Lead with the task or outcome the reader will achieve. Do not open with a page description.

- ❌ "This guide is about configuring X."
- ❌ "This guide walks you through X." — starts with a page description, not the task
- ✅ "Configure X to enable Y in your application."
- ✅ "Add X support to your ThunderID deployment by updating the flow."

**No "This page/guide/document" sentence openers in the body**
Flag these in the body text — they signal weak intros that lead with the page rather than the task. Note: `/docs-check` only checks this pattern in the `description` frontmatter field, not in body text.

**Page ending label**
Must be `## Next Steps` or `## Related Guides`. Flag `## Go Further`, `## What's Next?` (with question mark), or anything else.

---

### Concept

**No imperative instructions**
Concept pages explain; they do not instruct. Flag any "Click X", "Run Y", "Navigate to Z". These belong in a guide — link there instead.

**Opening must define the concept**
The first paragraph must answer: *"What is X?"* It must not begin with a task, a list, or a historical claim.

**No "How to" in the title or any heading**
Concept pages are not tasks. A heading like `## How to Use Flows` in a concept page signals misplaced content — either move it to a guide or reframe it as explanation.

**Page ending label**
Must be `## Related`. Flag any other label.

---

### Reference

**No narrative prose where a table fits better**
If a section describes a list of fields, parameters, or options in paragraphs, flag it and suggest a table.

**No second-person address**
Reference material is read non-linearly. Avoid "you" — write in the imperative or third-person for parameter descriptions.

- ❌ "You can set this to `true` to enable caching."
- ✅ "Set to `true` to enable caching."

**No "you need to" or "you should" constructions**
Reference pages state facts, not guidance.

**No page ending section required.** Skip the ending check.

---

### Use-Case

**Intro must frame scenario and audience**
The opening paragraph must answer: what scenario does this cover, and what kind of product or user does it apply to? It must not open with a feature list.

**"When to Choose This Pattern" section**
Must use bullet criteria, not prose paragraphs. If written as prose, flag and suggest converting to a bullet list.

**Page ending label**
Must be `## Try It Out` or `## Next Steps`. Flag other labels.

---

## Output Format

Group findings by check category. Omit categories with no issues. For each finding:
- Quote the original text (with line number where possible)
- State what is wrong in one line
- Suggest a rewrite, or mark `[needs writer input]` if context is required to fix it

```
Reviewing: docs/content/guides/guides/applications/manage-applications.mdx
Doc type: guide
Sibling pages read: manage-users.mdx, manage-roles.mdx

CROSS-PAGE CONSISTENCY
  ❌  This page opens each step with a one-sentence imperative ("Click Save."), but manage-users.mdx
      and manage-roles.mdx both lead steps with a one-line rationale before the action
      (e.g., "To scope this role to a team, select an organization unit first."). This page reads
      terser and less explained than its siblings.
      → Add a short rationale clause before at least the non-obvious steps, matching the sibling pattern.

ACTIVE VOICE
  ❌  line 23: "The application can be deleted by navigating to Settings."
      → "Navigate to Settings and delete the application."

AI VOCABULARY
  ❌  line 41: "leverage the SDK" → "use the SDK"
  ❌  line 67: "streamline your workflow" → cut or replace with what it specifically does

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

If a doc type check passes cleanly, include one line for it:
```
QUICKSTART: STEP HEADINGS
  ✅  all H2 headings are imperative verb phrases
```

`Result` is `FAIL` only when at least one Hard Gate Rule (above) is triggered; otherwise it is `PASS` even if non-gating ❌ issues remain — those still need fixing before merge, but a human editorial call is expected on judgment-based findings (e.g., cross-page consistency, section necessity) in a way a hard gate is not. If all checks pass cleanly with zero issues:

```
─────────────────────────────────────
Result: PASS
Hard gates: 0
Issues: 0 failures · N pass
```
