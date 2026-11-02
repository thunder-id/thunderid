// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {buildRegistry, type SampleBundle} from '@thunderid/configure-onboarding';

// The quick start bundles the build stages into ./sample-bundles, inlined at build time.
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
