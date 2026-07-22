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

import {describe, expect, it} from 'vitest';
import isValidLogoUri from '../isValidLogoUri';

describe('isValidLogoUri', () => {
  // Mirrors the backend `IsValidLogoURI` allowlist (http_util.go).
  it.each([
    ['', false],
    ['https://example.com/logo.png', true],
    ['http://example.com/logo.png', true],
    ['data:image/png;base64,abc123', true],
    ['blob:https://example.com/uuid', true],
    ['emoji:smile', true],
    ['/images/logo.png', true],
    ['./logo.png', false],
    ['logo.png', false],
    ['://invalid', false],
    ['javascript:alert(1)', false],
    ['file:///etc/passwd', false],
    ['ftp://example.com/logo.png', false],
    ['http:///no-host', false],
    ['https:///no-host', false],
    ['http://example.com:8080/logo.png', true],
    ['http://user@example.com/logo.png', true],
    ['http://example.com:99999/logo.png', true],
    ['http://example.com:abc', false],
    ['http://exa mple.com', false],
  ])('returns %s -> %s', (uri, expected) => {
    expect(isValidLogoUri(uri)).toBe(expected);
  });
});
