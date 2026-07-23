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
 * Picks `count` random, distinct items out of `items`.
 */
export function sample<T>(items: T[], count: number): T[] {
  if (count <= 0) return [];
  return [...items].sort(() => Math.random() - 0.5).slice(0, count);
}

/**
 * Same as {@link sample}, but guarantees `mustInclude` is part of the result (when present in
 * `items`) so a previously-selected icon is never missing — and therefore never
 * unselected-looking — when the grid (re)mounts, e.g. on reopening its popover.
 */
export function sampleIncluding(items: string[], count: number, mustInclude: string): string[] {
  if (!mustInclude || !items.includes(mustInclude) || count <= 0) return sample(items, count);
  const rest: string[] = items.filter((item) => item !== mustInclude);
  const picked: string[] = sample(rest, count - 1);
  return [...picked, mustInclude].sort(() => Math.random() - 0.5);
}
