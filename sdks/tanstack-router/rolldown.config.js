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

const external = [...Object.keys(pkg.dependencies || {}), ...Object.keys(pkg.peerDependencies || {})];

const commonOptions = {
  external,
  input: 'src/index.ts',
  platform: 'browser',
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
