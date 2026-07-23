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

import {describe, it, expect} from 'vitest';
import {sampleIndicesIncluding} from '../sampleIndices';

describe('sampleIndicesIncluding', () => {
  it('should return a sample of the requested size', () => {
    expect(sampleIndicesIncluding(20, 5, 3)).toHaveLength(5);
  });

  it('should always include the given index, even when the odds of a plain random sample are low', () => {
    for (let i = 0; i < 50; i += 1) {
      expect(sampleIndicesIncluding(50, 3, 37)).toContain(37);
    }
  });

  it('should contain no duplicates', () => {
    const result = sampleIndicesIncluding(20, 8, 5);
    expect(new Set(result).size).toBe(result.length);
  });

  it('should fall back to a plain sample when mustInclude is out of range', () => {
    const result = sampleIndicesIncluding(10, 4, -1);
    expect(result).toHaveLength(4);
    expect(result.every((n) => n >= 0 && n < 10)).toBe(true);
  });

  it('should fall back to a plain sample when mustInclude is beyond count', () => {
    const result = sampleIndicesIncluding(10, 4, 99);
    expect(result).toHaveLength(4);
  });
});
