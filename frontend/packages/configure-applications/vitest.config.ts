// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {playwright} from '@vitest/browser-playwright';
import {defineConfig} from 'vitest/config';

export default defineConfig({
  // This package pulls in an unusually large and varied set of shared/feature packages across its
  // many test files; esbuild's dependency scanner doesn't reliably discover all of their transitive
  // imports (e.g. @thunderid/components -> @thunderid/hooks, @thunderid/react's own re-exports) in
  // one cold-start pass, which surfaces as "does not provide an export" errors on tests that happen
  // to be the first to touch a given dependency. Listing them explicitly avoids that scan gap.
  optimizeDeps: {
    include: [
      '@thunderid/components',
      '@thunderid/configure-connections',
      '@thunderid/configure-organization-units',
      '@thunderid/configure-settings',
      '@thunderid/configure-user-types',
      '@thunderid/contexts',
      '@thunderid/design',
      '@thunderid/hooks',
      '@thunderid/logger',
      '@thunderid/logger/react',
      '@thunderid/react',
      '@thunderid/utils',
    ],
  },
  test: {
    coverage: {
      provider: 'istanbul',
      reporter: [['lcov', {projectRoot: '../../..'}], 'text-summary'],
      include: ['src/**/*.{ts,tsx}'],
      exclude: [
        '**/*.d.ts',
        '**/*.config.*',
        '**/__tests__/**',
        '**/test/**',
        '**/*.{test,spec}.{ts,tsx}',
        '**/test-setup.{ts,tsx}',
        '**/index.ts',
      ],
    },
    browser: {
      enabled: true,
      headless: true,
      instances: [{browser: 'chromium'}],
      provider: playwright(),
    },
    setupFiles: ['@thunderid/test-utils/setup'],
  },
});
