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
 * Picks `size` random, distinct indices out of `[0, count)`.
 */
export function sampleIndices(count: number, size: number): number[] {
  const all: number[] = Array.from({length: count}, (_, i) => i);
  return all.sort(() => Math.random() - 0.5).slice(0, size);
}

/**
 * Same as {@link sampleIndices}, but guarantees `mustInclude` is part of the result (when it's
 * a valid index) so a previously-selected swatch is never missing — and therefore never
 * unselected-looking — when the grid (re)mounts, e.g. on reopening its popover.
 */
export function sampleIndicesIncluding(count: number, size: number, mustInclude: number): number[] {
  if (mustInclude < 0 || mustInclude >= count || size <= 0) return sampleIndices(count, size);
  const rest: number[] = Array.from({length: count}, (_, i) => i).filter((i) => i !== mustInclude);
  const picked: number[] = rest.sort(() => Math.random() - 0.5).slice(0, size - 1);
  return [...picked, mustInclude].sort(() => Math.random() - 0.5);
}
