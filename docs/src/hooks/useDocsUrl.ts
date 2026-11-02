// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  useActiveVersion,
  useDocsPreferredVersion,
  useLatestVersion,
} from '@docusaurus/plugin-content-docs/client';

/**
 * Returns a helper that rewrites a hardcoded `/docs/next/...` link to the base path
 * of the doc version the reader is currently in, so links follow the reader instead
 * of always pointing at the future/unreleased ("Next") docs.
 *
 * Custom pages (`src/pages/`) and standalone components live outside the docs
 * plugin's version routing, so they tend to hardcode `/docs/next/...`. The target
 * version is resolved in priority order:
 *
 *   1. The active version, when the component renders on a versioned doc route
 *      (e.g. the footer on a `/docs/next/` page → `/docs/next`; on `/docs/v1.0.x/`
 *      → `/docs/v1.0.x`).
 *   2. The preferred version, when there is no active version (a version-less
 *      custom page such as `/sdks-and-tools/`) but the reader has selected one via the
 *      version dropdown — that choice is persisted and carries across pages.
 *   3. The latest published version (`lastVersion` in docusaurus.config.ts), as the
 *      default when neither applies (a first visit to a custom page).
 *
 *   const docsUrl = useDocsUrl();
 *   <Link to={docsUrl('/docs/next/getting-started/get-thunderid')} />
 *
 * A path that does not start with `/docs/next` is returned unchanged, so it is safe
 * to apply to every link in a list regardless of whether each one is versioned.
 */
export function useDocsUrl(): (href: string) => string {
  const active = useActiveVersion(undefined);
  const {preferredVersion} = useDocsPreferredVersion(undefined);
  const latest = useLatestVersion(undefined);
  const base = (active ?? preferredVersion ?? latest).path;
  return (href: string): string => href.replace(/^\/docs\/next(?=\/|$)/, base);
}
