// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Schema for the SDKs & tools registry.
 *
 * Every SDK, integration, and agent plugin is described by a single
 * `content/sdks-and-tools/<id>/entry.yaml` file. Those files are the source of
 * truth for three surfaces:
 *
 *   1. the `/sdks` listing page (card fields: name, icon, kind, description)
 *   2. the detail page, wherever `docs.overview` points
 *   3. the API reference, wherever `docs.apiReference` points
 *
 * An entry's pages usually sit beside its `entry.yaml`, but not always: an
 * agent plugin's live under `content/working-with-ai/`, and an unreleased entry
 * has none at all. `docs:` is what says where they are, so nothing here depends
 * on a page existing at a path derived from the id.
 *
 * The registry sits inside `content/` so `docusaurus docs:version` snapshots it
 * with the pages, letting a released version keep its own copy of both while
 * `next` moves on.
 *
 * `plugins/ecosystemPlugin.ts` parses and validates them at build time and
 * exposes the result as plugin global data. Nothing here is parsed at runtime.
 */

/**
 * Second-level grouping: the platform an entry targets.
 *
 * Orthogonal to `type`. "Integration" is deliberately absent, since that is a
 * kind of thing rather than a platform and belongs on the other axis.
 */
export type EcosystemCategory = 'agent' | 'spa' | 'fullstack' | 'backend' | 'mobile';

/** What an entry is. Combined with `origin` to form the top-level grouping. */
export type EcosystemType = 'sdk' | 'plugin' | 'integration' | 'guide';

/** Who maintains it. Drives the Official / Community badge on a card. */
export type EcosystemOrigin = 'official' | 'community';

/** Availability. `soon` entries render in the "Coming soon" grid. */
export type EcosystemStatus = 'ga' | 'beta' | 'soon';

/** Registry a package is published to. Drives version lookup and install tabs. */
export type EcosystemPackageManager =
  | 'npm'
  | 'go'
  | 'pip'
  | 'gradle'
  | 'maven'
  | 'pod'
  | 'spm'
  | 'pub'
  | 'plugin';

/** Colour role for a badge or pill. Mapped to real colours by the renderer. */
export type EcosystemTone = 'success' | 'danger' | 'warning' | 'accent' | 'primary' | 'purple' | 'neutral';

/** How a table cell is typeset. */
export type EcosystemCellStyle = 'text' | 'code' | 'accent' | 'muted';

/** Named icon for a hero meta item. Kept to a small closed set on purpose. */
export type EcosystemMetaIcon = 'package' | 'licence' | 'clock' | 'download' | 'people';

/** Named icon for a guide card. A closed set, so a typo fails the build. */
export type EcosystemGuideIcon =
  | 'shield'
  | 'terminal'
  | 'server'
  | 'pen'
  | 'building'
  | 'exchange'
  | 'key'
  | 'book';

export interface EcosystemPackage {
  /** Package coordinate as a user would type it, e.g. `@thunderid/react`. */
  name: string;
  manager?: EcosystemPackageManager;
  /** Registry page. Omit to derive from `manager` + `name` where possible. */
  url?: string;
  /** Show the live npm weekly-download count on the detail hero. Defaults to `false` (hidden). */
  showDownloadCount?: boolean;
}

/**
 * Routes into the docs for this entry. All are site-absolute `/docs/next/...`
 * paths, rewritten to the reader's version at render time.
 *
 * `overview` is where the detail page lives and is what a listing card links
 * to; an entry with `status: soon` has none. The `api` and `quickstart`
 * sections read their targets from here rather than repeating them.
 */
export interface EcosystemDocs {
  overview?: string;
  apiReference?: string;
  quickstart?: string;
}

/** Links out of the docs: source, registry, and upstream project pages. */
export interface EcosystemLinks {
  github?: string;
  package?: string;
  changelog?: string;
  samples?: string;
  /** Upstream docs for an entry we do not maintain (e.g. Spring Security). */
  external?: string;
}

export interface EcosystemCta {
  label: string;
  href: string;
  variant?: 'primary' | 'secondary';
  external?: boolean;
}

export interface EcosystemMetaItem {
  icon: EcosystemMetaIcon;
  label: string;
  /** Render the label in monospace (package coordinates, versions). */
  mono?: boolean;
}

export interface EcosystemCode {
  /** Filename or context shown in the code block's title bar. */
  file?: string;
  /** Language label shown on the right of the title bar. */
  lang?: string;
  content: string;
}

export interface EcosystemHeroTab {
  label: string;
  code: EcosystemCode;
}

/**
 * The card to the right of the hero. `install` renders tabs (package managers
 * or build tools); `snippet` renders a single fixed block.
 */
export interface EcosystemHeroAside {
  type: 'install' | 'snippet';
  title?: string;
  tabs?: EcosystemHeroTab[];
  code?: EcosystemCode;
  /** Small highlighted line under the block. */
  footnote?: string;
}

export interface EcosystemHero {
  /** Long-form description. Falls back to the card `description`. */
  description?: string;
  /** Extra pills beside the title, on top of the origin and status badges. */
  badges?: {label: string; tone?: EcosystemTone}[];
  meta?: EcosystemMetaItem[];
  ctas?: EcosystemCta[];
  aside?: EcosystemHeroAside;
}

export interface EcosystemStep {
  title: string;
  description?: string;
  code?: EcosystemCode;
  /**
   * Package-manager (or similarly interchangeable) variants of this step's command,
   * shown as tabs inside the code block's own bar — same shape as
   * `EcosystemHeroAside`'s `install` tabs. Mutually exclusive with `code`; set one
   * or the other, not both.
   */
  tabs?: EcosystemHeroTab[];
  /** Click path through a GUI, rendered as a numbered list. */
  uiPath?: string[];
  note?: string;
}

/** A group of steps behind one tab, e.g. "Claude Code" vs "Claude Desktop". */
export interface EcosystemStepGroup {
  label: string;
  blurb?: string;
  steps: EcosystemStep[];
}

interface EcosystemSectionBase {
  /** Anchor and TOC key. Must be unique within an entry. */
  id: string;
  title: string;
  intro?: string;
  /** Small label beside the heading, e.g. "~8 min". */
  meta?: string;
}

/**
 * Numbered walkthrough. Either `steps` directly, or `tabs` when the same task
 * has several surfaces (CLI vs desktop vs web).
 */
export interface EcosystemStepsSection extends EcosystemSectionBase {
  type: 'steps';
  steps?: EcosystemStep[];
  tabs?: EcosystemStepGroup[];
}

/**
 * Row table. Used for compatibility matrices, requirements, and claim mapping.
 * `columns` fixes the column count; every row must supply that many cells.
 */
export interface EcosystemTableSection extends EcosystemSectionBase {
  type: 'table';
  columns: {label?: string; style?: EcosystemCellStyle}[];
  /** Render the column labels as a header row. Defaults to false. */
  showHeader?: boolean;
  rows: {cells: string[]; badge?: {label: string; tone?: EcosystemTone}}[];
}

/** Slash-command or CLI reference, grouped by a coloured tag. */
export interface EcosystemCommandsSection extends EcosystemSectionBase {
  type: 'commands';
  /** Tag label to tone, e.g. `{plugin: 'accent', manage: 'neutral'}`. */
  groupTones?: Record<string, EcosystemTone>;
  items: {group?: string; command: string; description: string}[];
}

/** Grid of outbound links, e.g. "Beyond this guide". */
export interface EcosystemLinkGridSection extends EcosystemSectionBase {
  type: 'linkGrid';
  items: {label: string; href: string; external?: boolean}[];
}

/**
 * One guide: its own page under `/sdks/<id>/guides/<guide id>`.
 *
 * A guide is built from the same sections an entry is, so it renders through
 * the same components and is indistinguishable from a section on the entry's
 * own page. Keeping guides as data rather than MDX is what makes that true.
 */
export interface EcosystemGuide {
  id: string;
  title: string;
  /** Card copy on the entry's page. */
  description?: string;
  icon?: EcosystemGuideIcon;
  /** Lead paragraph at the top of the guide's own page. */
  intro?: string;
  /** Filled in at build time from the guide's own content. */
  minutes?: number;
  sections: EcosystemSection[];
}

/**
 * Grid of guide cards pointing into the docs.
 *
 * `minutes` is filled in at build time from the target page's word count, so a
 * guide's reading time cannot drift from the guide. An authored value wins,
 * for a target the build cannot resolve.
 */
export interface EcosystemGuidesSection extends EcosystemSectionBase {
  type: 'guides';
  /**
   * Cards for the entry's own guides, in this order. Omit to show every guide
   * the entry declares. Each id must match one in `guides`.
   */
  items?: string[];
}

/**
 * Single prominent card linking to the canonical quickstart at
 * `docs.quickstart`, which the entry must therefore set.
 */
export interface EcosystemQuickstartSection extends EcosystemSectionBase {
  type: 'quickstart';
  label: string;
  /** Breadcrumb-style trail shown on the card, e.g. "Getting started / React". */
  path: string;
  minutes?: number;
  /** Copy below the card, explaining where the quickstart is maintained. */
  outro?: string;
}

/** One export shown in the API explorer. */
export interface EcosystemApiEntry {
  /** Heading it sits under in the explorer's left nav, e.g. "Hooks". */
  group: string;
  /** Badge beside the name, e.g. "hook", "component", "type". */
  kind: string;
  name: string;
  description: string;
  /**
   * Declaration as it appears in the package's types.
   *
   * Optional because a reference page often documents its surface as a table
   * of props or return values rather than a declaration. Where that is the
   * case, `rows` carries it and no signature is invented to stand in.
   */
  signature?: string;
  /** Heading over `rows`, e.g. "Props", "Returns", "Fields". */
  rowsLabel?: string;
  rows?: {name: string; type: string; description?: string; tag?: string}[];
  example?: string;
}

/**
 * Two-pane explorer over a curated handful of the package's exports.
 *
 * Deliberately a selection rather than the whole surface: the complete
 * reference is generated from the package's type declarations and lives at
 * `docs.apiReference`, linked from this section's header when it exists.
 */
export interface EcosystemApiSection extends EcosystemSectionBase {
  type: 'api';
  exports: EcosystemApiEntry[];
  /** Badge colour per `kind` value, e.g. `{hook: 'accent', type: 'warning'}`. */
  kindTones?: Record<string, EcosystemTone>;
}

/**
 * Closing support block. `official` is a single "backed by the team" panel;
 * `split` is the two-panel "our guide vs their library" treatment used for
 * integrations we do not own.
 */
export interface EcosystemSupportSection extends EcosystemSectionBase {
  type: 'support';
  variant: 'official' | 'split';
  eyebrow?: string;
  body?: string;
  ctas?: EcosystemCta[];
  /** `split` only. Exactly two panels. */
  panels?: {title: string; body: string; ctas?: EcosystemCta[]}[];
}

export type EcosystemSection =
  | EcosystemStepsSection
  | EcosystemTableSection
  | EcosystemCommandsSection
  | EcosystemLinkGridSection
  | EcosystemGuidesSection
  | EcosystemQuickstartSection
  | EcosystemApiSection
  | EcosystemSupportSection;

export interface EcosystemRail {
  /** Heading over the stats block, e.g. "Package", "Plugin", "Ownership". */
  statsTitle?: string;
  stats?: {label: string; value: string}[];
  relatedTitle?: string;
  /** Entry ids. Validated to exist at build time. */
  related?: string[];
}

/**
 * Where the API reference data comes from.
 *
 * `typedoc` and `symbolgraph` are generated into `api.json` by a build script;
 * `manual` means a hand-authored `api.yaml` sitting beside `sdk.yaml`. The
 * renderer only ever sees the normalised output, so swapping a source is a
 * build change with no UI work.
 */
export interface EcosystemApiReference {
  source: 'typedoc' | 'symbolgraph' | 'manual';
  /** Path relative to the entry directory. Defaults per source. */
  file?: string;
}

/** One SDK, integration, or agent plugin. Parsed from `entry.yaml`. */
export interface EcosystemEntry {
  id: string;
  name: string;
  /** Key into `src/components/Ecosystem/iconRegistry.ts`. */
  icon: string;
  category: EcosystemCategory;
  type: EcosystemType;
  origin: EcosystemOrigin;
  status: EcosystemStatus;
  /** Brand colour for this entry's accents. Defaults to the ThunderID blue. */
  accent?: string;
  package: EcosystemPackage;
  /** Handle credited on community entries, without the leading `@`. */
  author?: string;
  /** Card copy on the listing page. Supports `{{ProductName}}`. */
  description: string;
  /** Pinned version label. Omit to look it up from the registry at runtime. */
  version?: string;
  docs?: EcosystemDocs;
  links?: EcosystemLinks;
  hero?: EcosystemHero;
  sections?: EcosystemSection[];
  /** Guides, each rendered on its own page. */
  guides?: EcosystemGuide[];
  rail?: EcosystemRail;
  apiReference?: EcosystemApiReference;
}

/**
 * Shape of the plugin's global data, read through `useEcosystem()`.
 * Keyed by docs version name (`current`, `v1.0.x`, ...).
 */
export interface EcosystemGlobalData {
  byVersion: Record<string, EcosystemEntry[]>;
}
