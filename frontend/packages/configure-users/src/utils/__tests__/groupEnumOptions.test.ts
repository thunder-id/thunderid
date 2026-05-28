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

import {describe, it, expect} from 'vitest';
import {groupEnumOptions, getModelDisplayName} from '../groupEnumOptions';

describe('groupEnumOptions', () => {
  it('groups enum values by provider prefix', () => {
    const enums = [
      'claude-opus-4.7',
      'claude-sonnet-4.6',
      'claude-haiku-4.5',
      'openai-gpt-5.4-pro',
      'openai-gpt-5.4-mini',
    ];
    const result = groupEnumOptions(enums);

    expect(result.size).toBe(2);
    expect(result.get('claude')).toEqual(['claude-opus-4.7', 'claude-sonnet-4.6', 'claude-haiku-4.5']);
    expect(result.get('openai')).toEqual(['openai-gpt-5.4-pro', 'openai-gpt-5.4-mini']);
  });

  it('puts values without dashes into an "other" group', () => {
    const enums = ['claude-sonnet-4.6', 'other'];
    const result = groupEnumOptions(enums);

    expect(result.get('claude')).toEqual(['claude-sonnet-4.6']);
    expect(result.get('other')).toEqual(['other']);
  });

  it('returns empty map for empty array', () => {
    const result = groupEnumOptions([]);
    expect(result.size).toBe(0);
  });

  it('preserves insertion order of groups', () => {
    const enums = ['gemini-3.5-flash', 'claude-sonnet-4.6', 'gemini-3.1-pro'];
    const result = groupEnumOptions(enums);
    const keys = [...result.keys()];

    expect(keys).toEqual(['gemini', 'claude']);
  });

  it('handles single-provider list', () => {
    const enums = ['claude-opus-4.7', 'claude-opus-4.6'];
    const result = groupEnumOptions(enums);

    expect(result.size).toBe(1);
    expect(result.get('claude')).toEqual(['claude-opus-4.7', 'claude-opus-4.6']);
  });
});

describe('getModelDisplayName', () => {
  it('returns human-readable name for known models', () => {
    expect(getModelDisplayName('claude-sonnet-4.6')).toBe('Sonnet 4.6');
  });

  it('returns human-readable name for provider keys', () => {
    expect(getModelDisplayName('claude')).toBe('Claude');
    expect(getModelDisplayName('openai')).toBe('OpenAI');
  });

  it('returns capitalized fallback for unknown values', () => {
    expect(getModelDisplayName('unknown-model-x')).toBe('Model-x');
  });

  it('returns capitalized value for unknown single-word values', () => {
    expect(getModelDisplayName('custom')).toBe('Custom');
  });
});
