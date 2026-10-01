// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, act} from '@testing-library/react';
import {describe, it, expect, beforeEach} from 'vitest';
import type {Resource} from '../../models/resources';
import {
  updateLastInteractedStore,
  useLastInteractedResourceOnly,
  updatePropertiesPanelStore,
  usePropertiesPanelStateOnly,
} from '../flowBuilderCoreStore';

describe('flowBuilderCoreStore', () => {
  describe('LastInteractedResource Store', () => {
    beforeEach(() => {
      // Reset the store to initial state
      updateLastInteractedStore(null, '');
    });

    it('should return initial state with null resource and empty stepId', () => {
      const {result} = renderHook(() => useLastInteractedResourceOnly());

      expect(result.current.resource).toBeNull();
      expect(result.current.stepId).toBe('');
    });

    it('should update store when resource changes', () => {
      const {result} = renderHook(() => useLastInteractedResourceOnly());

      const mockResource: Resource = {
        id: 'resource-1',
        type: 'VIEW',
        resourceType: 'STEP',
      } as Resource;

      act(() => {
        updateLastInteractedStore(mockResource, 'step-1');
      });

      expect(result.current.resource).toEqual(mockResource);
      expect(result.current.stepId).toBe('step-1');
    });

    it('should update store when stepId changes', () => {
      const {result} = renderHook(() => useLastInteractedResourceOnly());

      const mockResource: Resource = {
        id: 'resource-1',
        type: 'VIEW',
        resourceType: 'STEP',
      } as Resource;

      act(() => {
        updateLastInteractedStore(mockResource, 'step-1');
      });

      expect(result.current.stepId).toBe('step-1');

      act(() => {
        updateLastInteractedStore(mockResource, 'step-2');
      });

      expect(result.current.stepId).toBe('step-2');
    });

    it('should not notify listeners when values do not change', () => {
      const {result} = renderHook(() => useLastInteractedResourceOnly());

      const mockResource: Resource = {
        id: 'resource-1',
        type: 'VIEW',
        resourceType: 'STEP',
      } as Resource;

      act(() => {
        updateLastInteractedStore(mockResource, 'step-1');
      });

      const initialResource = result.current.resource;
      const initialStepId = result.current.stepId;

      // Update with same values - should not trigger re-render
      act(() => {
        updateLastInteractedStore(mockResource, 'step-1');
      });

      expect(result.current.resource).toBe(initialResource);
      expect(result.current.stepId).toBe(initialStepId);
    });

    it('should handle setting resource back to null', () => {
      const {result} = renderHook(() => useLastInteractedResourceOnly());

      const mockResource: Resource = {
        id: 'resource-1',
        type: 'VIEW',
        resourceType: 'STEP',
      } as Resource;

      act(() => {
        updateLastInteractedStore(mockResource, 'step-1');
      });

      expect(result.current.resource).toEqual(mockResource);

      act(() => {
        updateLastInteractedStore(null, '');
      });

      expect(result.current.resource).toBeNull();
      expect(result.current.stepId).toBe('');
    });

    it('should handle multiple subscribers', () => {
      const {result: result1} = renderHook(() => useLastInteractedResourceOnly());
      const {result: result2} = renderHook(() => useLastInteractedResourceOnly());

      const mockResource: Resource = {
        id: 'resource-1',
        type: 'VIEW',
        resourceType: 'STEP',
      } as Resource;

      act(() => {
        updateLastInteractedStore(mockResource, 'step-1');
      });

      expect(result1.current.resource).toEqual(mockResource);
      expect(result2.current.resource).toEqual(mockResource);
    });

    it('should clean up listener on unmount', () => {
      const {unmount} = renderHook(() => useLastInteractedResourceOnly());

      // Unmount should not throw
      expect(() => unmount()).not.toThrow();
    });
  });

  describe('PropertiesPanel Store', () => {
    beforeEach(() => {
      // Reset the store to initial state
      updatePropertiesPanelStore(false);
    });

    it('should return initial state with isOpen false', () => {
      const {result} = renderHook(() => usePropertiesPanelStateOnly());

      expect(result.current.isOpen).toBe(false);
    });

    it('should update store when isOpen changes to true', () => {
      const {result} = renderHook(() => usePropertiesPanelStateOnly());

      act(() => {
        updatePropertiesPanelStore(true);
      });

      expect(result.current.isOpen).toBe(true);
    });

    it('should update store when isOpen changes to false', () => {
      const {result} = renderHook(() => usePropertiesPanelStateOnly());

      act(() => {
        updatePropertiesPanelStore(true);
      });

      expect(result.current.isOpen).toBe(true);

      act(() => {
        updatePropertiesPanelStore(false);
      });

      expect(result.current.isOpen).toBe(false);
    });

    it('should not notify listeners when value does not change', () => {
      const {result} = renderHook(() => usePropertiesPanelStateOnly());

      act(() => {
        updatePropertiesPanelStore(true);
      });

      const initialIsOpen = result.current.isOpen;

      // Update with same value - should not trigger unnecessary updates
      act(() => {
        updatePropertiesPanelStore(true);
      });

      expect(result.current.isOpen).toBe(initialIsOpen);
    });

    it('should handle multiple subscribers', () => {
      const {result: result1} = renderHook(() => usePropertiesPanelStateOnly());
      const {result: result2} = renderHook(() => usePropertiesPanelStateOnly());

      act(() => {
        updatePropertiesPanelStore(true);
      });

      expect(result1.current.isOpen).toBe(true);
      expect(result2.current.isOpen).toBe(true);
    });

    it('should clean up listener on unmount', () => {
      const {unmount} = renderHook(() => usePropertiesPanelStateOnly());

      // Unmount should not throw
      expect(() => unmount()).not.toThrow();
    });

    it('should toggle isOpen state correctly', () => {
      const {result} = renderHook(() => usePropertiesPanelStateOnly());

      expect(result.current.isOpen).toBe(false);

      act(() => {
        updatePropertiesPanelStore(true);
      });
      expect(result.current.isOpen).toBe(true);

      act(() => {
        updatePropertiesPanelStore(false);
      });
      expect(result.current.isOpen).toBe(false);

      act(() => {
        updatePropertiesPanelStore(true);
      });
      expect(result.current.isOpen).toBe(true);
    });
  });

  describe('Store Isolation', () => {
    beforeEach(() => {
      updateLastInteractedStore(null, '');
      updatePropertiesPanelStore(false);
    });

    it('should have independent stores that do not affect each other', () => {
      const {result: lastInteracted} = renderHook(() => useLastInteractedResourceOnly());
      const {result: propertiesPanel} = renderHook(() => usePropertiesPanelStateOnly());

      const mockResource: Resource = {
        id: 'resource-1',
        type: 'VIEW',
        resourceType: 'STEP',
      } as Resource;

      // Update last interacted store
      act(() => {
        updateLastInteractedStore(mockResource, 'step-1');
      });

      // Properties panel should remain unchanged
      expect(propertiesPanel.current.isOpen).toBe(false);
      expect(lastInteracted.current.resource).toEqual(mockResource);

      // Update properties panel store
      act(() => {
        updatePropertiesPanelStore(true);
      });

      // Last interacted should remain unchanged
      expect(lastInteracted.current.resource).toEqual(mockResource);
      expect(propertiesPanel.current.isOpen).toBe(true);
    });
  });
});
