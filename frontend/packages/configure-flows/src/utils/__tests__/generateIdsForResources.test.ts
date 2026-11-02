// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it, vi, beforeEach, afterEach} from 'vitest';
import generateIdsForResources from '../generateIdsForResources';

describe('generateIdsForResources', () => {
  beforeEach(() => {
    vi.spyOn(Math, 'random');
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe('Basic Functionality', () => {
    it('should replace placeholder ID with generated ID', () => {
      const resources = {id: '{{ID}}', type: 'button'};
      const result = generateIdsForResources(resources);

      expect(result.id).not.toBe('{{ID}}');
      expect(result.id).toMatch(/^button_[a-z0-9]+$/);
    });

    it('should use default matcher "ID"', () => {
      const resources = {id: '{{ID}}'};
      const result = generateIdsForResources(resources);

      expect(result.id).not.toBe('{{ID}}');
    });

    it('should support custom matcher', () => {
      const resources = {id: '{{CUSTOM}}'};
      const result = generateIdsForResources(resources, 'CUSTOM');

      expect(result.id).not.toBe('{{CUSTOM}}');
    });

    it('should not replace non-matching placeholders', () => {
      const resources = {id: '{{OTHER}}'};
      const result = generateIdsForResources(resources);

      expect(result.id).toBe('{{OTHER}}');
    });
  });

  describe('Type-based ID Generation', () => {
    it('should use type property in lowercase for ID prefix', () => {
      vi.mocked(Math.random).mockReturnValue(0.5);

      const resources = {id: '{{ID}}', type: 'BUTTON'};
      const result = generateIdsForResources(resources);

      expect(result.id).toMatch(/^button_/);
    });

    it('should use "component" as default prefix when type is missing', () => {
      vi.mocked(Math.random).mockReturnValue(0.5);

      const resources = {id: '{{ID}}'};
      const result = generateIdsForResources(resources);

      expect(result.id).toMatch(/^component_/);
    });

    it('should use "component" as default prefix when type is not a string', () => {
      vi.mocked(Math.random).mockReturnValue(0.5);

      const resources = {id: '{{ID}}', type: 123};
      const result = generateIdsForResources(resources);

      expect(result.id).toMatch(/^component_/);
    });
  });

  describe('Nested Objects', () => {
    it('should replace IDs in nested objects', () => {
      const resources = {
        id: '{{ID}}',
        type: 'container',
        children: {
          id: '{{ID}}',
          type: 'button',
        },
      };
      const result = generateIdsForResources(resources);

      expect(result.id).toMatch(/^container_/);
      expect(result.children.id).toMatch(/^button_/);
    });

    it('should replace IDs in deeply nested objects', () => {
      const resources = {
        level1: {
          level2: {
            level3: {
              id: '{{ID}}',
              type: 'input',
            },
          },
        },
      };
      const result = generateIdsForResources(resources);

      expect(result.level1.level2.level3.id).toMatch(/^input_/);
    });
  });

  describe('Arrays', () => {
    it('should replace IDs in arrays', () => {
      const resources = [
        {id: '{{ID}}', type: 'button'},
        {id: '{{ID}}', type: 'input'},
      ];
      const result = generateIdsForResources(resources);

      expect(result[0].id).toMatch(/^button_/);
      expect(result[1].id).toMatch(/^input_/);
    });

    it('should replace IDs in nested arrays', () => {
      const resources = {
        components: [
          {id: '{{ID}}', type: 'text'},
          {id: '{{ID}}', type: 'link'},
        ],
      };
      const result = generateIdsForResources(resources);

      expect(result.components[0].id).toMatch(/^text_/);
      expect(result.components[1].id).toMatch(/^link_/);
    });

    it('should handle arrays of primitives', () => {
      const resources = {
        tags: ['tag1', 'tag2'],
        id: '{{ID}}',
        type: 'component',
      };
      const result = generateIdsForResources(resources);

      expect(result.tags).toEqual(['tag1', 'tag2']);
      expect(result.id).toMatch(/^component_/);
    });
  });

  describe('Primitive Values', () => {
    it('should return strings unchanged', () => {
      const result = generateIdsForResources('hello');

      expect(result).toBe('hello');
    });

    it('should return numbers unchanged', () => {
      const result = generateIdsForResources(42);

      expect(result).toBe(42);
    });

    it('should return booleans unchanged', () => {
      const result = generateIdsForResources(true);

      expect(result).toBe(true);
    });

    it('should return null unchanged', () => {
      const result = generateIdsForResources(null);

      expect(result).toBeNull();
    });
  });

  describe('Non-ID Properties', () => {
    it('should preserve other properties', () => {
      const resources = {
        id: '{{ID}}',
        type: 'button',
        label: 'Click me',
        disabled: false,
        count: 5,
      };
      const result = generateIdsForResources(resources);

      expect(result.type).toBe('button');
      expect(result.label).toBe('Click me');
      expect(result.disabled).toBe(false);
      expect(result.count).toBe(5);
    });

    it('should not replace non-id properties with placeholder pattern', () => {
      const resources = {
        id: '{{ID}}',
        type: 'button',
        template: '{{ID}}',
      };
      const result = generateIdsForResources(resources);

      expect(result.template).toBe('{{ID}}');
    });
  });

  describe('Edge Cases', () => {
    it('should handle empty objects', () => {
      const resources = {};
      const result = generateIdsForResources(resources);

      expect(result).toEqual({});
    });

    it('should handle empty arrays', () => {
      const resources: unknown[] = [];
      const result = generateIdsForResources(resources);

      expect(result).toEqual([]);
    });

    it('should handle objects without id property', () => {
      const resources = {name: 'test', value: 123};
      const result = generateIdsForResources(resources);

      expect(result).toEqual({name: 'test', value: 123});
    });

    it('should handle id property that is not a placeholder', () => {
      const resources = {id: 'existing-id', type: 'button'};
      const result = generateIdsForResources(resources);

      expect(result.id).toBe('existing-id');
    });

    it('should generate unique IDs for multiple placeholders', () => {
      const resources = [
        {id: '{{ID}}', type: 'button'},
        {id: '{{ID}}', type: 'button'},
        {id: '{{ID}}', type: 'button'},
      ];
      const result = generateIdsForResources(resources);

      const ids = result.map((r: {id: string}) => r.id);
      const uniqueIds = new Set(ids);

      expect(uniqueIds.size).toBe(3);
    });
  });

  describe('Type Preservation', () => {
    it('should preserve TypeScript generic type', () => {
      interface Resource {
        id: string;
        type: string;
        name: string;
      }

      const resources: Resource = {id: '{{ID}}', type: 'button', name: 'Test'};
      const result = generateIdsForResources<Resource>(resources);

      expect(result.name).toBe('Test');
      expect(result.id).toMatch(/^button_/);
    });
  });
});
