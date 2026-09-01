---
paths:
  - "docs/src/**"
  - "docs/docusaurus.config.ts"
  - "docs/docusaurus.product.config.ts"
  - "docs/sidebars.ts"
  - "docs/plugins/**"
  - "docs/scripts/**"
---

# Docusaurus Site Code

Rules for the machinery of the documentation site (`docs/src/`, plugins, config, build scripts), not for writing
documentation content. Content under `docs/content/` is the `docs` skill's job.

For component and styling conventions in `docs/src/`, see `.agent/guides/oxygen-ui.md` as well: the site's own
components use the same Oxygen UI library as the React apps.

## Versioned Fetches Must Use `useDocsVersion()`

A component rendered inside actual doc content (an `.mdx` page under `docs/content/` or `docs/versioned_docs/`) that
builds a URL to `fetch()` a versioned static asset (anything mirrored per-version under `static/docs/<versionPath>/...`
or `static/api/<versionPath>/...`) must derive `versionPath` from `useDocsVersion()`
(`@docusaurus/plugin-content-docs/client`), mapping `version === 'current' ? 'next' : version`. This is the pattern
already used in `docs/src/components/ApiVersionReference.tsx`.

Do not hardcode a `/docs/next/...` literal and rewrite it with `useDocsUrl()` (`@site/src/hooks/useDocsUrl`). That hook
exists for version-*less* contexts (footer links, homepage, custom pages) that fall back through the site's global
active/preferred/latest version state. A component embedded in a specific doc page already has its own authoritative
version and should read it directly.

`useDocsUrl()` remains the correct tool for rewriting `<Link>`/`href` navigation targets. This rule applies only to
runtime `fetch()` URLs.

## Product Name Templating

Never hardcode the product name. The correct mechanism depends on where the text sits:

| Location | Use |
|---|---|
| `.mdx` prose, headings, JSX string content | `<ProductName />` (globally registered, no import needed) |
| `.mdx` frontmatter (`title`, `description`, `sidebar_label`) | `{{ProductName}}` (replaced at build time via `parseFrontMatter`) |
| `.mdx` fenced code blocks and inline code spans | `{{ProductName}}` (replaced by the `rehypeProductName` plugin) |
| `.mdx`, repository URLs and GitHub links | `<RepoLink path="/issues">link text</RepoLink>` |
| `.ts`/`.tsx` under `docs/src/` | read it from site config (below); in JSX prefer `<ProductName />` |

The product slug follows the same rule, as `{{productSlug}}`.

In `docs/src/` TypeScript, read the values from Docusaurus config rather than hardcoding them:

```ts
const {siteConfig} = useDocusaurusContext();
const {project} = siteConfig.customFields?.product as DocusaurusProductConfig;
const productName = project.name;
const repoUrl = project.source.github.url;
```

Exception: the marketing tagline in `docs/docusaurus.product.config.ts` is a deliberate slogan, not the prose category
descriptor. Leave it alone.

## Validation

`make lint_docs` runs Vale plus `scripts/docs-lint.sh`. `make build_docs` catches broken links and MDX compile errors
that linting alone misses. Neither is part of `make pr_checks`, so run both when you change anything under `docs/`.

## Flagging this in review

Flag as a defect: a hardcoded `/docs/next/...` literal rewritten with `useDocsUrl()` for a `fetch()` URL; a hardcoded
product name or repository URL in any of the locations above; and new `docs/src/css/custom.css` rules that `sx` or
`styled()` could express.
