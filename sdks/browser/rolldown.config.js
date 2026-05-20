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
import {createRequire} from 'module';
import {resolve, dirname} from 'path';
import {fileURLToPath} from 'url';
import {rolldown} from 'rolldown';

const __dirname = dirname(fileURLToPath(import.meta.url));

const require = createRequire(import.meta.url);
const pkg = JSON.parse(readFileSync('./package.json', 'utf8'));

// Get dependencies excluding crypto-related ones that need to be bundled
const externalDeps = [...Object.keys(pkg.dependencies || {}), ...Object.keys(pkg.peerDependencies || {})].filter(
  (dep) => !['crypto-browserify', 'randombytes', 'buffer'].includes(dep),
);

const polyfillAliases = {
  crypto: require.resolve('crypto-browserify'),
  buffer: require.resolve('buffer/index.js'),
};

const inlineWorkerPlugin = () => ({
  name: 'inline-worker',
  async load(id) {
    if (!/\.worker\.(ts|js|mjs)$/.test(id)) return null;

    const bundle = await rolldown({
      input: id,
      platform: 'browser',
      define: {
        global: 'self',
        globalThis: 'self',
        'process.browser': 'true',
        'process.env.NODE_DEBUG': 'false',
        'process.version': '"16.0.0"',
      },
      resolve: {
        alias: {
          ...polyfillAliases,
          '@thunderid/javascript': resolve(__dirname, '../javascript/src/index.ts'),
        },
      },
    });

    const {output} = await bundle.generate({format: 'iife'});
    await bundle.close();

    const chunk = output.find((item) => item.type === 'chunk');

    return `
      const blob = new Blob([${JSON.stringify(chunk.code)}], {type: 'application/javascript'});
      const url = URL.createObjectURL(blob);
      export default class extends Worker {
        constructor() { super(url); }
      }
    `;
  },
});

const commonOptions = {
  external: externalDeps,
  input: 'src/index.ts',
  platform: 'browser',
  define: {
    global: 'globalThis', // Required by crypto-browserify
    'process.browser': 'true',
    'process.env.NODE_DEBUG': 'false',
    'process.version': '"16.0.0"',
  },
  resolve: {alias: polyfillAliases},
  plugins: [inlineWorkerPlugin()],
};

const esmBundle = await rolldown(commonOptions);
await esmBundle.write({
  banner: `import { Buffer } from 'buffer/index.js';\nif (typeof window !== 'undefined' && !window.Buffer) { window.Buffer = Buffer; }`,
  file: 'dist/index.js',
  format: 'esm',
  sourcemap: true,
});
await esmBundle.close();

const cjsBundle = await rolldown(commonOptions);
await cjsBundle.write({
  banner: `const { Buffer } = require('buffer/index.js');\nif (typeof window !== 'undefined' && !window.Buffer) { window.Buffer = Buffer; }`,
  file: 'dist/cjs/index.js',
  format: 'cjs',
  sourcemap: true,
});
await cjsBundle.close();
