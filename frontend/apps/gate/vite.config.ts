/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {resolve, dirname} from 'path';
import {fileURLToPath} from 'url';
import basicSsl from '@vitejs/plugin-basic-ssl';
import react from '@vitejs/plugin-react';
import svgr from 'vite-plugin-svgr';
import {defineConfig} from 'vitest/config';

const currentDir = dirname(fileURLToPath(import.meta.url));
const PORT = process.env.PORT ? Number(process.env.PORT) : 5190;
const HOST = process.env.HOST ?? 'localhost';
const BASE_URL = process.env.BASE_URL ?? '/gate';

// https://vite.dev/config/
export default defineConfig({
  base: BASE_URL,
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
    },
  },
  plugins: [
    basicSsl(),
    svgr(),
    react({
      babel: {
        plugins: [['babel-plugin-react-compiler']],
      },
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
        '**/*.type.ts',
        '**/*.test.ts',
        '**/*.test.tsx',
        '**/*.spec.ts',
        '**/*.spec.tsx',
      ],
    },
  },
});
