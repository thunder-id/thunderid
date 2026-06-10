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

/**
 * Regular expression to match a translation flow template literal `{{t(key)}}` (exact, full-string match).
 * Optional whitespace around `{{` / `}}` is allowed.
 */
export const TRANSLATION_FLOW_TEMPLATE_LITERAL_PATTERN = /^\{\{\s*t\([^)]+\)\s*\}\}$/;

/**
 * Regular expression to extract the key from a translation flow template literal `{{t(key)}}`.
 */
export const TRANSLATION_FLOW_TEMPLATE_LITERAL_KEY_PATTERN = /^\{\{\s*t\(([^)]+)\)\s*\}\}$/;

/**
 * Check if a string is exactly a translation flow template literal (`{{ t(key) }}`).
 *
 * This checks that the **entire** string is the template pattern. Use
 * {@link FLOW_TEMPLATE_LITERAL_REGEX} from `parseFlowTemplateLiteral` to detect
 * templates embedded inside a larger string.
 *
 * @param value - The string to test.
 * @returns `true` if the trimmed value matches the pattern, `false` otherwise.
 *
 * @example
 * ```typescript
 * isTranslationFlowTemplateLiteral('{{ t(signin:heading) }}') // true
 * isTranslationFlowTemplateLiteral('hello world')            // false
 * isTranslationFlowTemplateLiteral('Login via {{ t(key) }}') // false — embedded, not exact
 * ```
 */
export default function isTranslationFlowTemplateLiteral(value: string): boolean {
  return TRANSLATION_FLOW_TEMPLATE_LITERAL_PATTERN.test(value.trim());
}
