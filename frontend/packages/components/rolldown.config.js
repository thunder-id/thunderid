/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import {readFileSync} from 'fs';
import {join, resolve} from 'path';
import {defineConfig} from 'rolldown';

const pkg = JSON.parse(readFileSync('./package.json', 'utf8'));

const external = [
  ...Object.keys(pkg.dependencies || {}),
  ...Object.keys(pkg.peerDependencies || {}),
  'react/jsx-runtime',
  // Needed to avoid hook ordering issues.
  /^@mui\//,
  // Peer dep subpaths are not matched by exact string — add them explicitly.
  '@thunderid/logger/react',
];

const alias = {
  '@': resolve('src'),
  '@/api': resolve('src', 'api'),
  '@/components': resolve('src', 'components'),
  '@/config': resolve('src', 'config'),
  '@/constants': resolve('src', 'constants'),
  '@/contexts': resolve('src', 'contexts'),
  '@/data': resolve('src', 'data'),
  '@/hooks': resolve('src', 'hooks'),
  '@/models': resolve('src', 'models'),
  '@/pages': resolve('src', 'pages'),
  '@/utils': resolve('src', 'utils'),
};

const commonOptions = {
  input: join('src', 'index.ts'),
  external,
  target: 'es2020',
  sourcemap: true,
  resolve: {alias},
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
