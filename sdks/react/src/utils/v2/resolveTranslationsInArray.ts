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

import {FlowMetadataResponse} from '@thunderid/browser';
import {resolveTranslationsInObject} from './resolveTranslationsInObject';
import {UseTranslation} from '../../hooks/useTranslation';

/**
 * Recursively resolves translation and meta template strings in an array of objects.
 * @param items - Array of objects to process
 * @param t - The translation function from useTranslation
 * @param properties - Array of property names to resolve (optional)
 * @param meta - Optional flow metadata for resolving meta() expressions
 * @returns A new array with resolved translations
 */
const resolveTranslationsInArray = <T extends Record<string, any>>(
  items: T[],
  t: UseTranslation['t'],
  properties?: string[],
  meta?: FlowMetadataResponse | null,
): T[] =>
  items.map((item: T) => {
    const resolved: T = resolveTranslationsInObject(item, t, properties, meta);

    // If the item has nested components (like BLOCK or STACK type), resolve those too
    if (resolved['components'] && Array.isArray(resolved['components'])) {
      (resolved as any).components = resolveTranslationsInArray(resolved['components'], t, properties, meta);
    }

    return resolved;
  });

export default resolveTranslationsInArray;
