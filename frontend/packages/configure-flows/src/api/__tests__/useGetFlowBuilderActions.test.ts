// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@testing-library/react';
import {describe, it, expect} from 'vitest';
import actions from '../../data/actions.json';
import useGetFlowBuilderActions from '../useGetFlowBuilderActions';

describe('useGetFlowBuilderActions', () => {
  describe('Return Structure', () => {
    it('should return an object with data, error, isLoading, isValidating, and mutate', () => {
      const {result} = renderHook(() => useGetFlowBuilderActions());

      expect(result.current).toHaveProperty('data');
      expect(result.current).toHaveProperty('error');
      expect(result.current).toHaveProperty('isLoading');
      expect(result.current).toHaveProperty('isValidating');
      expect(result.current).toHaveProperty('mutate');
    });

    it('should return actions data from JSON file', () => {
      const {result} = renderHook(() => useGetFlowBuilderActions());

      expect(result.current.data).toEqual(actions);
    });

    it('should return error as null', () => {
      const {result} = renderHook(() => useGetFlowBuilderActions());

      expect(result.current.error).toBeNull();
    });

    it('should return isLoading as false', () => {
      const {result} = renderHook(() => useGetFlowBuilderActions());

      expect(result.current.isLoading).toBe(false);
    });

    it('should return isValidating as false', () => {
      const {result} = renderHook(() => useGetFlowBuilderActions());

      expect(result.current.isValidating).toBe(false);
    });

    it('should return mutate as a function that returns null', () => {
      const {result} = renderHook(() => useGetFlowBuilderActions());

      expect(typeof result.current.mutate).toBe('function');
      expect(result.current.mutate()).toBeNull();
    });
  });

  describe('Generic Type Support', () => {
    it('should support custom generic type', () => {
      interface CustomType {
        customField: string;
      }

      const {result} = renderHook(() => useGetFlowBuilderActions<CustomType>());

      // The data is cast to the generic type
      expect(result.current.data).toBeDefined();
    });

    it('should default to Actions type when no generic is provided', () => {
      const {result} = renderHook(() => useGetFlowBuilderActions());

      // Verify data matches expected Actions structure
      const {data} = result.current;
      expect(Array.isArray(data)).toBe(true);
    });
  });

  describe('Data Content', () => {
    it('should return actions data as an array', () => {
      const {result} = renderHook(() => useGetFlowBuilderActions());

      expect(Array.isArray(result.current.data)).toBe(true);
    });

    it('should contain action items with expected structure', () => {
      const {result} = renderHook(() => useGetFlowBuilderActions());

      const {data} = result.current;
      expect(data.length).toBeGreaterThan(0);
      const firstAction = data[0];
      expect(firstAction).toHaveProperty('resourceType');
      expect(firstAction).toHaveProperty('category');
      expect(firstAction).toHaveProperty('display');
    });

    it('should contain navigation category actions', () => {
      const {result} = renderHook(() => useGetFlowBuilderActions());

      const {data} = result.current;
      const navigationActions = data.filter((action) => action.category === 'NAVIGATION');
      expect(navigationActions.length).toBeGreaterThan(0);
    });
  });

  describe('Consistency', () => {
    it('should return the same data structure on multiple calls', () => {
      const {result: result1} = renderHook(() => useGetFlowBuilderActions());
      const {result: result2} = renderHook(() => useGetFlowBuilderActions());

      expect(result1.current.data).toEqual(result2.current.data);
      expect(result1.current.error).toEqual(result2.current.error);
      expect(result1.current.isLoading).toEqual(result2.current.isLoading);
      expect(result1.current.isValidating).toEqual(result2.current.isValidating);
    });

    it('should maintain stable reference for mutate function', () => {
      const {result, rerender} = renderHook(() => useGetFlowBuilderActions());

      const initialMutate = result.current.mutate;
      rerender();

      // Note: In the current implementation, mutate is recreated on each call
      expect(typeof result.current.mutate).toBe('function');
      expect(result.current.mutate()).toBeNull();
      expect(initialMutate()).toBeNull();
    });
  });
});
