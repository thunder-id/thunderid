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

import {describe, expect, it, vi, beforeEach, afterEach} from 'vitest';
import updateTemplatePlaceholderReferences from '../updateTemplatePlaceholderReferences';

describe('updateTemplatePlaceholderReferences', () => {
  beforeEach(() => {
    vi.spyOn(Math, 'random');
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe('Basic Placeholder Replacement', () => {
    it('should replace placeholder with value from replacer', () => {
      const obj = {name: '{{NAME}}'};
      const replacers = [{key: 'NAME', value: 'John'}];

      const [result] = updateTemplatePlaceholderReferences(obj, replacers);

      expect(result.name).toBe('John');
    });

    it('should replace placeholder using placeholder property', () => {
      const obj = {title: '{{TITLE}}'};
      const replacers = [{placeholder: 'TITLE', value: 'Hello World'}];

      const [result] = updateTemplatePlaceholderReferences(obj, replacers);

      expect(result.title).toBe('Hello World');
    });

    it('should keep original value when no replacer matches', () => {
      const obj = {field: '{{UNKNOWN}}'};
      const replacers = [{key: 'OTHER', value: 'test'}];

      const [result] = updateTemplatePlaceholderReferences(obj, replacers);

      expect(result.field).toBe('{{UNKNOWN}}');
    });

    it('should keep original value when replacer has no value', () => {
      const obj = {field: '{{NAME}}'};
      const replacers = [{key: 'NAME'}];

      const [result] = updateTemplatePlaceholderReferences(obj, replacers);

      expect(result.field).toBe('{{NAME}}');
    });
  });

  describe('ID Type Replacer', () => {
    it('should generate resource ID for type=ID replacers', () => {
      vi.mocked(Math.random).mockReturnValue(0.5);

      const obj = {id: '{{COMPONENT_ID}}'};
      const replacers = [{key: 'COMPONENT_ID', type: 'ID'}];

      const [result] = updateTemplatePlaceholderReferences(obj, replacers);

      expect(result.id).toMatch(/^ID_/);
    });

    it('should use the replacer prefix for generated ids so nodes are tellable apart', () => {
      vi.mocked(Math.random).mockReturnValue(0.5);

      const obj = {id: '{{RECOVERY_CALL_STEP_ID}}'};
      const replacers = [{key: 'RECOVERY_CALL_STEP_ID', type: 'ID', prefix: 'recovery_call'}];

      const [result] = updateTemplatePlaceholderReferences(obj, replacers);

      expect(result.id).toMatch(/^recovery_call_/);
    });
  });

  describe('Nested Objects', () => {
    it('should replace placeholders in nested objects', () => {
      const obj = {
        user: {
          name: '{{NAME}}',
          email: '{{EMAIL}}',
        },
      };
      const replacers = [
        {key: 'NAME', value: 'Alice'},
        {key: 'EMAIL', value: 'alice@test.com'},
      ];

      const [result] = updateTemplatePlaceholderReferences(obj, replacers);

      expect(result.user.name).toBe('Alice');
      expect(result.user.email).toBe('alice@test.com');
    });

    it('should replace placeholders in deeply nested objects', () => {
      const obj = {
        level1: {
          level2: {
            level3: {
              value: '{{DEEP}}',
            },
          },
        },
      };
      const replacers = [{key: 'DEEP', value: 'found'}];

      const [result] = updateTemplatePlaceholderReferences(obj, replacers);

      expect(result.level1.level2.level3.value).toBe('found');
    });
  });

  describe('Arrays', () => {
    it('should replace placeholders in arrays', () => {
      const obj = [{name: '{{NAME1}}'}, {name: '{{NAME2}}'}];
      const replacers = [
        {key: 'NAME1', value: 'First'},
        {key: 'NAME2', value: 'Second'},
      ];

      const [result] = updateTemplatePlaceholderReferences(obj, replacers);

      expect(result[0].name).toBe('First');
      expect(result[1].name).toBe('Second');
    });

    it('should replace placeholders in nested arrays', () => {
      const obj = {
        items: [{label: '{{LABEL}}'}],
      };
      const replacers = [{key: 'LABEL', value: 'Test Label'}];

      const [result] = updateTemplatePlaceholderReferences(obj, replacers);

      expect(result.items[0].label).toBe('Test Label');
    });
  });

  describe('Placeholder Cache', () => {
    it('should return placeholder cache as second element', () => {
      const obj = {name: '{{NAME}}'};
      const replacers = [{key: 'NAME', value: 'Test'}];

      const [, cache] = updateTemplatePlaceholderReferences(obj, replacers);

      expect(cache).toBeInstanceOf(Map);
      expect(cache.get('NAME')).toBe('Test');
    });

    it('should reuse cached value for duplicate placeholders', () => {
      vi.mocked(Math.random).mockReturnValue(0.5);

      const obj = {
        id1: '{{ID}}',
        id2: '{{ID}}',
      };
      const replacers = [{key: 'ID', type: 'ID'}];

      const [result, cache] = updateTemplatePlaceholderReferences(obj, replacers);

      expect(result.id1).toBe(result.id2);
      expect(cache.size).toBe(1);
    });

    it('should cache multiple different placeholders', () => {
      const obj = {
        a: '{{A}}',
        b: '{{B}}',
        c: '{{C}}',
      };
      const replacers = [
        {key: 'A', value: 'value-a'},
        {key: 'B', value: 'value-b'},
        {key: 'C', value: 'value-c'},
      ];

      const [, cache] = updateTemplatePlaceholderReferences(obj, replacers);

      expect(cache.size).toBe(3);
      expect(cache.get('A')).toBe('value-a');
      expect(cache.get('B')).toBe('value-b');
      expect(cache.get('C')).toBe('value-c');
    });
  });

  describe('Non-Placeholder Values', () => {
    it('should preserve non-string values', () => {
      const obj = {
        count: 42,
        enabled: true,
        data: null,
      };
      const replacers = [{key: 'TEST', value: 'test'}];

      const [result] = updateTemplatePlaceholderReferences(obj, replacers);

      expect(result.count).toBe(42);
      expect(result.enabled).toBe(true);
      expect(result.data).toBeNull();
    });

    it('should preserve regular strings', () => {
      const obj = {
        title: 'Regular string',
        placeholder: '{{NAME}}',
      };
      const replacers = [{key: 'NAME', value: 'Test'}];

      const [result] = updateTemplatePlaceholderReferences(obj, replacers);

      expect(result.title).toBe('Regular string');
      expect(result.placeholder).toBe('Test');
    });
  });

  describe('Edge Cases', () => {
    it('should handle empty object', () => {
      const obj = {};
      const replacers = [{key: 'TEST', value: 'test'}];

      const [result] = updateTemplatePlaceholderReferences(obj, replacers);

      expect(result).toEqual({});
    });

    it('should handle empty array', () => {
      const obj: unknown[] = [];
      const replacers = [{key: 'TEST', value: 'test'}];

      const [result] = updateTemplatePlaceholderReferences(obj, replacers);

      expect(result).toEqual([]);
    });

    it('should handle empty replacers array', () => {
      const obj = {name: '{{NAME}}'};
      const replacers: {key: string; value: string}[] = [];

      const [result] = updateTemplatePlaceholderReferences(obj, replacers);

      expect(result.name).toBe('{{NAME}}');
    });

    it('should handle primitive values', () => {
      const [stringResult] = updateTemplatePlaceholderReferences('hello', []);

      expect(stringResult).toBe('hello');

      const [numberResult] = updateTemplatePlaceholderReferences(42, []);

      expect(numberResult).toBe(42);

      const [boolResult] = updateTemplatePlaceholderReferences(true, []);

      expect(boolResult).toBe(true);
    });

    it('should handle null input', () => {
      const [result] = updateTemplatePlaceholderReferences(null, []);

      expect(result).toBeNull();
    });
  });

  describe('Type Preservation', () => {
    it('should preserve generic type', () => {
      interface Config {
        name: string;
        value: number;
      }

      const obj: Config = {name: '{{NAME}}', value: 100};
      const replacers = [{key: 'NAME', value: 'Test'}];

      const [result] = updateTemplatePlaceholderReferences<Config>(obj, replacers);

      expect(result.name).toBe('Test');
      expect(result.value).toBe(100);
    });
  });
});
