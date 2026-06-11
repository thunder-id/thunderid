/**
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {readFileSync, copyFileSync, existsSync, writeFileSync} from 'fs';
import {resolve, dirname} from 'path';
import {fileURLToPath} from 'url';
import {prismjsInjectCore} from '@thunderid/build-plugins/vite';
import basicSsl from '@vitejs/plugin-basic-ssl';
import react from '@vitejs/plugin-react';
import {visualizer} from 'rollup-plugin-visualizer';
import svgr from 'vite-plugin-svgr';
import {defineConfig} from 'vitest/config';

const currentDir = dirname(fileURLToPath(import.meta.url));
const PORT = process.env.PORT ? Number(process.env.PORT) : 5191;
const HOST = process.env.HOST ?? 'localhost';
const BASE_URL = process.env.BASE_URL ?? '/console';

// Copy version.txt from monorepo root into public/ so it is served at runtime
// and included in the build output, then read the local copy for the build constant.
// If the root version.txt is missing, create a fallback with v0.0.0.
const rootVersionFile = resolve(currentDir, '../../../version.txt');
const publicVersionFile = resolve(currentDir, 'public', 'version.txt');

if (existsSync(rootVersionFile)) {
  copyFileSync(rootVersionFile, publicVersionFile);
} else {
  writeFileSync(publicVersionFile, 'v0.0.0');
}

const VERSION = readFileSync(publicVersionFile, 'utf-8').trim();
const ANALYZER_ENABLED = process.env.ANALYZE === 'true' || false;

// https://vite.dev/config/
export default defineConfig({
  base: BASE_URL,
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
  },
  plugins: [
    prismjsInjectCore(),
    basicSsl(),
    svgr(),
    react({
      babel: {
        plugins: [['babel-plugin-react-compiler']],
      },
    }),
    // Add visualizer plugin for bundle analysis (only when ANALYZE=true)

    ...(ANALYZER_ENABLED
      ? [
          visualizer({
            filename: resolve(currentDir, 'dist', 'stats.html'),
            open: true,
            gzipSize: true,
            brotliSize: true,
          }),
        ]
      : []),
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
});
