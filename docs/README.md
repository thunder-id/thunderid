# ThunderID Documentation ⚡

Documentation for **ThunderID** - a modern identity management suite. This documentation covers installation, configuration, development, and contribution guidelines for the ThunderID platform.

## Writing and reviewing docs with agent skills

Contributors do not have to write and review documentation from scratch, or memorize every style rule by hand. This repo ships a set of Claude Code skills, invoked as `/skill-name` in an agent session, that scaffold, write, and review ThunderID docs consistently. They live in `.agent/skills/` (mirrored in `.claude/skills/` for Claude Code), and the ones a contributor uses directly are:

- **`/docs-new-page`**: scaffolds a new page. Collects the page type, title, and description, checks whether a similar page already exists before creating a duplicate, creates the file from the matching template, and proposes where it belongs in the sidebar for your approval.
- **`/docs-edit`**: writes content into a page, whether that is a placeholder left by `/docs-new-page`, a gap in something you started, or a new section on an existing page. You can also hand it a draft, a design doc, or notes to write from. It verifies every technical claim against the codebase, an existing doc, or the draft you supplied before writing it, and asks you for the missing fact instead of guessing.
- **`/docs-review`**: the full pre-merge check. Structure, writing quality, and technical accuracy in one command.
- **`/docs-seo`**: an optional discoverability pass for new pages or major rewrites.

### Example workflow

Say you want to add a page explaining how adaptive authentication works in ThunderID. This example walks through the workflow of documenting the content (the specifics of adaptive authentication are not the point here).

1. **Start with `/docs-new-page`.** Give it the page type (this would likely be a `concept` page), a working title, and roughly where it belongs. It searches the existing docs first: if a page on adaptive authentication, or something close enough to be the same topic, already exists, it stops and asks whether you meant to edit that page instead. If not, it scaffolds the file from the concept template and proposes a sidebar position for you to approve before touching `sidebars.ts`.

2. **Fill it in with `/docs-edit`.** For each section, it identifies the factual claim the section needs, for example "what signals ThunderID's risk engine actually evaluates," and checks it against the codebase or existing docs before writing anything. If you already have a design doc or notes on adaptive authentication, hand those over too: `/docs-edit` treats them as a source, still checks any checkable claim against the codebase, and flags it if the two disagree rather than trusting either one blindly. If it cannot verify a claim from any source, it stops and asks you for the specific fact instead of writing something that sounds plausible. This is the step where you supply the real product knowledge; the skill's job is to make sure nothing gets published that nobody actually confirmed.

3. **Review it with `/docs-review`** before opening a PR. It runs structure, writing-quality, and technical-accuracy checks together and gives you one pass/fail result. If it flags something like an oversized step sequence or an awkward step order, that is not a hard failure: it proposes a fix and asks whether you agree. Decline if your structure is intentional, and it moves on.
4. **Run `/docs-seo`** if this is a new page, to catch a generic title or heading before it ships.

### What not to do

- **Do not ask a skill to "just write the adaptive authentication page" and accept whatever it produces without supplying the real facts.** If `/docs-edit` cannot verify a claim, that is the system working as intended. Give it the specific detail instead of nudging it to guess.
- **Do not create a new file by hand and skip `/docs-new-page`.** Its duplicate check exists because two contributors can otherwise write two competing pages on the same topic, and its sidebar step exists because a page without a sidebar entry fails CI.
- **Do not treat every suggestion as a blocker.** Some checks, like step count, step ordering, or sidebar placement, are deliberately not hard gates. They ask whether your structure or placement is intentional. If it is, say so and move on.
