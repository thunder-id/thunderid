// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The configuration files a quick start bundle ships: a declarative export and the variables it
 * expects, as the host application staged them.
 *
 * @public
 */
export interface SampleBundleConfigs {
  declarative?: string;
  env?: string;
}

/**
 * A quick start bundle the welcome pages can import.
 *
 * @public
 */
export interface SampleBundle {
  configs: SampleBundleConfigs;
}

const BUNDLE_ROOT = './sample-bundles/';

/**
 * Names the bundle a staged file belongs to: its directory under the bundle root, which may be
 * nested for grouping (e.g. `wayfinder/redirect-based`).
 *
 * @public
 */
export const bundleKeyFromPath = (path: string): string => {
  const tail = path.slice(BUNDLE_ROOT.length);
  const lastSlash = tail.lastIndexOf('/');
  return lastSlash === -1 ? tail : tail.slice(0, lastSlash);
};

/**
 * Builds the bundle registry from the staged files a host application globbed, keyed by bundle.
 *
 * @public
 */
export const buildRegistry = (
  yamlModules: Record<string, string>,
  envModules: Record<string, string>,
): Record<string, SampleBundle> => {
  const registry: Record<string, SampleBundle> = {};

  const ensure = (key: string): SampleBundle => {
    if (!registry[key]) {
      registry[key] = {configs: {}};
    }
    return registry[key];
  };

  for (const [path, content] of Object.entries(yamlModules)) {
    ensure(bundleKeyFromPath(path)).configs.declarative = content;
  }

  for (const [path, content] of Object.entries(envModules)) {
    ensure(bundleKeyFromPath(path)).configs.env = content;
  }

  return registry;
};
