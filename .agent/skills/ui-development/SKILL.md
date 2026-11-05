---
name: ui-development
description: Oxygen UI component, styling, and layout conventions for the ThunderID Console and Gate. Use whenever writing or changing React UI under frontend/, including adding a page, dialog, form, table, card, or shared component, styling with sx or the theme, choosing an icon, or deciding whether a component already exists before writing one.
allowed-tools: Read Write Edit Bash
---

# Frontend UI: Oxygen UI, Styling, and Components

ThunderID's UI is built on **Oxygen UI** (`@wso2/oxygen-ui`), WSO2's component library on top of Material UI v7, plus
ThunderID's own shared components in `@thunderid/components`. The goal of this skill is that every screen looks and
behaves the same, and that no feature package grows a private copy of something that already exists.

Read with [frontend/AGENTS.md](../../../frontend/AGENTS.md), which covers routing, error display, and i18n fallbacks.

## 1. Before Writing Any Component, Look It Up

Resolve every UI need in this order and stop at the first hit:

1. **`@thunderid/components`**: ThunderID's shared building blocks (catalog in section 3). Read
   `frontend/packages/components/src/index.ts` for the current export list. Never trust a copied list, this one
   included.
2. **Oxygen custom components**: `PageContent`, `PageTitle`, `ListingTable`, `AppShell`, `Header`, `Sidebar`, `Form.*`,
   `SearchBar`, `ComplexSelect`, `StatCard`, `CodeBlock`, `NotificationBanner`, `UserMenu`, and others.
3. **Oxygen's re-exported MUI primitives**: `Box`, `Stack`, `Typography`, `Button`, `Dialog*`, `Alert`, `Tabs`,
   `TextField`, `Tooltip`, `CircularProgress`, MUI X as namespaces (`DataGrid.DataGrid`, `DatePickers.DatePicker`,
   `TreeView.TreeView`), and the rest.
4. **Only then a new component.** If it is needed by one feature, keep it in that `configure-*` package. If a second
   package needs it, move it into `@thunderid/components` instead of copying it.

## 1a. Read the Component's Docs and Guides First

Every Oxygen component has a docs page, and many carry usage guidance: which variant to use when, and an
**Accessibility** block describing how to label and operate it. Read them before using a component you have not used
in this codebase, and before changing how an existing one is configured (switching a table variant, adding a toolbar,
changing a dialog's actions). Do not guess a prop or variant.

Check the pinned version first. It is the `@wso2/oxygen-ui` entry in the root `pnpm-workspace.yaml` catalog, written
`<version>` below.

| Source | Where | Use it for |
|---|---|---|
| Storybook docs | https://wso2.github.io/oxygen-ui, page per component | Description, "use this variant when" notes, Accessibility block, live examples. Tracks Oxygen's `main` branch, so it may document props newer than the pinned version |
| Story source at the pinned version | `https://github.com/wso2/oxygen-ui/tree/@wso2/oxygen-ui@<version>/packages/oxygen-ui-docs/stories/<Category>/<Component>.stories.tsx` | The same guidance, matching what is installed. Authoritative when it disagrees with the hosted Storybook |
| How-to guides and page templates | `stories/Guides/` (browser tab title), the root-level `stories/HowToAddRuntimeThemeSupport.stories.tsx`, and `stories/Templates/` (wizard, tabbed content, empty state, form validation, create-service form, dashboard, login) | Composed patterns, before assembling a page from scratch |
| Accessibility audit | `packages/oxygen-ui-docs/ACCESSIBILITY.md` at the same tag | WCAG 2.1 AA status, per-component fixes, known exceptions (section 7) |
| Bundled agent docs | `frontend/apps/console/node_modules/@wso2/oxygen-ui/.ai/` (`AGENTS.md`, `components.md`, `patterns.md`, `theming.md`, `migration.md`) | Quick offline API reference. It is incomplete and can lag the stories: in 0.16.0 `components.md` lists only the `table` and `card` `ListingTable` variants (the stories also document `data-grid` and `data-grid-card`), and its `PageTitle` section omits `PageTitle.BackButton` and `PageTitle.Actions` |
| Type definitions | `frontend/apps/console/node_modules/@wso2/oxygen-ui/dist/components/<Name>/*.d.ts` | The final word on which props and sub-components exist in the installed version. Check here whenever the docs omit something |

Fetch a story file without cloning:

```bash
gh api 'repos/wso2/oxygen-ui/contents/packages/oxygen-ui-docs/stories/AppElements/ListingTable.stories.tsx?ref=@wso2/oxygen-ui@0.16.0' \
  -H 'Accept: application/vnd.github.raw'
```

Story files are large (the `ListingTable` one is about 2,900 lines), so pipe the output through
`grep -n -i "use this\|when\|accessib"` to find the guidance. It sits in the JSDoc above each exported story and in
the meta's `parameters.docs.description`. List the story folders with the same call on
`packages/oxygen-ui-docs/stories`.

The `node_modules` paths in this table exist only after `pnpm install`. Without an install (a fresh worktree, a
sandbox), read the same files from the GitHub tag: the bundled docs are at `packages/oxygen-ui/.ai/`.

When a story's guidance conflicts with how ThunderID already uses the component, follow the ThunderID convention in
section 4 and mention the conflict in the PR. Changing a convention changes it everywhere, so it is a separate
decision. Use a documented prop or default before writing custom styling to get the same effect.

## 2. Imports

| Need | Import from | Never from |
|---|---|---|
| Components, `useTheme`, `useColorScheme`, `alpha`, types | `@wso2/oxygen-ui` | `@mui/material`, `@mui/x-*`, `@mui/system` |
| Icons | `@wso2/oxygen-ui-icons-react` | `@mui/icons-material`, `lucide-react` |
| Shared ThunderID components | `@thunderid/components` | a relative path into another package |
| Theme types | `@wso2/oxygen-ui` (the one existing `@wso2/oxygen-ui/styles/index` import is legacy, do not copy it) | |

No app or package source imports `@mui/*` (only `test-utils` mocks and the Vite configs reference it). Keep it that
way. `@emotion/*` is imported directly in two packages only: `keyframes` in two `configure-flows` files, and the
Emotion cache that scopes styles into the Gate preview iframe in `configure-design`. Do not add others.

Third-party brand marks (Google, GitHub, React, and so on) are exported from `@thunderid/components` as `*Icon` /
`*Logo`. Add a new vendor mark there, not inside a feature package.

## 3. ThunderID Shared Components

| Component | Use for | Do not |
|---|---|---|
| `DocumentTitle` | The browser tab title on every routed page: `<DocumentTitle title={...} />` renders `Title \| <product name>` from the brand config | Set `document.title` or use `Helmet` for a title |
| `Helmet` | Other `<head>` tags (meta, link, script) | Use it for the page title |
| `PageLoadingAnimation` | In-page loading state (centered spinner, `role="status"`, min height 400px) for a page body or tab | Hand-roll a centered `Box` + `CircularProgress` |
| `PageLoader` | Full-viewport loading before any layout exists: the lazy-route `Suspense` fallback in each `App.tsx` | Use it inside `PageContent` |
| `QueryErrorNotice` | Every read failure (see Error Display in frontend/AGENTS.md) | Hand-roll an `<Alert>` or empty state for a failed query |
| `SettingsCard` | Each titled section of an edit page tab, including the danger zone | Build sections from bare `Paper` + `Typography` |
| `UnsavedChangesBar` | The fixed save/reset bar on edit pages with a dirty form | Put a save button at the bottom of each tab |
| `FullScreenCreationWizardLayout` | Every create flow: breadcrumbs, close, progress bar, optional live preview, footer actions | Build a wizard frame per feature |
| `CopyableField` | A labeled read-only value with a copy button (IDs, endpoints, client IDs) | Write another `navigator.clipboard` + "copied" timeout handler |
| `CopyableId` | An unlabeled inline monospace ID with copy | Same as above |
| `ExternalLink` | A "Learn more" link to the ThunderID docs page for a `docKey`. Confirms before leaving by default and renders nothing when no doc link is configured. List page subheaders are split today: about half pass `confirmBeforeNavigate={false}`. Match the sibling pages and do not settle it in a feature PR | Use a bare `<a target="_blank">` |
| `useExternalLinkConfirmation` + `ExternalLinkConfirmDialog` | Any other external URL that leaves the Console (reference: `AgentOverview.tsx`, `IntegrationGuides.tsx`) | Open external URLs without the confirmation |
| `ResourceAvatar` | The avatar/icon of a resource (application, agent, and so on), editable or not | Render a bare `Avatar` with emoji/URL logic |
| `NameSuggestion` | Suggested names in create flows | |
| `ToggleCard` | A bordered checkbox row with title and subtitle | |
| `OrganizationUnitSummaryChip` | Showing which organization unit a resource belongs to | |
| `I18nTextInput` | A text field whose value can be an i18n key | |
| `BuilderLayout` and `Builder*Panel` | Canvas-style editors (flows, design) | |
| `getInitials` | Two-letter avatar fallback from a name | Re-implement initials |

Also exported, for narrower needs: `Kbd`, `EmojiPicker`, `LogoPicker`, `CspOriginHint`, `GradientBorderButton`,
`StackblitzQuickstartCard`, and the vendor logos.

Known duplication to avoid extending:

- About 30 files hand-roll a clipboard copy handler, and `configure-gateways` (`CopyableUrl`) and
  `configure-connections` (`ReadOnlyCopyField`) each have a local copy field. New code uses `CopyableField` or
  `CopyableId`.
- `CreateUserTypePage` and `CreateResourceServerPage` build their own wizard frame (progress bar, breadcrumbs, close
  button) instead of using `FullScreenCreationWizardLayout`. New create flows use the shared layout.

Do not refactor the existing copies as part of unrelated work. Mention them as a follow-up instead.

## 4. Page Anatomy

Match the existing shape exactly. Read the reference file before writing a new page of the same kind.

The reference files show **structure** (which components, in what order). They predate some rules here and in
frontend/AGENTS.md, so do not copy their details. For example, `GroupEditPage` hardcodes `listUrl = '/groups'` instead
of using `useGroupRoutes()`, has untranslated `aria-label`s, and uses `px` literals. Some shared components
(`ExternalLink`, `CopyableField`) and `AgentOverview` also use `style={{}}`, `rgba()` or `px` internally. Write new code
to the rules, not to the nearest example.

**List page** (reference: `packages/configure-groups/src/pages/GroupsListPage.tsx`)

```tsx
<PageContent>
  <DocumentTitle title={t('groups:listing.title', 'Groups')} />
  <PageTitle>
    {/* The reference file omits the fallback strings. New code adds them, per frontend/AGENTS.md. */}
    <PageTitle.Header>{t('groups:listing.title', 'Groups')}</PageTitle.Header>
    <PageTitle.SubHeader>
      {t('groups:listing.subtitle', '...')} <ExternalLink docKey="groups" confirmBeforeNavigate={false} />
    </PageTitle.SubHeader>
    <PageTitle.Actions>
      <Button variant="contained" startIcon={<Plus size={18} />} onClick={...}>{t('groups:listing.addGroup', 'Add group')}</Button>
    </PageTitle.Actions>
  </PageTitle>
  <GroupsList />
</PageContent>
```

The table is `ListingTable.Provider variant="data-grid-card" loading={isLoading}` >
`ListingTable.Container disablePaper` > `ListingTable.DataGrid`, with `ListingTable.RowActions` for per-row menus
(reference: `packages/configure-groups/src/components/GroupsList.tsx`). Every top-level `ListingTable` resource list in
the Console uses `data-grid-card`. Not every list is a `ListingTable`, though:

- Member and assignment sub-lists inside edit pages and dialogs use `DataGrid.DataGrid` directly (groups
  `ManageMembersSection`, roles `ManageAssignmentsSection`).
- Connections and design render card grids, and organization units render a tree view.

Match the sibling surface. Choose a different variant or list type only after reading the variant guidance in the
`ListingTable` stories.

**Edit page** (reference: `packages/configure-groups/src/pages/GroupEditPage.tsx`): `PageContent` with
`PageTitle.BackButton` (a `Link` built from the package's routes hook), `PageTitle.Header` with an inline-editable
name, `Tabs` with one tab panel per settings area, each panel a `Stack spacing={3}` of `SettingsCard`s, the destructive
action in a "danger zone" `SettingsCard` on the last tab, and `UnsavedChangesBar` when the form is dirty. Loading
renders `PageLoadingAnimation`, a failed read renders `QueryErrorNotice`, and both keep the `DocumentTitle`.

**Create flow**: a full-screen page in `FullScreenCreationWizardLayout`. Its route mounts under `FullScreenLayout` in
`App.tsx`, outside the `DashboardLayout` app shell.

**Delete confirmation** (reference: `packages/configure-groups/src/components/GroupDeleteDialog.tsx`): `Dialog`
`maxWidth="sm" fullWidth`, `DialogTitle`, `DialogContentText`, a `warning` `Alert` stating it cannot be undone, the
error `Alert` inside the dialog, Cancel as a text `Button`, and confirm as `variant="contained" color="error"` whose
label switches to `common:status.deleting` while pending. Close only on success.

**Padding**: pages inside the app shell rely on `PageContent`'s default padding. Do not add outer padding or margins.
Full-screen pages (create wizards, onboarding, import/export) own their layout.

## 5. Styling

Style with the `sx` prop and theme tokens. Do not use `style={{}}`, CSS or SCSS files, or `styled()` for a one-off.

These rules, and the accessibility rules in section 7, govern **new and changed code**. Older code still has about 48
files with `rgba()` literals, about 40 with `style={{}}`, a handful that drive styles from `useColorScheme().mode`, and
many controls without accessible names. Do not copy those patterns from a neighboring file. Do not rewrite them
either, unless you are already changing that code.

| Property | Do | Not |
|---|---|---|
| Spacing | `p: 2`, `mb: 3`, `gap: 1`, `Stack spacing={2}` (theme units, 8px each) | `padding: '16px'`, `mb: '24px'` |
| Color | `color: 'text.secondary'`, `bgcolor: 'background.paper'`, `borderColor: 'divider'`, `color="error"` | `'#fa7b3f'`, `'rgba(0,0,0,0.5)'`, `'grey'` |
| Color outside `sx` (icon `color`, SVG fill) | `theme.vars?.palette.warning.main` from `useTheme()`, which follows the color scheme | `theme.palette.*` in render output, which does not switch with dark mode |
| Translucency | `alpha(theme.palette.primary.main, 0.08)` from `@wso2/oxygen-ui`, for colors that are the same in both schemes (brand primary, error). For a scheme-dependent color, set each scheme with `theme.applyStyles` | an `rgba()` literal |
| Text | `Typography variant="body2"` and the theme scale (`h1`-`h6`, `subtitle1/2`, `body1/2`, `caption`, `overline`) | `fontSize: 13`, `fontWeight: 600` to fake a variant |
| Radius, shadow | Component defaults and `borderRadius: 1` (theme units) | `borderRadius: '8px'`, custom `boxShadow` |
| Dark mode | `...theme.applyStyles('dark', {...})` inside an `sx` callback, which switches through CSS variables | Branching styles on a mode value: `theme.palette.mode` never updates on a scheme switch (Oxygen's `theming.md` shows it, but it is pinned to the default scheme), and `useColorScheme().mode` re-renders the whole tree and is `'system'` unless resolved |

Use `useColorScheme()` only when a non-CSS consumer needs the mode as a value (a Monaco or code-block theme, an image
choice). Resolve `system` to the OS value with `mode === 'system' ? systemMode : mode`, as
`packages/configure-applications/src/components/create-application/Preview.tsx` does.

Literal colors are allowed only where they are the content: vendor logos in `components/src/icons`, color pickers
and preset swatches in the theme editor, and previews of a customer's own theme.

**Gate and `@thunderid/design`**: sign-in, sign-up, and recovery screens are rendered from flow definitions through
`@thunderid/design` and are styled by the customer's configured theme. Never hardcode a color, font, logo, or product
name there, since it overrides the customer's branding. Read the product name from `config.brand.product_name` and the
theme from the design context.

## 6. Icons, Text, and Feedback

- **Icons** come from `@wso2/oxygen-ui-icons-react` and take a numeric `size`. Match the sibling components. 16 is the
  common inline and button size, and page-level primary actions use 18. For a product category, use the icon in the
  category table below.
- **Every icon-only button** has an `aria-label` and a `Tooltip`, both translated (section 7).
- **Text** is sentence case ("Add group", "Sign out"), goes through `t()` with a fallback string (frontend/AGENTS.md),
  and has no em dashes or double hyphens. Nothing a user can see or hear is hardcoded, including `aria-label`s,
  tooltips, placeholders, column headers, and toast messages. Section 9 has a command that finds hardcoded
  text. Reuse the `common:` keys in `packages/i18n/src/locales/en-US.ts`
  (`actions.cancel`, `actions.delete`, `actions.save`, `actions.copy`, `actions.copied`, `status.saving`,
  `status.deleting`, and others) instead of adding feature-local copies.
- **Loading**: `PageLoadingAnimation` for a page or tab body, `CircularProgress` from Oxygen for inline spinners, and a
  pending label plus `disabled` on the button that started a mutation. Do not write a custom loader component.
  `Skeleton` appears only on a few surfaces (connections, design, CORS settings, onboarding home). Use it only where
  the surrounding surface already does, and default to spinners elsewhere.
- **Feedback**: inline `Alert` for mutation errors in forms and dialogs, `useToast` from `@thunderid/contexts` for
  success confirmations, `QueryErrorNotice` for read failures (frontend/AGENTS.md, Error Display).

### Category Icons

The WSO2 cross-product UI unification guidelines fix one icon per product category, so the same concept looks the
same in every WSO2 console. When a sidebar item, tab, card, or page header represents one of these categories, use its
icon:

| Category | Icon | Category | Icon |
|---|---|---|---|
| Overview | `LayoutDashboard` | Publish | `Megaphone` |
| Projects | `BoxIcon` (see below) | Observability | `Activity` |
| Develop | `Code` | Logs | `Logs` |
| Policies | `ShieldCheck` | Metrics | `Gauge` |
| Definition | `Braces` | Insights | `ChartNoAxesCombined` |
| Documents | `FileText` | Environments | `Layers` |
| Build | `Hammer` | Gateways | `Network` |
| Deploy | `Rocket` | Pipelines | `Workflow` |
| Test | `FlaskConical` | Portals | `PanelsTopLeft` |

- All of these are `lucide-react` icons, which `@wso2/oxygen-ui-icons-react` re-exports. Import them from
  `@wso2/oxygen-ui-icons-react`, never from `lucide-react`.
- The Projects icon is `Box`, which clashes with Oxygen's `Box` layout component. Import it as `BoxIcon`, the alias the
  icon package also exports, as `configure-flows` `ResourcePanel` does.
- ThunderID already matches the table for Gateways (`Network`) and the control plane's Deploy button (`Rocket`).
- Three of these icons currently stand for other things in the Console sidebar: `Layers` for Connections, `Workflow`
  for Flows, and `ShieldCheck` for Roles and Presentations. The guidelines do not say whether those icons are reserved
  for their categories. Do not change the existing sidebar icons in a feature PR, and raise the question with the
  unification team instead.

## 7. Accessibility

Oxygen targets WCAG 2.1 AA and is audited against it (`ACCESSIBILITY.md`, section 1a). Oxygen components are accessible
out of the box only when used as their stories show, and most failures come from how a screen composes them. Read the
component's Accessibility block in Storybook before using it, then apply these rules:

- **Name every control.** An icon-only `IconButton` needs a translated `aria-label`. A `Tooltip` with a string title
  does name its child, but set `aria-label` explicitly anyway, because the name is lost when the tooltip is removed or
  switched to `describeChild`. A standalone `Checkbox`, `Switch`, or `Slider` with no visible label gets one through
  `slotProps`. A `Select` links `InputLabel id` to `Select labelId`. A text field gets a `FormLabel htmlFor` matching
  the field's `id`, as `EditGeneralSettings` does. Every `CircularProgress` gets an `aria-label`
  (`PageLoadingAnimation` already has one).
- **Hide decorative icons** next to visible text with `aria-hidden`.
- **Name icon-only columns.** A table column that holds only row-action buttons still needs a translated header name.
  For `DataGrid`, set the column's `headerName` from the feature's `<namespace>:listing.columns.actions` key, as every
  list does (for example `groups:listing.columns.actions`). For a plain table, put an `aria-label` on the header cell.
- **Interactive means a real control.** Do not put `onClick` on a plain `Box`, `Typography`, `Stack`, or `Avatar`. Use
  `Button`, `IconButton`, `ButtonBase`, `Link`, or a `ListItemButton`, so it is focusable and works with Enter and
  Space. A `Box component="button"` works too, but prefer `ButtonBase`. Do not nest a button or link inside another
  clickable control. Oxygen tracks that bug in `Form.CardButton` ([#562](https://github.com/wso2/oxygen-ui/issues/562)),
  so do not add controls inside one.
- **Keep focus visible.** Never set `outline: 'none'` or remove `:focus-visible` styles. Dialogs, menus, drawers, and
  tabs get focus trapping and restoration from MUI, so do not replace them with hand-built popovers.
- **Do not convey state by color alone.** Pair a color (error, success, enabled) with text or an icon. Body-size text
  in the brand primary color fails 4.5:1 contrast, both in Oxygen's orange themes
  ([#558](https://github.com/wso2/oxygen-ui/issues/558)) and in ThunderID's default `#3688FF` (about 3.4:1 on white).
  Keep primary for buttons, links, and accents, not for running text.
- **Scrollable regions** with a fixed height get `tabIndex={0}` and an `aria-label` so keyboard users can scroll them.
- **Motion**: Oxygen's baseline collapses every CSS animation and transition under `prefers-reduced-motion: reduce`,
  including custom `keyframes`, so CSS motion needs nothing extra. JavaScript-driven motion (canvas,
  `requestAnimationFrame`, animated scrolling) does not inherit it and must check
  `useMediaQuery('(prefers-reduced-motion: reduce)')`.
- **Announce async results** that appear away from focus (a toast, an inline save error) through the existing
  `useToast` and `Alert` paths, which carry the right roles. Do not invent a new live region.
- **Test by role and name.** Query with `getByRole('button', {name: ...})` in tests, which fails when an accessible
  name is missing.

## 8. Theming and Branding

- Never wrap a feature in its own `OxygenUIThemeProvider` or override the theme from a feature package. The host app
  owns the theme.
- Never hardcode the product name, logo, or theme. They come from the runtime brand config (`useConfig()` from
  `@thunderid/contexts`) so a deployment can rebrand without code changes.
- Component-wide visual changes belong in Oxygen, not in a per-screen `sx` override. If Oxygen's default looks wrong
  everywhere, raise it against Oxygen UI rather than patching each usage.

## 9. Before You Finish

Run through this on the diff:

- [ ] Searched `@thunderid/components` and Oxygen's docs for anything you built new (`rg -n "ComponentName"
      frontend/packages frontend/apps`), and reused what exists.
- [ ] Read the Storybook docs (variant guidance and Accessibility block) for every Oxygen component you added or
      reconfigured.
- [ ] No `@mui/*`, `lucide-react`, or new `@emotion/*` imports.
- [ ] No hex, `rgb(a)`, or `px` literals in `sx`, no `style={{}}`, no `fontWeight`/`fontSize` faking a variant.
- [ ] No new `navigator.clipboard` handler, loader, delete-dialog shell, or wizard frame duplicating a shared one.
- [ ] Every new page has `DocumentTitle`, a loading state, and a `QueryErrorNotice` for its primary query.
- [ ] Text is sentence case, translated with fallbacks, and reuses `common:` keys.
- [ ] No hardcoded user-facing text: every visible string, `aria-label`, `title`, `placeholder`, tooltip, column
      `headerName`, and toast message goes through `t()` with its key in the `i18n` locale files. Run this from the
      repo root to list candidates in the files you touched, including new untracked ones:

      ```bash
      remote=$(git remote -v | awk '/thunder-id\/thunderid(\.git)? \(fetch\)/ {print $1; exit}')
      base=$(git merge-base HEAD "$remote/main") &&
      { git diff --name-only --diff-filter=d "$base"; git ls-files --others --exclude-standard; } \
        | grep -E '^frontend/.*\.tsx?$' | grep -v -E '__tests__|\.test\.|\.stories\.' \
        | xargs grep -Hn -E \
            -e '(aria-label|title|placeholder|label|helperText|alt|tooltip)="[^"]*[A-Za-z]' \
            -e '>[[:space:]]*[A-Za-z][^<>{}=]*[A-Za-z.!?][[:space:]]*</[A-Za-z]' \
            -e "(headerName|label|title|placeholder|helperText|description|tooltip|message):[[:space:]]*'[A-Z][a-z]" \
            -e "showToast\([[:space:]]*'[A-Za-z]"
      ```

      The base is the merge base with `main` on the `thunder-id/thunderid` remote, as in `pr-preflight` Step 1, so a
      fork's stale `origin` does not pull in unrelated files. For a change that targets a release branch, use that
      branch instead of `main`. Deleted files are left out, since there is nothing to search in them. If it prints
      `Not a valid object name /main`, add and fetch the remote first (`git remote add upstream
      https://github.com/thunder-id/thunderid.git && git fetch upstream main`).

      No output means no candidates. It flags string props, JSX text, column and option labels, and toast literals.
      It also matches `<Trans defaults="...">` fallbacks and JSDoc examples, which are fine. It reports every line in a
      touched file, so fix the hits on lines you wrote and list older ones as a follow-up rather than rewriting them.
      It cannot see strings built in plain variables, so still read your diff for any text a user will see.
- [ ] Every control has an accessible name, nothing clickable is a `Box` or `Typography`, and focus styles are intact.
- [ ] Walked the screen with the keyboard only: Tab reaches every control in order, Enter or Space activates it, and
      Escape closes dialogs and menus.
- [ ] Checked the screen in light and dark mode. Use [.agent/skills/console/SKILL.md](../console/SKILL.md) for browser
      verification when it is needed.
- [ ] Ran `pnpm test`, `pnpm lint`, and `pnpm prettier --check` scoped to the touched packages. Update visual
      snapshots under `__screenshots__` only when the visual change is intended.
