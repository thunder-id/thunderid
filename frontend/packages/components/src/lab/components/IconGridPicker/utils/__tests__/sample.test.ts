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
import {sampleIncluding} from '../sample';

const ITEMS = Array.from({length: 30}, (_, i) => `icon-${i}`);

describe('sampleIncluding', () => {
  it('should return a sample of the requested size', () => {
    expect(sampleIncluding(ITEMS, 5, 'icon-3')).toHaveLength(5);
  });

  it('should always include the given item, even when the odds of a plain random sample are low', () => {
    for (let i = 0; i < 50; i += 1) {
      expect(sampleIncluding(ITEMS, 3, 'icon-27')).toContain('icon-27');
    }
  });

  it('should contain no duplicates', () => {
    const result = sampleIncluding(ITEMS, 8, 'icon-5');
    expect(new Set(result).size).toBe(result.length);
  });

  it('should fall back to a plain sample when mustInclude is empty', () => {
    const result = sampleIncluding(ITEMS, 4, '');
    expect(result).toHaveLength(4);
  });

  it('should fall back to a plain sample when mustInclude is not in items', () => {
    const result = sampleIncluding(ITEMS, 4, 'not-a-real-icon');
    expect(result).toHaveLength(4);
    expect(result).not.toContain('not-a-real-icon');
  });
});
