# ThunderID Documentation ⚡

Documentation for **ThunderID** - a modern identity management suite. This documentation covers installation, configuration, development, and contribution guidelines for the ThunderID platform.

## Writing and reviewing docs with agent skills

Contributors do not have to write and review documentation from scratch, or memorize every style rule by hand. This repo ships a Claude Code skill, `docs` (invoked as `/docs` in an agent session, or automatically whenever a request matches), that scaffolds, writes, and reviews ThunderID docs consistently. It lives in `.agent/skills/docs/` (mirrored in `.claude/skills/` for Claude Code): a `SKILL.md` dispatch table plus one reference file per stage, so a request only loads the part it actually needs. The reference files a contributor's requests route to:

- **`new-page.md`**: scaffolds a new page. Collects the page type, title, and description, checks whether a similar page already exists before creating a duplicate, creates the file from the matching template, and proposes where it belongs in the sidebar for your approval.
- **`edit.md`**: writes content into a page, whether that is a placeholder left by `new-page.md`, a gap in something you started, or a new section on an existing page. You can also hand it a draft, a design doc, or notes to write from. It verifies every technical claim against the codebase, an existing doc, or the draft you supplied before writing it, and asks you for the missing fact instead of guessing.
- **`check.md`**: structural standards only — frontmatter, headings, links, Stepper config, sidebar registration.
- **`style.md`**: writing quality only — AI-sounding prose, tone, voice consistency, em/en dashes, and more.
- **`tech.md`**: technical accuracy only — protocol, API, SDK, config, and security claims verified against source.
- **`api.md`**: API documentation specifically — an OpenAPI spec (`api/*.yaml`) verified against the Go backend's actual registered routes, or an SDK reference page (`docs/content/sdks/*/apis/**`) verified against build artifacts when available locally. Unlike `seo.md`, this isn't advisory: technical accuracy is a hard gate here.
- **`review.md`**: the full pre-merge check, running `check.md`, `style.md`, `tech.md`, and (for API-reference paths) `api.md` together. Also has a diff mode: ask it to "review my changes" or "review the diff" without naming a file, and it reviews every changed doc file — including anything not yet committed — instead of one file at a time.
- **`seo.md`**: an optional discoverability pass for new pages or major rewrites.

### Invoking a specific stage

Natural language works for all of the above ("check this file," "review my changes," "write this section"), and the skill matches intent to the right reference file on its own. If you want to be explicit about which stage runs instead of relying on that match, invoke it as `/docs <action> [file-path]`:

| Action | Runs |
|---|---|
| `new-page` | `new-page.md` |
| `edit` | `edit.md` |
| `check` | `check.md` |
| `style` | `style.md` |
| `tech` | `tech.md` |
| `api` | `api.md` |
| `seo` | `seo.md` |
| `review` | `review.md` (omit the file path to review the diff instead of one file) |

### Example workflow

Say you want to add a page explaining how adaptive authentication works in ThunderID. This example walks through the workflow of documenting the content (the specifics of adaptive authentication are not the point here).

1. **Start by asking for a new page.** Give it the page type (this would likely be a `concept` page), a working title, and roughly where it belongs. It searches the existing docs first: if a page on adaptive authentication, or something close enough to be the same topic, already exists, it stops and asks whether you meant to edit that page instead. If not, it scaffolds the file from the concept template and proposes a sidebar position for you to approve before touching `sidebars.ts`.

2. **Fill it in.** For each section, it identifies the factual claim the section needs, for example "what signals ThunderID's risk engine actually evaluates," and checks it against the codebase or existing docs before writing anything. If you already have a design doc or notes on adaptive authentication, hand those over too: it treats them as a source, still checks any checkable claim against the codebase, and flags it if the two disagree rather than trusting either one blindly. If it cannot verify a claim from any source, it stops and asks you for the specific fact instead of writing something that sounds plausible. This is the step where you supply the real product knowledge; the skill's job is to make sure nothing gets published that nobody actually confirmed.

3. **Ask for a full review** before opening a PR. It runs structure, writing-quality, and technical-accuracy checks together and gives you one pass/fail result. If it flags something like an oversized step sequence or an awkward step order, that is not a hard failure: it proposes a fix and asks whether you agree. Decline if your structure is intentional, and it moves on. If your change touched more than this one page, ask it to "review my changes" instead of naming a file — it reviews every changed doc file at once, uncommitted included.
4. **Ask for an SEO check** if this is a new page, to catch a generic title or heading before it ships.

### What not to do

- **Do not ask it to "just write the adaptive authentication page" and accept whatever it produces without supplying the real facts.** If it cannot verify a claim, that is the system working as intended. Give it the specific detail instead of nudging it to guess.
- **Do not create a new file by hand and skip the new-page step.** Its duplicate check exists because two contributors can otherwise write two competing pages on the same topic, and its sidebar step exists because a page without a sidebar entry fails CI.
- **Do not treat every suggestion as a blocker.** Some checks, like step count, step ordering, or sidebar placement, are deliberately not hard gates. They ask whether your structure or placement is intentional. If it is, say so and move on.
