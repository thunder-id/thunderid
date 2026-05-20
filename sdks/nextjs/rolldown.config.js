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

import {readFileSync} from 'fs';
import {join} from 'path';
import {defineConfig} from 'rolldown';

const pkg = JSON.parse(readFileSync('./package.json', 'utf8'));

const external = [...Object.keys(pkg.dependencies || {}), ...Object.keys(pkg.peerDependencies || {})];

const nodeOptions = {
  input: [join('src', 'index.ts'), join('src', 'server', 'index.ts')],
  preserveModules: true,
  external,
  platform: 'node',
  target: 'es2020',
  sourcemap: true,
};

const edgeOptions = {
  input: [join('src', 'middleware.ts')],
  preserveModules: true,
  external,
  platform: 'browser',
  target: 'es2020',
  sourcemap: true,
};

export default defineConfig([
  // ESM build (node)
  {
    ...nodeOptions,
    output: {
      dir: 'dist',
      format: 'esm',
      preserveModulesRoot: 'src',
    },
  },
  // CommonJS build (node)
  {
    ...nodeOptions,
    output: {
      dir: join('dist', 'cjs'),
      entryFileNames: '[name].cjs',
      format: 'cjs',
      preserveModulesRoot: 'src',
    },
  },
  // Edge/middleware ESM build (browser)
  {
    ...edgeOptions,
    output: {
      dir: 'dist',
      format: 'esm',
      preserveModulesRoot: 'src',
    },
  },
  // Edge/middleware CommonJS build (browser)
  {
    ...edgeOptions,
    output: {
      dir: join('dist', 'cjs'),
      entryFileNames: '[name].cjs',
      format: 'cjs',
      preserveModulesRoot: 'src',
    },
  },
]);
