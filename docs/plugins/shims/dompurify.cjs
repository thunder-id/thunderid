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

// SSR shim for dompurify — DOMPurify requires a DOM environment and fails to
// initialize in Node.js SSR. This no-op shim is used only during Docusaurus
// server-side rendering; the real package runs in the browser.

// eslint-disable-next-line @typescript-eslint/no-empty-function
function noop() {}

const DOMPurify = {
  addHook: noop,
  clearConfig: noop,
  isSupported: false,
  removeAllHooks: noop,
  removeHook: noop,
  removeHooks: noop,
  sanitize: function (input) {
    return typeof input === 'string' ? input : '';
  },
  setConfig: noop,
  version: '0.0.0',
};

module.exports = DOMPurify;
module.exports.default = DOMPurify;
