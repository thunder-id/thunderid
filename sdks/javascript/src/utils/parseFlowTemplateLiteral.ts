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
 * Regular expression to detect a flow template literal wrapped in double braces.
 * Matches patterns like `{{ t(key) }}`, `{{ meta(key) }}`, etc.
 *
 * Note: this regex has no `g` flag — use `new RegExp(FLOW_TEMPLATE_LITERAL_REGEX.source, 'g')`
 * when global replacement is needed (e.g. in resolveFlowTemplateLiterals).
 */
export const FLOW_TEMPLATE_LITERAL_REGEX = /\{\{\s*([^}]+)\s*\}\}/;

/**
 * Regular expression to parse a function-call expression inside flow template braces.
 * Matches `funcName(arg)` and captures the function name and argument.
 */
export const FLOW_TEMPLATE_FUNCTION_REGEX = /^(\w+)\(([^)]+)\)$/;

/**
 * Flow template literal types supported by the SDK.
 *
 * Values correspond to the function name used in the template expression,
 * so that a `{ t }` object from `useTranslation()` can be passed directly
 * as a handler map.
 */
export enum FlowTemplateLiteralType {
  /** Meta template literal — `{{ meta(path) }}` — resolves against flow/page metadata */
  META = 'meta',
  /** Translation template literal — `{{ t(key) }}` */
  TRANSLATION = 't',
  /** Unknown or unsupported template literal format */
  UNKNOWN = 'unknown',
}

/**
 * Result of parsing a flow template literal.
 */
export interface FlowTemplateLiteralResult {
  /**
   * The extracted key or path from the template literal.
   * e.g. `"signin:heading"` from `"{{ t(signin:heading) }}"`.
   */
  key?: string;
  /** The original template literal content before parsing */
  originalValue: string;
  /** The type of flow template literal that was detected */
  type: FlowTemplateLiteralType;
}

/**
 * Map of handler functions keyed by {@link FlowTemplateLiteralType}.
 *
 * When provided to a resolver, the matching handler is called with the extracted key.
 * Because `FlowTemplateLiteralType.TRANSLATION === 't'`, you can pass the `{ t }` object
 * from `useTranslation()` directly.
 *
 * @example
 * ```typescript
 * const { t } = useTranslation();
 * // handler map: { t: (key) => string }
 * ```
 */
export type FlowTemplateLiteralHandlers = Partial<Record<FlowTemplateLiteralType, (key: string) => string>>;

/**
 * Parse a flow template literal content string and extract its type and key.
 *
 * Supports function-call expressions like:
 * - `t(signin:heading)`  → type `TRANSLATION`, key `"signin:heading"`
 * - `meta(application.name)` → type `META`, key `"application.name"`
 *
 * Surrounding quotes on the key argument are stripped automatically.
 *
 * @param content - The content inside the template literal braces (without `{{ }}`).
 * @returns Parsed flow template literal information.
 *
 * @example
 * ```typescript
 * parseFlowTemplateLiteral('t(signin:heading)')
 * // { type: FlowTemplateLiteralType.TRANSLATION, key: 'signin:heading', originalValue: 't(signin:heading)' }
 *
 * parseFlowTemplateLiteral('meta(application.name)')
 * // { type: FlowTemplateLiteralType.META, key: 'application.name', originalValue: 'meta(application.name)' }
 * ```
 */
export default function parseFlowTemplateLiteral(content: string): FlowTemplateLiteralResult {
  const originalValue: string = content;
  const match: RegExpExecArray | null = FLOW_TEMPLATE_FUNCTION_REGEX.exec(content);

  if (!match) {
    return {originalValue, type: FlowTemplateLiteralType.UNKNOWN};
  }

  const [, functionName, rawKey] = match;
  const key: string = rawKey.trim().replace(/^['"]|['"]$/g, '');

  switch (functionName as FlowTemplateLiteralType) {
    case FlowTemplateLiteralType.TRANSLATION:
      return {key, originalValue, type: FlowTemplateLiteralType.TRANSLATION};
    case FlowTemplateLiteralType.META:
      return {key, originalValue, type: FlowTemplateLiteralType.META};
    default:
      return {originalValue, type: FlowTemplateLiteralType.UNKNOWN};
  }
}
