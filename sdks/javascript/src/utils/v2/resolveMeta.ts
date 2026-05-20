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

import {FlowMetadataResponse} from '../../models/v2/flow-meta-v2';

/**
 * Resolves a dot-path expression against a FlowMetadataResponse object.
 *
 * Supports both camelCase paths (e.g. `logoUrl`) and snake_case API responses
 * (e.g. `logo_url`). When a camelCase segment is not found directly, the
 * function falls back to its snake_case equivalent.
 *
 * @example
 * resolveMeta('application.name', meta) // → 'My App'
 * resolveMeta('ou.name', meta)           // → 'My Org'
 *
 * @param path - Dot-separated path into the meta object (e.g. 'application.name')
 * @param meta - The FlowMetadataResponse to look up
 * @returns The resolved string value, or empty string if not found
 */
export default function resolveMeta(path: string, meta: FlowMetadataResponse): string {
  const value: unknown = path.split('.').reduce<unknown>((current: unknown, part: string) => {
    if (current == null || typeof current !== 'object') {
      return undefined;
    }

    const obj: Record<string, unknown> = current as Record<string, unknown>;
    const snakePart: string = part.replace(/[A-Z]/g, (c: string) => `_${c.toLowerCase()}`);

    return part in obj ? obj[part] : obj[snakePart];
  }, meta);

  return value != null ? String(value) : '';
}
