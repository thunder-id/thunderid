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
import buildValidatorFromRules from '../v2/buildValidatorFromRules';

describe('buildValidatorFromRules', () => {
  it('returns null when no rules are provided', () => {
    expect(buildValidatorFromRules(undefined)).toBeNull();
    expect(buildValidatorFromRules([])).toBeNull();
  });

  it('composes a validator that returns null when all rules pass', () => {
    const validate = buildValidatorFromRules([
      {type: 'minLength', value: 3, message: 'short'},
      {type: 'maxLength', value: 10, message: 'long'},
    ]);
    expect(validate).not.toBeNull();
    expect(validate!('hello')).toBeNull();
  });

  it('returns the first failing rule message — first failure wins', () => {
    const validate = buildValidatorFromRules([
      {type: 'minLength', value: 8, message: 'too short'},
      {type: 'regex', value: '[0-9]', message: 'must contain a digit'},
    ]);
    // "abc" fails BOTH rules. We expect the first one's message.
    expect(validate!('abc')).toBe('too short');
  });

  it('skips rules that pass and reports the first failing one', () => {
    const validate = buildValidatorFromRules([
      {type: 'minLength', value: 3, message: 'too short'},
      {type: 'regex', value: '[0-9]', message: 'no digit'},
      {type: 'maxLength', value: 20, message: 'too long'},
    ]);
    // "hello" passes minLength and maxLength but fails the regex.
    expect(validate!('hello')).toBe('no digit');
  });

  it('enforces a date format via a regex rule (DATE_INPUT use case)', () => {
    const validate = buildValidatorFromRules([
      {type: 'regex', value: '^\\d{4}-\\d{2}-\\d{2}$', message: 'validation.dateFormat.invalid'},
    ]);
    expect(validate!('1990-01-15')).toBeNull();
    expect(validate!('2026/02/30')).toBe('validation.dateFormat.invalid');
    expect(validate!('not-a-date')).toBe('validation.dateFormat.invalid');
  });

  it('returns a function with a FormField.validator-compatible signature', () => {
    // Sanity check: the validator returned matches the shape useForm expects
    // (value: string) => string | null — so it can be plugged in directly.
    const validate = buildValidatorFromRules([{type: 'minLength', value: 1, message: 'm'}]);
    expect(typeof validate).toBe('function');
    expect(validate!('x')).toBeNull();
    expect(validate!('')).toBe('m');
  });
});
