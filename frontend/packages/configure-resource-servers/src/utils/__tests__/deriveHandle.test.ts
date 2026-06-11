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
import {deriveHandle} from '../deriveHandle';

describe('deriveHandle', () => {
  it('joins words with hyphen by default', () => {
    expect(deriveHandle('Payments API')).toBe('payments-api');
  });

  it('joins words with underscore when delimiter is hyphen', () => {
    expect(deriveHandle('Payments API', '-')).toBe('payments_api');
  });

  it('strips special characters and joins remaining words', () => {
    expect(deriveHandle('My@Api#V2')).toBe('my-api-v2');
  });

  it('returns empty string when name is empty', () => {
    expect(deriveHandle('')).toBe('');
  });

  it('returns the word unchanged when name is already lowercase single word', () => {
    expect(deriveHandle('test')).toBe('test');
  });
});
