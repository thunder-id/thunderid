// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {resolve, dirname} from 'path';
import {fileURLToPath} from 'url';
import babel from '@rolldown/plugin-babel';
import {linkWorkspaceSource} from '@thunderid/build-plugins/vite';
import basicSsl from '@vitejs/plugin-basic-ssl';
import react, {reactCompilerPreset} from '@vitejs/plugin-react';
import svgr from 'vite-plugin-svgr';
import {defineConfig} from 'vitest/config';

const currentDir = dirname(fileURLToPath(import.meta.url));
const PORT = process.env.PORT ? Number(process.env.PORT) : 5192;
const HOST = process.env.HOST ?? 'localhost';
const BASE_URL = process.env.BASE_URL ?? '/admin';

// Dev backend URL, from THUNDERID_DEV_SERVER_URL (default https://localhost:8090). Injected into
// __DEV_SERVER_URL__ only for the dev server; production builds receive an empty string.
const DEV_SERVER_URL = process.env.THUNDERID_DEV_SERVER_URL?.trim();

// Dev gate app URL, from THUNDERID_DEV_GATE_URL (default https://localhost:5190). Injected into
// __DEV_GATE_URL__ only for the dev server; production builds receive an empty string.
const DEV_GATE_URL = process.env.THUNDERID_DEV_GATE_URL?.trim();

// https://vite.dev/config/
export default defineConfig(({command}) => ({
  base: BASE_URL,
  define: {
    VERSION: JSON.stringify('v0.0.0'),
    __DEV_SERVER_URL__: JSON.stringify(
      command === 'serve'
        ? DEV_SERVER_URL && DEV_SERVER_URL.length > 0
          ? DEV_SERVER_URL
          : 'https://localhost:8090'
        : '',
    ),
    __DEV_GATE_URL__: JSON.stringify(
      command === 'serve' ? (DEV_GATE_URL && DEV_GATE_URL.length > 0 ? DEV_GATE_URL : 'https://localhost:5190') : '',
    ),
  },
  plugins: [
    linkWorkspaceSource(),
    basicSsl(),
    svgr(),
    react(),
    babel({
      // Only compile this repository's own source. Prebuilt packages are already compiled, and a
      // linked one (outside node_modules) has hook names the compiler no longer recognizes.
      exclude: [/[\\/]node_modules[\\/]/, /[\\/]dist[\\/]/],
      presets: [reactCompilerPreset()],
    }),
  ],
  server: {
    port: PORT,
    host: HOST,
  },
  resolve: {
    alias: {
      '@': resolve(currentDir, 'src'),
      // Force using the same React instance to avoid "Invalid hook call" errors
      // when using linked packages
      react: resolve(currentDir, './node_modules/react'),
      'react-dom': resolve(currentDir, './node_modules/react-dom'),
      'react-router': resolve(currentDir, './node_modules/react-router'),
    },
    conditions: ['browser', 'module', 'import', 'default'],
  },
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: ['@thunderid/test-utils/setup'],
    reporters: process.env.CI ? ['dot'] : ['default'],
    restoreMocks: true,
    testTimeout: 30000,
    server: {
      deps: {
        inline: [
          '@thunderid/browser',
          '@thunderid/react',
          '@wso2/oxygen-ui',
          '@wso2/oxygen-ui-icons-react',
          '@mui/x-data-grid',
        ],
      },
    },
    coverage: {
      provider: 'istanbul',
      reporter: ['text', 'json', 'html'],
      exclude: ['node_modules/', 'dist/', 'public/', 'coverage/', '**/*.d.ts', '**/*.config.*', '**/*.test.tsx'],
    },
  },
}));
