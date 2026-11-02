// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {readFileSync, existsSync} from 'fs';
import {resolve, dirname} from 'path';
import {fileURLToPath} from 'url';
import {codecovVitePlugin} from '@codecov/vite-plugin';
import babel from '@rolldown/plugin-babel';
import {linkWorkspaceSource, prismjsInjectCore} from '@thunderid/build-plugins/vite';
import basicSsl from '@vitejs/plugin-basic-ssl';
import react, {reactCompilerPreset} from '@vitejs/plugin-react';
import {visualizer} from 'rollup-plugin-visualizer';
import type {Plugin} from 'vite';
import svgr from 'vite-plugin-svgr';
import {defineConfig} from 'vitest/config';

const currentDir = dirname(fileURLToPath(import.meta.url));
const PORT = process.env.PORT ? Number(process.env.PORT) : 5193;
const HOST = process.env.HOST ?? 'localhost';
const BASE_URL = process.env.BASE_URL ?? '/console';

// The control plane console serves the console's static assets (icons, favicons and config.js) until
// the control plane has a distribution of its own; nothing in them is specific to a plane.
const PUBLIC_DIR = resolve(currentDir, '../console/public');

// The version is read from the monorepo root, falling back to v0.0.0 when it is missing.
const rootVersionFile = resolve(currentDir, '../../../version.txt');
const VERSION = existsSync(rootVersionFile) ? readFileSync(rootVersionFile, 'utf-8').trim() : 'v0.0.0';

// Serves version.txt as the console does, but from this app itself: it is emitted into this app's own
// build output and answered by its dev server, rather than written into the public folder it shares
// with the console. That folder holds version.txt only once the console has been built, which a
// parallel build does not guarantee.
const versionFile = (): Plugin => ({
  name: 'console-cp-version-file',
  configureServer(server) {
    server.middlewares.use(`${BASE_URL.replace(/\/$/, '')}/version.txt`, (_req, res) => {
      res.setHeader('Content-Type', 'text/plain');
      res.end(VERSION);
    });
  },
  generateBundle() {
    this.emitFile({type: 'asset', fileName: 'version.txt', source: VERSION});
  },
});
const ANALYZER_ENABLED = process.env.ANALYZE === 'true';
const BUNDLE_ANALYSIS_ENABLED = process.env.CODECOV_BUNDLE_UPLOAD === 'true';

// Dev backend URL, from THUNDERID_DEV_SERVER_URL (default https://localhost:8090). Injected into
// __DEV_SERVER_URL__ only for the dev server; production builds receive an empty string.
const DEV_SERVER_URL = process.env.THUNDERID_DEV_SERVER_URL?.trim();

// Dev gate app URL, from THUNDERID_DEV_GATE_URL (default https://localhost:5190). Injected into
// __DEV_GATE_URL__ only for the dev server; production builds receive an empty string.
const DEV_GATE_URL = process.env.THUNDERID_DEV_GATE_URL?.trim();

// Dev URL of the issuer the console signs in through, from THUNDERID_DEV_TRUSTED_ISSUER_URL (unset
// by default, which signs in at the server itself). Injected into __DEV_TRUSTED_ISSUER_URL__ only
// for the dev server; production builds receive an empty string.
const DEV_TRUSTED_ISSUER_URL = process.env.THUNDERID_DEV_TRUSTED_ISSUER_URL?.trim();

// https://vite.dev/config/
export default defineConfig(({command}) => ({
  base: BASE_URL,
  publicDir: PUBLIC_DIR,
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
  define: {
    VERSION: JSON.stringify(VERSION),
    ANALYZER_ENABLED: JSON.stringify(ANALYZER_ENABLED),
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
    __DEV_TRUSTED_ISSUER_URL__: JSON.stringify(command === 'serve' ? (DEV_TRUSTED_ISSUER_URL ?? '') : ''),
  },
  plugins: [
    linkWorkspaceSource(),
    prismjsInjectCore(),
    basicSsl(),
    versionFile(),
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
      bundleName: 'console-cp',
      gitService: 'github',
    }),
  ],
  optimizeDeps: {
    include: ['lodash-es'],
  },
  server: {
    port: PORT,
    host: HOST,
  },
  resolve: {
    alias: {
      '@': resolve(currentDir, 'src'),
      '@/components': resolve(currentDir, 'src', 'components'),
      '@/layouts': resolve(currentDir, 'src', 'layouts'),
      '@/theme': resolve(currentDir, 'src', 'theme'),
      '@/contexts': resolve(currentDir, 'src', 'contexts'),
      '@/lib': resolve(currentDir, 'src', 'lib'),
      '@/hooks': resolve(currentDir, 'src', 'hooks'),
      '@/types': resolve(currentDir, 'src', 'types'),
      // Force using the same React instance to avoid "Invalid hook call" errors
      // when using linked packages
      react: resolve(__dirname, './node_modules/react'),
      'react-dom': resolve(__dirname, './node_modules/react-dom'),
      'react-router': resolve(__dirname, './node_modules/react-router'),
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
    // Pre-bundle remaining heavy dependencies with esbuild for faster test imports.
    deps: {
      optimizer: {
        client: {
          include: ['@mui/x-date-pickers', '@mui/x-tree-view', '@mui/x-charts'],
        },
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
        '**/mockData',
        '**/*.type.ts',
        '**/*.test.ts',
        '**/*.test.tsx',
        '**/*.spec.ts',
        '**/*.spec.tsx',
        '**/EditTokenSettings.tsx',
      ],
    },
  },
}));
