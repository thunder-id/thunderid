// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {readFileSync} from 'fs';
import {join} from 'path';
import {defineConfig} from 'rolldown';

const pkg = JSON.parse(readFileSync('./package.json', 'utf8'));

const external = [
  ...Object.keys(pkg.dependencies || {}),
  ...Object.keys(pkg.peerDependencies || {}),
  'react/jsx-runtime',
  // Needed to avoid hook ordering issues.
  /^@mui\//,
  // Peer dep subpaths are not matched by exact string - add them explicitly.
  '@thunderid/logger/react',
];

const commonOptions = {
  input: join('src', 'index.ts'),
  external,
  target: 'es2020',
  sourcemap: true,
};

export default defineConfig([
  // ✅ ESM build (for browsers/bundlers)
  {
    ...commonOptions,
    platform: 'browser',
    output: {
      dir: 'dist',
      format: 'esm',
      preserveModules: true,
      preserveModulesRoot: 'src',
    },
  },
  // ✅ CommonJS build (for Node/SSR/testing)
  {
    ...commonOptions,
    platform: 'node',
    output: {
      dir: join('dist', 'cjs'),
      entryFileNames: '[name].cjs',
      format: 'cjs',
      preserveModules: true,
      preserveModulesRoot: 'src',
    },
  },
]);
