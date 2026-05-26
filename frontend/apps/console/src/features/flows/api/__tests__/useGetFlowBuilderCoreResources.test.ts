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

import {renderHook} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import elements from '../../data/elements.json';
import steps from '../../data/steps.json';
import useGetFlowBuilderCoreResources from '../useGetFlowBuilderCoreResources';

const TEST_PRODUCT_NAME = 'TestProduct';

// Mock useConfig to avoid ConfigProvider requirement.
vi.mock('@thunderid/contexts', async (importOriginal) => {
  // eslint-disable-next-line @typescript-eslint/no-unnecessary-type-assertion
  const actual = (await importOriginal()) as Record<string, unknown>;

  return {
    ...actual,
    useConfig: () => ({
      config: {
        brand: {
          product_name: TEST_PRODUCT_NAME,
        },
      },
    }),
  };
});

describe('useGetFlowBuilderCoreResources', () => {
  describe('Return Structure', () => {
    it('should return an object with data, error, isLoading, isValidating, and mutate', () => {
      const {result} = renderHook(() => useGetFlowBuilderCoreResources());

      expect(result.current).toHaveProperty('data');
      expect(result.current).toHaveProperty('error');
      expect(result.current).toHaveProperty('isLoading');
      expect(result.current).toHaveProperty('isValidating');
      expect(result.current).toHaveProperty('mutate');
    });

    it('should return error as null', () => {
      const {result} = renderHook(() => useGetFlowBuilderCoreResources());

      expect(result.current.error).toBeNull();
    });

    it('should return isLoading as false', () => {
      const {result} = renderHook(() => useGetFlowBuilderCoreResources());

      expect(result.current.isLoading).toBe(false);
    });

    it('should return isValidating as false', () => {
      const {result} = renderHook(() => useGetFlowBuilderCoreResources());

      expect(result.current.isValidating).toBe(false);
    });

    it('should return mutate as a function that returns null', () => {
      const {result} = renderHook(() => useGetFlowBuilderCoreResources());

      expect(typeof result.current.mutate).toBe('function');
      expect(result.current.mutate()).toBeNull();
    });
  });

  describe('Data Content', () => {
    it('should return data containing elements from JSON file', () => {
      const {result} = renderHook(() => useGetFlowBuilderCoreResources());

      const {data} = result.current;
      expect(data.elements).toEqual(elements);
    });

    it('should return data containing steps from JSON file', () => {
      const {result} = renderHook(() => useGetFlowBuilderCoreResources());

      const {data} = result.current;
      expect(data.steps).toEqual(steps);
    });

    it('should return data containing templates resolved from JSON file', () => {
      const {result} = renderHook(() => useGetFlowBuilderCoreResources());

      const {data} = result.current;
      expect(Array.isArray(data.templates)).toBe(true);
      expect(data.templates.length).toBeGreaterThan(0);
    });

    it('should return data containing widgets resolved from JSON file', () => {
      const {result} = renderHook(() => useGetFlowBuilderCoreResources());

      const {data} = result.current;
      expect(Array.isArray(data.widgets)).toBe(true);
    });

    it('should return all resource types in data object', () => {
      const {result} = renderHook(() => useGetFlowBuilderCoreResources());

      const {data} = result.current;
      expect(data).toHaveProperty('elements');
      expect(data).toHaveProperty('steps');
      expect(data).toHaveProperty('templates');
      expect(data).toHaveProperty('widgets');
    });
  });

  describe('Brand Placeholder Substitution', () => {
    it('should substitute {{productName}} placeholders with the configured product name', () => {
      const {result} = renderHook(() => useGetFlowBuilderCoreResources());

      const serialised = JSON.stringify(result.current.data);

      expect(serialised).not.toContain('{{productName}}');
      expect(serialised).toContain(TEST_PRODUCT_NAME);
    });
  });

  describe('Generic Type Support', () => {
    it('should support custom generic type', () => {
      interface CustomResourceType {
        customField: string;
      }

      const {result} = renderHook(() => useGetFlowBuilderCoreResources<CustomResourceType>());

      // The data is cast to the generic type
      expect(result.current.data).toBeDefined();
    });

    it('should default to Resources type when no generic is provided', () => {
      const {result} = renderHook(() => useGetFlowBuilderCoreResources());

      // Verify data matches expected Resources structure
      const {data} = result.current;
      expect(data.elements).toBeDefined();
      expect(data.steps).toBeDefined();
      expect(data.widgets).toBeDefined();
      expect(data.templates).toBeDefined();
    });
  });

  describe('Memoization', () => {
    it('should return memoized data on subsequent renders', () => {
      const {result, rerender} = renderHook(() => useGetFlowBuilderCoreResources());

      const initialData = result.current.data;
      rerender();

      // Due to useMemo, the data reference should be stable
      expect(result.current.data).toBe(initialData);
    });

    it('should maintain stable data reference across multiple rerenders', () => {
      const {result, rerender} = renderHook(() => useGetFlowBuilderCoreResources());

      const firstData = result.current.data;
      rerender();
      const secondData = result.current.data;
      rerender();
      const thirdData = result.current.data;

      expect(firstData).toBe(secondData);
      expect(secondData).toBe(thirdData);
    });
  });

  describe('Data Arrays', () => {
    it('should return elements as an array', () => {
      const {result} = renderHook(() => useGetFlowBuilderCoreResources());

      const {data} = result.current;
      expect(Array.isArray(data.elements)).toBe(true);
    });

    it('should return steps as an array', () => {
      const {result} = renderHook(() => useGetFlowBuilderCoreResources());

      const {data} = result.current;
      expect(Array.isArray(data.steps)).toBe(true);
    });

    it('should return templates as an array', () => {
      const {result} = renderHook(() => useGetFlowBuilderCoreResources());

      const {data} = result.current;
      expect(Array.isArray(data.templates)).toBe(true);
    });

    it('should return widgets as an array', () => {
      const {result} = renderHook(() => useGetFlowBuilderCoreResources());

      const {data} = result.current;
      expect(Array.isArray(data.widgets)).toBe(true);
    });
  });

  describe('Element Resource Types', () => {
    it('should contain ELEMENT resource type items in elements array', () => {
      const {result} = renderHook(() => useGetFlowBuilderCoreResources());

      const {data} = result.current;
      expect(data.elements.length).toBeGreaterThan(0);
      const firstElement = data.elements[0];
      expect(firstElement).toHaveProperty('resourceType', 'ELEMENT');
    });

    it('should contain elements with display property', () => {
      const {result} = renderHook(() => useGetFlowBuilderCoreResources());

      const {data} = result.current;
      expect(data.elements.length).toBeGreaterThan(0);
      const firstElement = data.elements[0];
      expect(firstElement).toHaveProperty('display');
    });
  });

  describe('Step Resource Types', () => {
    it('should contain STEP resource type items in steps array', () => {
      const {result} = renderHook(() => useGetFlowBuilderCoreResources());

      const {data} = result.current;
      expect(data.steps.length).toBeGreaterThan(0);
      const firstStep = data.steps[0];
      expect(firstStep).toHaveProperty('resourceType', 'STEP');
    });

    it('should contain steps with type property', () => {
      const {result} = renderHook(() => useGetFlowBuilderCoreResources());

      const {data} = result.current;
      expect(data.steps.length).toBeGreaterThan(0);
      const firstStep = data.steps[0];
      expect(firstStep).toHaveProperty('type');
    });
  });

  describe('Consistency', () => {
    it('should return consistent data across multiple hook instances', () => {
      const {result: result1} = renderHook(() => useGetFlowBuilderCoreResources());
      const {result: result2} = renderHook(() => useGetFlowBuilderCoreResources());

      // Data content should be equal (though not necessarily the same reference across different hook instances)
      expect(result1.current.data).toEqual(result2.current.data);
      expect(result1.current.error).toEqual(result2.current.error);
      expect(result1.current.isLoading).toEqual(result2.current.isLoading);
      expect(result1.current.isValidating).toEqual(result2.current.isValidating);
    });
  });
});
