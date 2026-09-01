---
paths:
  - "frontend/**/*.tsx"
  - "docs/src/**/*.tsx"
  - "docs/src/**/*.ts"
---

# Oxygen UI Conventions

`@wso2/oxygen-ui` is the component library for both the React apps under `frontend/` and the Docusaurus site's own
components under `docs/src/`. Both depend on the same catalog-pinned version, so the rules are identical in both places.

The codebase follows these almost perfectly today (zero imports from `@mui/*`, zero from `lucide-react`) but **nothing
enforces it**: there is no `no-restricted-imports` rule. That makes it easy for one stray import to establish a
precedent, so it is worth checking on every `.tsx` you touch.

## The five critical rules

1. **Import components from `@wso2/oxygen-ui`, never from `@mui/material`.** Oxygen UI is built on MUI v7 and re-exports
   what you need.
2. **Import icons from `@wso2/oxygen-ui-icons-react`** (a Lucide React wrapper), never from `@mui/icons-material` or
   `lucide-react`. Custom brand or technology logos are the exception: those live in
   `frontend/packages/components/src/icons/logos/vendor/` and `docs/src/components/icons/`.
3. **Wrap the tree in `OxygenUIThemeProvider`.** Already done per app; see `frontend/apps/console/src/hocs/withTheme.tsx`
   and the Gate equivalent. Both pass a CSP `nonce`, so do not drop it.
4. **MUI X components come through namespaces**: `DataGrid.DataGrid`, `DatePickers.DatePicker`, `TreeView.TreeView`.
   Types come the same way (`DataGrid.GridColDef`, `DataGrid.GridRenderCellParams`). In app code, prefer
   `ListingTable.DataGrid` over a bare `DataGrid.DataGrid` (see below).
5. **Style with theme tokens, not literals.**
   ```tsx
   <Box sx={{p: 2, bgcolor: 'background.paper', color: 'text.primary'}} />   // correct
   <Box sx={{padding: '16px', backgroundColor: '#ffffff', color: '#333'}} /> // wrong
   ```

## Styling mechanism

- Use the `sx` prop. Reach for `styled()` only when `sx` genuinely cannot express it (complex descendant selectors,
  keyframes) — there are three `styled()` calls in the whole frontend, against roughly 2,400 uses of `sx`.
- Do not use inline `style={{...}}`.
- In `docs/src/`, do not add CSS to `docs/src/css/custom.css` unless it targets Scalar API-reference elements or another
  third party that `sx`/`styled()` cannot reach.

## Prefer components over raw HTML

Raw HTML elements are effectively absent from app code: no `<form>`, `<label>`, or `<table>` at all, and single-digit
counts of `<button>` and `<input>`. Use `Box` for `<div>`, `Typography` for text and headings (`variant="h3"`, not
`<h2>`), `Button` for `<button>`, and the Oxygen form components for inputs. A bare `<div>` is acceptable only as an
unstyled wrapper. SVG elements inside a deliberate inline diagram are exempt.

Prefer a Mermaid diagram over a hand-built SVG for architecture, flow, or sequence diagrams. Accept raw SVG only when
the layout is something Mermaid cannot express.

## Custom Oxygen components worth knowing

Reaching for a hand-rolled equivalent of one of these is the most common avoidable mistake:

- **Layout**: `AppShell`, `Layout`, `Header`, `Sidebar`, `Footer`, `PageContent`
- **Data display**: `ListingTable`, `PageTitle`, `CodeBlock`, `StatCard`
- **Forms**: `Form.CardButton`, `Form.Section`, `Form.Stack`, `Form.Wizard`
- **Inputs**: `SearchBar`, `SearchBarWithAdvancedFilter`, `ComplexSelect`
- **Feedback**: `NotificationPanel`, `NotificationBanner`
- **Theming**: `OxygenUIThemeProvider`, `ThemeSwitcher`, `ColorSchemeToggle`, `ColorSchemeImage`

## Page and listing structure in the Console

- A page is `<PageContent>` wrapping `<PageTitle>` with `<PageTitle.Header>` and `<PageTitle.SubHeader>`. Canonical:
  `frontend/packages/configure-design/src/pages/DesignPage.tsx`.
- An `<ExternalLink docKey="...">` in the subheader resolves through `useConfig().getDocumentationLink` against the key
  registry in `frontend/apps/console/public/config.js`. **It renders `null` when the key has no URL**, and most keys
  there are currently empty, so adding a documentation link means editing `config.js`, not just the component.
- Listings use `ListingTable` (`ListingTable.Provider` → `ListingTable.Container` → `ListingTable.DataGrid`), not a bare
  DataGrid. Canonical: `frontend/packages/configure-users/src/components/UsersList.tsx`. Within it:
  `ListingTable.CellIcon` for the primary cell; `ListingTable.RowActions` for the trailing actions column; every row
  action `IconButton` wrapped in a `<Tooltip>` and calling `e.stopPropagation()` because the whole row is clickable;
  `ListingTable.EmptyState` for empty results.
- **`localeText={useDataGridLocaleText()}`** (from `@thunderid/hooks`) is required on every DataGrid for i18n and is
  easy to forget.
- Full-screen creation flows use `FullScreenCreationWizardLayout` from `@thunderid/components`. Read its JSDoc before
  changing layout props: the content column stays left-aligned whether or not a `preview` panel is present, deliberately,
  so that toggling the panel between steps does not also shift the content.

## Icon usage

Icons take a numeric `size` prop, not MUI's `fontSize`. The house sizes are `16` (default), `14`, `18`, and `20`.
Note the library exposes some icons under more than one name (`Trash2`/`Trash`, `Pencil`/`Edit`, `Plus`/`PlusIcon`);
match whatever the surrounding file already uses rather than introducing the second spelling.

## The Library Ships Its Own Fuller Reference

`@wso2/oxygen-ui` bundles about 3,300 lines of component and theming documentation inside the installed package, at
`frontend/apps/console/node_modules/@wso2/oxygen-ui/.ai/`:

| File | Covers |
|---|---|
| `AGENTS.md` | The critical rules above, plus the list of custom components and available themes |
| `components.md` | Component-by-component API reference |
| `patterns.md` | Worked UI patterns |
| `theming.md` | Theme structure and customization |
| `migration.md` | Migrating MUI code to Oxygen UI |

Read those directly when you need an API detail this guide does not cover. They are not vendored into the repository on
purpose: reading them from `node_modules` means you always get the documentation for the version actually installed
(currently 0.13.1, pinned by the `pnpm-workspace.yaml` catalog), with no copy to keep in sync.

### Do Not Run `npx @wso2/oxygen-ui init` in This Repository

The package ships a CLI that installs those docs into a consuming project. It is incompatible with this repository's
layout in three ways, and running it will cause damage:

1. Its `--claude` mode writes into `.claude/skills/`, which here is a **symlink to the tracked `.agent/skills/`**, so it
   would add four vendor skills to our own skill tree.
2. It creates or rewrites the root `CLAUDE.md`, which here is the `@AGENTS.md` import shim.
3. It writes `.claude/oxygen-ui/`, which `.gitignore` excludes, so nothing it produced would be committed anyway.

Its universal mode is no better: it creates a root `AGENTS.md`, which already exists.

## Flagging this in review

Flag as a defect: any import from `@mui/*` or another icon library; an inline `style={{...}}` prop; a hardcoded color,
hex value, or pixel spacing in `sx`; a raw HTML element that has an obvious Oxygen equivalent; a bare DataGrid in a
Console listing; a DataGrid missing `localeText`; and new `custom.css` rules that `sx` or `styled()` could express.
