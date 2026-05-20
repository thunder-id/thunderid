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

import {describe, it, expect, vi, beforeEach} from 'vitest';

const loadWithPrefix = async (prefix: string): Promise<(typeof import('../withVendorCSSClassPrefix'))['default']> => {
  vi.resetModules();
  vi.doMock('../../constants/VendorConstants', () => ({
    default: {VENDOR_PREFIX: prefix},
  }));
  const mod: typeof import('../withVendorCSSClassPrefix') = await import('../withVendorCSSClassPrefix');
  return mod.default;
};

describe('withVendorCSSClassPrefix', () => {
  beforeEach(() => {
    vi.resetModules();
    vi.clearAllMocks();
  });

  it('should prefix a simple class name with the vendor prefix', async () => {
    const withVendorCSSClassPrefix: (className: string) => string = await loadWithPrefix('wso2');
    expect(withVendorCSSClassPrefix('sign-in-button')).toBe('wso2-sign-in-button');
  });

  it('should work with BEM-style class names unchanged after the hyphen', async () => {
    const withVendorCSSClassPrefix: (className: string) => string = await loadWithPrefix('wso2');
    expect(withVendorCSSClassPrefix('card__title--large')).toBe('wso2-card__title--large');
  });

  it('should respect different vendor prefixes', async () => {
    const withVendorCSSClassPrefix: (className: string) => string = await loadWithPrefix('acme');
    expect(withVendorCSSClassPrefix('foo')).toBe('acme-foo');
  });

  it('should handle an empty class name by returning just the prefix and hyphen', async () => {
    const withVendorCSSClassPrefix: (className: string) => string = await loadWithPrefix('wso2');
    expect(withVendorCSSClassPrefix('')).toBe('wso2-');
  });

  it('should not mutate or trim the provided class name (preserve spaces/characters as-is)', async () => {
    const withVendorCSSClassPrefix: (className: string) => string = await loadWithPrefix('wso2');
    const original = '  spaced name  ';
    const result: string = withVendorCSSClassPrefix(original);
    expect(result).toBe('wso2-  spaced name  ');
    expect(original).toBe('  spaced name  ');
  });
});
