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

import {describe, it, expect, vi} from 'vitest';
import ThunderIDRuntimeError from '../../errors/ThunderIDRuntimeError';
import isRecognizedBaseUrlPattern from '../isRecognizedBaseUrlPattern';

vi.mock('../logger', () => ({default: {warn: vi.fn()}}));

describe('isRecognizedBaseUrlPattern', () => {
  it('should return true for recognized base URL pattern', () => {
    expect(isRecognizedBaseUrlPattern('https://dev.asgardeo.io/t/dxlab')).toBe(true);
    expect(isRecognizedBaseUrlPattern('https://example.com/t/org')).toBe(true);
    expect(isRecognizedBaseUrlPattern('https://foo.com/t/bar/')).toBe(true);
    expect(isRecognizedBaseUrlPattern('https://foo.com/t/bar/extra')).toBe(true);
  });

  it('should return false for unrecognized base URL pattern', () => {
    expect(isRecognizedBaseUrlPattern('https://dev.asgardeo.io/tenant/dxlab')).toBe(false);
    expect(isRecognizedBaseUrlPattern('https://dev.asgardeo.io/')).toBe(false);
    expect(isRecognizedBaseUrlPattern('https://dev.asgardeo.io/t')).toBe(false);
    expect(isRecognizedBaseUrlPattern('https://dev.asgardeo.io/other/path')).toBe(false);
  });

  it('should throw ThunderIDRuntimeError if baseUrl is undefined', () => {
    expect(() => isRecognizedBaseUrlPattern(undefined)).toThrow(ThunderIDRuntimeError);

    try {
      isRecognizedBaseUrlPattern(undefined);
    } catch (e: any) {
      expect(e).toBeInstanceOf(ThunderIDRuntimeError);
      expect(e.message).toMatch(/Base URL is required/);
    }
  });

  it('should throw ThunderIDRuntimeError for invalid URL format', () => {
    expect(() => isRecognizedBaseUrlPattern('not-a-valid-url')).toThrow(ThunderIDRuntimeError);

    try {
      isRecognizedBaseUrlPattern('not-a-valid-url');
    } catch (e: any) {
      expect(e).toBeInstanceOf(ThunderIDRuntimeError);
      expect(e.message).toMatch(/Invalid base URL format/);
    }
  });
});
