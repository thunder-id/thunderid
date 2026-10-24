// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {resolve} from 'path';
import {playwright} from '@vitest/browser-playwright';
import {defineConfig} from 'vitest/config';

export default defineConfig({
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
      '@/api': resolve(__dirname, 'src', 'api'),
      '@/components': resolve(__dirname, 'src', 'components'),
      '@/config': resolve(__dirname, 'src', 'config'),
      '@/constants': resolve(__dirname, 'src', 'constants'),
      '@/contexts': resolve(__dirname, 'src', 'contexts'),
      '@/data': resolve(__dirname, 'src', 'data'),
      '@/hooks': resolve(__dirname, 'src', 'hooks'),
      '@/models': resolve(__dirname, 'src', 'models'),
      '@/pages': resolve(__dirname, 'src', 'pages'),
      '@/utils': resolve(__dirname, 'src', 'utils'),
    },
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
