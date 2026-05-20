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

import parseFlowTemplateLiteral, {
  FLOW_TEMPLATE_LITERAL_REGEX,
  FlowTemplateLiteralResult,
  FlowTemplateLiteralType,
} from './parseFlowTemplateLiteral';
import resolveMeta from './resolveMeta';
import {TranslationFn} from '../../models/v2/translation';
import {ResolveFlowTemplateLiteralsOptions} from '../../models/v2/vars';

/**
 * Global version of {@link FLOW_TEMPLATE_LITERAL_REGEX} for use with `String.prototype.replace`.
 */
const FLOW_TEMPLATE_LITERAL_REGEX_GLOBAL = new RegExp(FLOW_TEMPLATE_LITERAL_REGEX.source, 'g');

/**
 * Resolves all flow template literal expressions in a string.
 *
 * Supported patterns:
 *   - `{{ t(key) }}`       — resolved via the i18n translation function.
 *                            Colon-separated namespaces are converted to dots:
 *                            `{{ t(signin:heading.label) }}` → `t('signin.heading.label')`
 *   - `{{ meta(path) }}`   — resolved via a dot-path lookup on FlowMetadataResponse.
 *                            `{{ meta(application.name) }}` → `meta.application?.name`
 *
 * Flow template literals can be embedded inside larger strings:
 *   `"Login using {{ meta(application.name) }}"` → `"Login using My App"`
 *
 * Unrecognized expressions are left unchanged.
 *
 * @template TFn - The concrete translation function type.
 *
 * @param text - The string to resolve (may contain zero or more flow template literals)
 * @param options - Resolution context: translation function and optional flow metadata
 * @returns The resolved string
 */
export default function resolveFlowTemplateLiterals<TFn extends TranslationFn = TranslationFn>(
  text: string | undefined,
  {t, meta}: ResolveFlowTemplateLiteralsOptions<TFn>,
): string {
  if (!text) {
    return '';
  }

  return text.replace(FLOW_TEMPLATE_LITERAL_REGEX_GLOBAL, (match: string, content: string): string => {
    const parsed: FlowTemplateLiteralResult = parseFlowTemplateLiteral(content.trim());

    if (parsed.type === FlowTemplateLiteralType.TRANSLATION && parsed.key) {
      // Convert colon-separated namespace to dot-separated key
      // e.g. "signin:fields.password.label" → "signin.fields.password.label"
      return t(parsed.key.replace(/:/g, '.'));
    }

    if (parsed.type === FlowTemplateLiteralType.META && parsed.key && meta) {
      return resolveMeta(parsed.key, meta);
    }

    return match;
  });
}
