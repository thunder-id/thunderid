// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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

    it('should replace a placeholder embedded in a longer string', () => {
      const obj = {label: '<p><a href="#" data-action-ref="{{RECOVERY_ACTION_REF}}">Reset</a></p>'};
      const replacers = [{key: 'RECOVERY_ACTION_REF', value: 'action_recovery'}];

      const [result] = updateTemplatePlaceholderReferences(obj, replacers);

      expect(result.label).toBe('<p><a href="#" data-action-ref="action_recovery">Reset</a></p>');
    });

    it('should leave a literal that reads like a replacer key unchanged', () => {
      const obj = {key: 'ID', title: 'Set the ID'};
      const replacers = [{key: 'ID', value: 'generated-id'}];

      const [result] = updateTemplatePlaceholderReferences(obj, replacers);

      expect(result.key).toBe('ID');
      expect(result.title).toBe('Set the ID');
    });

    it('should leave a runtime template literal unchanged', () => {
      const obj = {placeholder: '{{ t(signin:forms.credentials.fields.username.placeholder) }}'};
      const replacers = [{key: 'username', value: 'replaced'}];

      const [result] = updateTemplatePlaceholderReferences(obj, replacers);

      expect(result.placeholder).toBe('{{ t(signin:forms.credentials.fields.username.placeholder) }}');
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
