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

import isPlainObject from './isPlainObject';

function mergeValue(tgtVal: unknown, srcVal: unknown): unknown {
  if (Array.isArray(srcVal)) {
    const tgtArr = Array.isArray(tgtVal) ? tgtVal : [];

    return mergeArrays(tgtArr, srcVal);
  }

  if (isPlainObject(srcVal)) {
    const tgtObj = isPlainObject(tgtVal) ? tgtVal : {};

    mergeTwo(tgtObj, srcVal);
    return tgtObj;
  }

  return srcVal;
}

function mergeArrays(target: unknown[], source: unknown[]): unknown[] {
  source.forEach((srcVal, i) => {
    if (srcVal === undefined) return;

    Object.assign(target, {[i]: mergeValue(target[i], srcVal)});
  });

  return target;
}

function mergeTwo(target: Record<string, unknown>, source: Record<string, unknown>): void {
  Object.keys(source).forEach((key) => {
    const srcVal = source[key];

    if (srcVal === undefined) return;

    const tgtVal = target[key];

    Object.assign(target, {[key]: mergeValue(tgtVal, srcVal)});
  });
}

/**
 * Drop-in replacement for lodash `merge`.
 *
 * Recursively merges own enumerable properties of source objects into the
 * destination object. Source properties that resolve to `undefined` do not
 * overwrite existing destination values. Array and plain-object values are
 * merged recursively; all other values are assigned by reference.
 *
 * Mutates and returns the destination object.
 *
 * @param object - The destination object.
 * @param sources - One or more source objects.
 * @returns The mutated destination object.
 */
export default function merge<T extends object>(object: T, ...sources: object[]): T {
  sources.forEach((source) => {
    if (source == null) return;

    mergeTwo(object as Record<string, unknown>, source as Record<string, unknown>);
  });

  return object;
}
