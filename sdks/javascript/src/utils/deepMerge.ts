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

/**
 * Checks if a value is a plain object (not an array, function, date, etc.)
 *
 * @param value - The value to check
 * @returns True if the value is a plain object
 */
const isPlainObject = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' &&
  value !== null &&
  !Array.isArray(value) &&
  !(value instanceof Date) &&
  !(value instanceof RegExp) &&
  Object.prototype.toString.call(value) === '[object Object]';

/**
 * Recursively merges the properties of source objects into a target object.
 * Similar to Lodash's merge function, this creates a deep copy and merges
 * nested objects recursively. Arrays and non-plain objects are replaced entirely.
 *
 * @param target - The target object to merge into
 * @param sources - One or more source objects to merge from
 * @returns A new object with merged properties
 *
 * @example
 * ```typescript
 * const obj1 = { a: 1, b: { x: 1, y: 2 } };
 * const obj2 = { b: { y: 3, z: 4 }, c: 3 };
 * const result = deepMerge(obj1, obj2);
 * // Result: { a: 1, b: { x: 1, y: 3, z: 4 }, c: 3 }
 * ```
 *
 * @example
 * ```typescript
 * const config = { theme: { colors: { primary: 'blue' } } };
 * const userPrefs = { theme: { colors: { secondary: 'red' } } };
 * const merged = deepMerge(config, userPrefs);
 * // Result: { theme: { colors: { primary: 'blue', secondary: 'red' } } }
 * ```
 */
const deepMerge = <T extends Record<string, any>>(
  target: T,
  ...sources: (Record<string, any> | undefined | null)[]
): T => {
  if (!target || typeof target !== 'object') {
    throw new Error('Target must be an object');
  }

  const result: T = {...target};

  sources.forEach((source: Record<string, any> | undefined | null) => {
    if (!source || typeof source !== 'object') {
      return;
    }

    Object.keys(source).forEach((key: string) => {
      const sourceValue: any = source[key];
      const targetValue: any = (result as any)[key];

      if (isPlainObject(sourceValue) && isPlainObject(targetValue)) {
        (result as any)[key] = deepMerge(targetValue, sourceValue);
      } else if (sourceValue !== undefined) {
        (result as any)[key] = sourceValue;
      }
    });
  });

  return result;
};

export default deepMerge;
