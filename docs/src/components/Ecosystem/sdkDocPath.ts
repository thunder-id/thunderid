// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** Directory that holds both the registry and the SDK & tool docs. */
const REGISTRY_SEGMENT = 'sdks-and-tools';

/**
 * The registry entry a docs URL belongs to, if any.
 *
 * Pages under `sdks-and-tools/<id>/` are leaves of that entry's detail page
 * rather than nodes in a docs tree, so they trade the sidebar for a link back
 * to it. Returns undefined for the entry's own overview, which is a docs page
 * in its own right, and for anything outside the registry.
 */
export default function sdkIdFromPath(pathname: string): string | undefined {
  const match = new RegExp(`/${REGISTRY_SEGMENT}/([^/]+)/(.+)$`).exec(pathname);
  if (!match) return undefined;
  const [, id, rest] = match;
  return rest.replace(/\/$/, '') === 'overview' ? undefined : id;
}
