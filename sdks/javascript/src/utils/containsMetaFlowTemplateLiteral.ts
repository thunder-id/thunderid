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
 * Build a regex that matches `{{ meta(key) }}` (with optional whitespace) anywhere
 * within a string, escaping any special regex characters in `key`.
 */
function buildMetaFlowTemplateLiteralRegex(key: string): RegExp {
  const escapedKey: string = key.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

  return new RegExp(`\\{\\{\\s*meta\\(${escapedKey}\\)\\s*\\}\\}`);
}

/**
 * Check whether a string contains a `{{ meta(key) }}` flow template literal anywhere within it.
 *
 * Unlike {@link isMetaFlowTemplateLiteral}, which requires the **entire** string to be the
 * template, this function detects the pattern embedded inside a larger string such as an
 * HTML label or sentence.
 *
 * Whitespace around `{{` / `}}` is allowed, e.g. `{{ meta(application.signUpUrl) }}`.
 *
 * @param str - The string to search (may be a plain value or an HTML fragment).
 * @param key - The meta path to look for, e.g. `"application.signUpUrl"`.
 * @returns `true` if the pattern is found anywhere in `str`, `false` otherwise.
 *
 * @example
 * ```typescript
 * containsMetaFlowTemplateLiteral('<a href="{{meta(application.signUpUrl)}}">Sign up</a>', 'application.signUpUrl')
 * // true
 *
 * containsMetaFlowTemplateLiteral('<a href="https://example.com">Sign up</a>', 'application.signUpUrl')
 * // false
 * ```
 */
export default function containsMetaFlowTemplateLiteral(str: string, key: string): boolean {
  return buildMetaFlowTemplateLiteralRegex(key).test(str);
}

/**
 * Replace all occurrences of `{{ meta(key) }}` (with optional whitespace) in `str`
 * with `replacement`.
 *
 * @param str - The source string.
 * @param key - The meta path to replace, e.g. `"application.signUpUrl"`.
 * @param replacement - The value to substitute for each match.
 * @returns A new string with all occurrences replaced.
 *
 * @example
 * ```typescript
 * replaceMetaFlowTemplateLiteral('Sign up at {{ meta(application.signUpUrl) }}', 'application.signUpUrl', 'https://example.com')
 * // 'Sign up at https://example.com'
 * ```
 */
export function replaceMetaFlowTemplateLiteral(str: string, key: string, replacement: string): string {
  const escapedKey: string = key.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const regex = new RegExp(`\\{\\{\\s*meta\\(${escapedKey}\\)\\s*\\}\\}`, 'g');

  return str.replace(regex, replacement);
}
