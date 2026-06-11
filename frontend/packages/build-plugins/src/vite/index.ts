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

import type {Plugin} from 'vite';

// prismjs language files reference `Prism` as a global with no import — add one so
// Rollup sees the dependency edge and evaluates the core before any language file.
export function prismjsInjectCore(): Plugin {
  return {
    name: 'prismjs-inject-core',
    transform(code: string, id: string) {
      if (/[/\\]prismjs[/\\]components[/\\]prism-(?!core)/.test(id)) {
        // map: null intentionally omitted — prepending a line shifts devtools line numbers by 1.
        return {code: `import Prism from 'prismjs';\n${code}`, map: null};
      }
      return null;
    },
  };
}
