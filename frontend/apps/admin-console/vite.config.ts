// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {resolve, dirname} from 'path';
import {fileURLToPath} from 'url';
import {codecovVitePlugin} from '@codecov/vite-plugin';
import babel from '@rolldown/plugin-babel';
import {linkWorkspaceSource, prismjsInjectCore} from '@thunderid/build-plugins/vite';
import basicSsl from '@vitejs/plugin-basic-ssl';
import react, {reactCompilerPreset} from '@vitejs/plugin-react';
import {visualizer} from 'rollup-plugin-visualizer';
import svgr from 'vite-plugin-svgr';
import {defineConfig} from 'vitest/config';

const currentDir = dirname(fileURLToPath(import.meta.url));
const PORT = process.env.PORT ? Number(process.env.PORT) : 5192;
const HOST = process.env.HOST ?? 'localhost';
// Served from the Data Plane's existing `/console` mount (see build.sh's
// prepare_frontend_for_packaging, which copies this app's build output into that slot) rather
// than its own `/admin-console` path, so the already-registered CONSOLE OAuth2 client's redirect
// URI keeps working unchanged.
const BASE_URL = process.env.BASE_URL ?? '/console';
const ANALYZER_ENABLED = process.env.ANALYZE === 'true';
const BUNDLE_ANALYSIS_ENABLED = process.env.CODECOV_BUNDLE_UPLOAD === 'true';

// Dev backend URL, from THUNDERID_DEV_SERVER_URL (default https://localhost:8090). Injected into
// __DEV_SERVER_URL__ only for the dev server; production builds receive an empty string.
const DEV_SERVER_URL = process.env.THUNDERID_DEV_SERVER_URL?.trim();

// Dev gate app URL, from THUNDERID_DEV_GATE_URL (default https://localhost:5190). Injected into
// __DEV_GATE_URL__ only for the dev server; production builds receive an empty string. Needed
// because sign-in redirects through Gate, same as Console.
const DEV_GATE_URL = process.env.THUNDERID_DEV_GATE_URL?.trim();

// Dev OAuth2 client ID, from THUNDERID_DEV_CLIENT_ID (unset by default, which keeps the client_id in
// config.js). Injected into __DEV_CLIENT_ID__ only for the dev server; production builds receive an
// empty string. The quickstart's Data Plane registers this console as ADMIN_CONSOLE and rewrites the
// packaged config.js to match, which the dev server does not use.
const DEV_CLIENT_ID = process.env.THUNDERID_DEV_CLIENT_ID?.trim();

// https://vite.dev/config/
export default defineConfig(({command}) => ({
  base: BASE_URL,
  define: {
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
    __DEV_CLIENT_ID__: JSON.stringify(command === 'serve' ? (DEV_CLIENT_ID ?? '') : ''),
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('node_modules/@mui/x-data-grid') || id.includes('node_modules/@mui/x-virtualizer')) {
            return 'vendor-mui-x';
          }
          if (
            id.includes('node_modules/@mui/material') ||
            id.includes('node_modules/@mui/system') ||
            id.includes('node_modules/@mui/styled-engine')
          ) {
            return 'vendor-mui';
          }
          if (id.includes('node_modules/@emotion/')) {
            return 'vendor-emotion';
          }
          if (id.includes('node_modules/@wso2/oxygen-ui')) {
            return 'vendor-oxygen';
          }
          if (id.includes('node_modules/react-i18next') || id.includes('node_modules/i18next')) {
            return 'vendor-i18n';
          }
          if (id.includes('node_modules/react-dom') || id.includes('node_modules/react/')) {
            return 'vendor-react';
          }
        },
      },
    },
  },
  server: {
    port: PORT,
    host: HOST,
  },
  resolve: {
    alias: {
      // Force using the same React instance to avoid "Invalid hook call" errors
      // when using linked packages
      react: resolve(__dirname, './node_modules/react'),
      'react-dom': resolve(__dirname, './node_modules/react-dom'),
      'react-router': resolve(__dirname, './node_modules/react-router'),
    },
  },
  plugins: [
    linkWorkspaceSource(),
    prismjsInjectCore(),
    basicSsl(),
    svgr(),
    react(),
    babel({
      presets: [reactCompilerPreset()],
    }),
    ...(ANALYZER_ENABLED
      ? [
          visualizer({
            filename: resolve(currentDir, 'dist', 'stats.html'),
            open: true,
            gzipSize: true,
            brotliSize: true,
          }) as import('vite').PluginOption,
        ]
      : []),
    // Upload bundle stats to Codecov (no-op unless CODECOV_BUNDLE_UPLOAD=true in CI).
    // Must be the last plugin so it analyzes the final bundle.
    codecovVitePlugin({
      enableBundleAnalysis: BUNDLE_ANALYSIS_ENABLED,
      bundleName: 'admin-console',
      gitService: 'github',
    }),
  ],
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: resolve(currentDir, 'src', 'test', 'setup.ts'),
    reporters: process.env.CI ? ['dot'] : ['default'],
    restoreMocks: true,
    css: {
      modules: {
        classNameStrategy: 'non-scoped',
      },
    },
    // Inline deps that need Vite's CSS pipeline or have Node.js-style imports.
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
      reporter: process.env.CI
        ? [['lcov', {projectRoot: resolve(currentDir, '..', '..', '..')}]]
        : ['text', 'json', 'html', ['lcov', {projectRoot: resolve(currentDir, '..', '..', '..')}]],
      exclude: [
        'node_modules/',
        'dist/',
        'public/',
        'coverage/',
        'src/test/',
        '**/*.d.ts',
        '**/*.config.*',
        '**/*.type.ts',
        '**/*.test.ts',
        '**/*.test.tsx',
        '**/*.spec.ts',
        '**/*.spec.tsx',
      ],
    },
  },
}));
