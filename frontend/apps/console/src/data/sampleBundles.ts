// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

export interface SampleBundleConfigs {
  declarative?: string;
  env?: string;
}

export interface SampleBundle {
  configs: SampleBundleConfigs;
}

const BUNDLE_ROOT = './sample-bundles/';

export const bundleKeyFromPath = (path: string): string => {
  const tail = path.slice(BUNDLE_ROOT.length);
  const lastSlash = tail.lastIndexOf('/');
  return lastSlash === -1 ? tail : tail.slice(0, lastSlash);
};

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

const yamlModules: Record<string, string> = import.meta.glob('./sample-bundles/**/*.{yaml,yml}', {
  query: '?raw',
  import: 'default',
  eager: true,
});

const envModules: Record<string, string> = import.meta.glob('./sample-bundles/**/*.env', {
  query: '?raw',
  import: 'default',
  eager: true,
});

export const SAMPLE_BUNDLES: Record<string, SampleBundle> = buildRegistry(yamlModules, envModules);
