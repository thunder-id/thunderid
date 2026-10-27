// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Builds the console as a *remote*: an ES module (`remote-entry.js`) a host application imports at
 * runtime and mounts without an iframe. See src/embed/remote-entry.tsx for the entry and
 * src/embed/contract.ts for the contract.
 *
 * The output is served from the console's own origin under `/remotes/thunderid/`, which the host's
 * ingress routes to this build.
 */
import {resolve} from 'path';
import {mergeConfig, type UserConfig} from 'vite';
import baseConfig from './vite.config';

const BASE = '/remotes/thunderid/';
const PORT = Number(process.env.PORT ?? 5301);

const base: UserConfig =
  typeof baseConfig === 'function' ? baseConfig({command: 'build', mode: 'production'}) : baseConfig;

export default mergeConfig(base, {
  base: BASE,
  build: {
    emptyOutDir: true,
    outDir: resolve(__dirname, 'dist-remote'),
    rollupOptions: {
      input: {'remote-entry': resolve(__dirname, 'src/embed/remote-entry.tsx')},
      output: {entryFileNames: '[name].js'},
      preserveEntrySignatures: 'exports-only',
    },
    sourcemap: true,
  },
  preview: {cors: true, host: '127.0.0.1', port: PORT, strictPort: true},
  resolve: {
    // The host keeps its own copies; the remote must not end up with two Reacts of its own.
    dedupe: ['react', 'react-dom', 'react-router', '@emotion/react', '@emotion/styled', '@emotion/cache'],
  },
} satisfies UserConfig);
