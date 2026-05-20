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
import {rolldown} from 'rolldown';

const pkg = JSON.parse(readFileSync('./package.json', 'utf8'));

const externalPackages = [...Object.keys(pkg.dependencies || {}), ...Object.keys(pkg.peerDependencies || {})];
const external = (id) => externalPackages.some((dep) => id === dep || id.startsWith(dep + '/'));

const preserveDirectivesPlugin = () => {
  const moduleDirectives = new Map();

  return {
    name: 'preserve-directives',
    transform(code, id) {
      if (!/\.(js|ts|jsx|tsx)$/.test(id) || /node_modules/.test(id)) return null;

      const directives = [];
      let remaining = code;
      const re = /^(['"])use ([^'"]+)\1;?\r?\n/;
      let match;
      while ((match = re.exec(remaining))) {
        directives.push(`"use ${match[2]}"`);
        remaining = remaining.slice(match[0].length);
      }

      if (directives.length === 0) return null;

      moduleDirectives.set(id, directives);
      return {code: remaining, map: null};
    },
    renderChunk(code, chunk) {
      const seen = new Set();
      for (const id of chunk.moduleIds) {
        for (const d of moduleDirectives.get(id) ?? []) seen.add(d);
      }
      if (seen.size === 0) return null;
      return {code: `${[...seen].map((d) => `${d};`).join('\n')}\n${code}`, map: null};
    },
  };
};

const commonOptions = {
  external,
  input: 'src/index.ts',
  platform: 'browser',
  plugins: [preserveDirectivesPlugin()],
};

const esmBundle = await rolldown(commonOptions);
await esmBundle.write({
  file: 'dist/index.js',
  format: 'esm',
  sourcemap: true,
});
await esmBundle.close();

const cjsBundle = await rolldown(commonOptions);
await cjsBundle.write({
  file: 'dist/cjs/index.js',
  format: 'cjs',
  sourcemap: true,
});
await cjsBundle.close();
