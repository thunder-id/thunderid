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
import parseApiErrorMessage from '../parseApiErrorMessage';

describe('parseApiErrorMessage', () => {
  it('should return description.defaultValue when present', () => {
    const errorText: string = JSON.stringify({
      code: 'SSE-5000',
      description: {defaultValue: 'An unexpected error occurred while processing the request', key: 'error.desc'},
      message: {defaultValue: 'Internal server error', key: 'error.msg'},
    });
    expect(parseApiErrorMessage(errorText)).toBe('An unexpected error occurred while processing the request');
  });

  it('should fall back to message.defaultValue when description is absent', () => {
    const errorText: string = JSON.stringify({
      code: 'SSE-5000',
      message: {defaultValue: 'Internal server error', key: 'error.msg'},
    });
    expect(parseApiErrorMessage(errorText)).toBe('Internal server error');
  });

  it('should return raw text when the response is plain text (not JSON)', () => {
    expect(parseApiErrorMessage('Invalid credentials')).toBe('Invalid credentials');
  });

  it('should return raw text when JSON does not contain known fields', () => {
    const errorText: string = JSON.stringify({code: 'ERR-001', error: 'something'});
    expect(parseApiErrorMessage(errorText)).toBe(errorText);
  });

  it('should return raw text when description.defaultValue is an empty string', () => {
    const errorText: string = JSON.stringify({
      code: 'SSE-5000',
      description: {defaultValue: '', key: 'error.desc'},
      message: {defaultValue: 'Internal server error', key: 'error.msg'},
    });
    expect(parseApiErrorMessage(errorText)).toBe('Internal server error');
  });

  it('should return raw text when both defaultValue fields are absent', () => {
    const errorText: string = JSON.stringify({
      code: 'SSE-5000',
      description: {key: 'error.desc'},
      message: {key: 'error.msg'},
    });
    expect(parseApiErrorMessage(errorText)).toBe(errorText);
  });

  it('should return raw text for malformed JSON', () => {
    expect(parseApiErrorMessage('{not valid json')).toBe('{not valid json');
  });

  it('should return empty string when given empty string', () => {
    expect(parseApiErrorMessage('')).toBe('');
  });
});
