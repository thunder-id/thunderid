// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  useActiveVersion,
  useDocsPreferredVersion,
  useDocsVersion,
  useLatestVersion,
} from '@docusaurus/plugin-content-docs/client';
import {usePluginData} from '@docusaurus/useGlobalData';
import type {EcosystemEntry, EcosystemGlobalData} from '@site/src/types/ecosystem';

/** Docs version name Docusaurus uses for the unreleased `content/` tree. */
const CURRENT_VERSION = 'current';

/**
 * Entries for one named docs version.
 *
 * Every version carries its own snapshot of the registry, because
 * `docusaurus docs:version` copies every `sdk.yaml` under `content/sdks` along
 * with the pages.
 *
 * A version cut before `sdk.yaml` existed has a `sdks` directory but no entries
 * in it, so an empty result falls back to `current` rather than rendering an
 * empty listing. That is why the check is on length and not just presence.
 */
function useEcosystemByVersion(name: string): EcosystemEntry[] {
  const {byVersion} = usePluginData('ecosystem-plugin') as EcosystemGlobalData;
  const entries = byVersion[name];
  if (entries && entries.length > 0) return entries;
  return byVersion[CURRENT_VERSION] ?? [];
}

/**
 * Reads the SDK & tools registry from a **version-less** page such as `/sdks`.
 *
 * Resolves the version the same way `useDocsUrl()` resolves link targets, so
 * the listing follows the reader: the active version on a versioned route, then
 * the version they picked in the dropdown, then the latest published one.
 *
 * Inside a doc page use `useDocEcosystem()` instead — that context has its own
 * authoritative version and should not go through this fallback chain.
 */
export function useEcosystem(): EcosystemEntry[] {
  const active = useActiveVersion(undefined);
  const {preferredVersion} = useDocsPreferredVersion(undefined);
  const latest = useLatestVersion(undefined);
  return useEcosystemByVersion((active ?? preferredVersion ?? latest).name);
}

/**
 * Reads the registry from a component rendered **inside** a doc page, using
 * that page's own version rather than the site's active/preferred/latest state.
 *
 * Only valid below the docs version context; it throws elsewhere, the same way
 * `useDocsVersion()` does.
 */
export function useDocEcosystem(): EcosystemEntry[] {
  return useEcosystemByVersion(useDocsVersion().version);
}

/** Looks up one entry by id, from inside a doc page. */
export function useDocEcosystemEntry(id: string): EcosystemEntry | undefined {
  return useDocEcosystem().find((entry) => entry.id === id);
}
