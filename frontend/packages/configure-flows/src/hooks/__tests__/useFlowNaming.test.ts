// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, act} from '@testing-library/react';
import {describe, it, expect} from 'vitest';
import useFlowNaming from '../useFlowNaming';

describe('useFlowNaming', () => {
  describe('Hook Interface', () => {
    it('should return flowName', () => {
      const {result} = renderHook(() => useFlowNaming());
      expect(result.current.flowName).toBeDefined();
      expect(typeof result.current.flowName).toBe('string');
    });

    it('should return flowHandle', () => {
      const {result} = renderHook(() => useFlowNaming());
      expect(result.current.flowHandle).toBeDefined();
      expect(typeof result.current.flowHandle).toBe('string');
    });

    it('should return needsAutoLayout', () => {
      const {result} = renderHook(() => useFlowNaming());
      expect(typeof result.current.needsAutoLayout).toBe('boolean');
    });

    it('should return setNeedsAutoLayout function', () => {
      const {result} = renderHook(() => useFlowNaming());
      expect(typeof result.current.setNeedsAutoLayout).toBe('function');
    });

    it('should return handleFlowNameChange function', () => {
      const {result} = renderHook(() => useFlowNaming());
      expect(typeof result.current.handleFlowNameChange).toBe('function');
    });
  });

  describe('Default values', () => {
    it('should use default name when no props provided', () => {
      const {result} = renderHook(() => useFlowNaming());
      expect(result.current.flowName).toBe('Login Flow');
    });

    it('should use default handle when no props provided', () => {
      const {result} = renderHook(() => useFlowNaming());
      expect(result.current.flowHandle).toBe('login-flow');
    });

    it('should use custom default name when provided', () => {
      const {result} = renderHook(() => useFlowNaming({defaultName: 'Custom Flow'}));
      expect(result.current.flowName).toBe('Custom Flow');
    });

    it('should use custom default handle when provided', () => {
      const {result} = renderHook(() => useFlowNaming({defaultHandle: 'custom-handle'}));
      expect(result.current.flowHandle).toBe('custom-handle');
    });

    it('should initialize needsAutoLayout to false', () => {
      const {result} = renderHook(() => useFlowNaming());
      expect(result.current.needsAutoLayout).toBe(false);
    });
  });

  describe('handleFlowNameChange', () => {
    it('should update flowName when called', () => {
      const {result} = renderHook(() => useFlowNaming());

      act(() => {
        result.current.handleFlowNameChange('New Flow Name');
      });

      expect(result.current.flowName).toBe('New Flow Name');
    });

    it('should generate handle from name', () => {
      const {result} = renderHook(() => useFlowNaming());

      act(() => {
        result.current.handleFlowNameChange('My Custom Flow');
      });

      expect(result.current.flowHandle).toBe('my-custom-flow');
    });

    it('should convert name to lowercase for handle', () => {
      const {result} = renderHook(() => useFlowNaming());

      act(() => {
        result.current.handleFlowNameChange('UPPERCASE NAME');
      });

      expect(result.current.flowHandle).toBe('uppercase-name');
    });

    it('should replace spaces with hyphens in handle', () => {
      const {result} = renderHook(() => useFlowNaming());

      act(() => {
        result.current.handleFlowNameChange('Multiple   Spaces   Here');
      });

      expect(result.current.flowHandle).toBe('multiple-spaces-here');
    });

    it('should remove special characters from handle', () => {
      const {result} = renderHook(() => useFlowNaming());

      act(() => {
        result.current.handleFlowNameChange('Flow@Name#With$Special!Characters');
      });

      expect(result.current.flowHandle).toBe('flownamewithspecialcharacters');
    });

    it('should remove leading and trailing hyphens', () => {
      const {result} = renderHook(() => useFlowNaming());

      act(() => {
        result.current.handleFlowNameChange(' - Trimmed Name - ');
      });

      expect(result.current.flowHandle).toBe('trimmed-name');
    });

    it('should collapse multiple hyphens into one', () => {
      const {result} = renderHook(() => useFlowNaming());

      act(() => {
        result.current.handleFlowNameChange('Name---With---Multiple---Hyphens');
      });

      expect(result.current.flowHandle).toBe('name-with-multiple-hyphens');
    });
  });

  describe('setNeedsAutoLayout', () => {
    it('should update needsAutoLayout when called', () => {
      const {result} = renderHook(() => useFlowNaming());

      expect(result.current.needsAutoLayout).toBe(false);

      act(() => {
        result.current.setNeedsAutoLayout(true);
      });

      expect(result.current.needsAutoLayout).toBe(true);
    });

    it('should allow toggling needsAutoLayout', () => {
      const {result} = renderHook(() => useFlowNaming());

      act(() => {
        result.current.setNeedsAutoLayout(true);
      });
      expect(result.current.needsAutoLayout).toBe(true);

      act(() => {
        result.current.setNeedsAutoLayout(false);
      });
      expect(result.current.needsAutoLayout).toBe(false);
    });
  });

  describe('existingFlowData synchronization', () => {
    it('should sync flowName from existingFlowData', () => {
      const {result} = renderHook(() =>
        useFlowNaming({
          existingFlowData: {
            name: 'Existing Flow Name',
            handle: 'existing-handle',
          },
        }),
      );

      expect(result.current.flowName).toBe('Existing Flow Name');
    });

    it('should sync flowHandle from existingFlowData', () => {
      const {result} = renderHook(() =>
        useFlowNaming({
          existingFlowData: {
            name: 'Existing Flow Name',
            handle: 'existing-handle',
          },
        }),
      );

      expect(result.current.flowHandle).toBe('existing-handle');
    });

    it('should generate handle from name when existingFlowData has name but no handle', () => {
      const {result} = renderHook(() =>
        useFlowNaming({
          existingFlowData: {
            name: 'Flow Without Handle',
            // No handle provided
          },
        }),
      );

      // Should generate handle from name
      expect(result.current.flowHandle).toBe('flow-without-handle');
    });

    it('should handle existingFlowData with only handle', () => {
      const {result} = renderHook(() =>
        useFlowNaming({
          existingFlowData: {
            handle: 'only-handle',
            // No name provided
          },
        }),
      );

      // Should use the provided handle
      expect(result.current.flowHandle).toBe('only-handle');
      // Name should remain as default
      expect(result.current.flowName).toBe('Login Flow');
    });

    it('should handle empty existingFlowData', () => {
      const {result} = renderHook(() =>
        useFlowNaming({
          existingFlowData: {},
        }),
      );

      expect(result.current.flowName).toBe('Login Flow');
      expect(result.current.flowHandle).toBe('login-flow');
    });

    it('should update when existingFlowData changes', () => {
      const {result, rerender} = renderHook(({existingFlowData}) => useFlowNaming({existingFlowData}), {
        initialProps: {
          existingFlowData: {
            name: 'Initial Name',
            handle: 'initial-handle',
          },
        },
      });

      expect(result.current.flowName).toBe('Initial Name');
      expect(result.current.flowHandle).toBe('initial-handle');

      rerender({
        existingFlowData: {
          name: 'Updated Name',
          handle: 'updated-handle',
        },
      });

      expect(result.current.flowName).toBe('Updated Name');
      expect(result.current.flowHandle).toBe('updated-handle');
    });

    it('should generate handle from complex name when no handle provided', () => {
      const {result} = renderHook(() =>
        useFlowNaming({
          existingFlowData: {
            name: 'Complex Flow Name With UPPERCASE and Special@Chars!',
            // No handle - should be generated
          },
        }),
      );

      // Handle should be generated from the name
      expect(result.current.flowHandle).toBe('complex-flow-name-with-uppercase-and-specialchars');
    });

    it('should prefer explicit handle over generated one', () => {
      const {result} = renderHook(() =>
        useFlowNaming({
          existingFlowData: {
            name: 'Different Name',
            handle: 'explicit-handle',
          },
        }),
      );

      // Should use the explicit handle, not generate from name
      expect(result.current.flowHandle).toBe('explicit-handle');
    });
  });

  describe('generateHandleFromName edge cases', () => {
    it('should handle empty string', () => {
      const {result} = renderHook(() => useFlowNaming());

      act(() => {
        result.current.handleFlowNameChange('');
      });

      expect(result.current.flowHandle).toBe('');
    });

    it('should handle string with only special characters', () => {
      const {result} = renderHook(() => useFlowNaming());

      act(() => {
        result.current.handleFlowNameChange('@#$%^&*()');
      });

      expect(result.current.flowHandle).toBe('');
    });

    it('should handle string with only spaces', () => {
      const {result} = renderHook(() => useFlowNaming());

      act(() => {
        result.current.handleFlowNameChange('     ');
      });

      expect(result.current.flowHandle).toBe('');
    });

    it('should handle numbers in name', () => {
      const {result} = renderHook(() => useFlowNaming());

      act(() => {
        result.current.handleFlowNameChange('Flow 123 Test');
      });

      expect(result.current.flowHandle).toBe('flow-123-test');
    });

    it('should trim whitespace from name before generating handle', () => {
      const {result} = renderHook(() => useFlowNaming());

      act(() => {
        result.current.handleFlowNameChange('  Trimmed Flow  ');
      });

      expect(result.current.flowHandle).toBe('trimmed-flow');
    });
  });

  describe('handle immutability for existing flows', () => {
    it('should not change handle when renaming an existing flow', () => {
      const existingFlowData = {
        name: 'Original Name',
        handle: 'original-handle',
      };

      const {result} = renderHook(() => useFlowNaming({existingFlowData}));

      expect(result.current.flowHandle).toBe('original-handle');

      act(() => {
        result.current.handleFlowNameChange('Renamed Flow');
      });

      expect(result.current.flowName).toBe('Renamed Flow');
      expect(result.current.flowHandle).toBe('original-handle');
    });

    it('should still update handle for new flows', () => {
      const {result} = renderHook(() => useFlowNaming());

      act(() => {
        result.current.handleFlowNameChange('Brand New Flow');
      });

      expect(result.current.flowName).toBe('Brand New Flow');
      expect(result.current.flowHandle).toBe('brand-new-flow');
    });
  });

  describe('undefined props handling', () => {
    it('should handle undefined props gracefully', () => {
      const {result} = renderHook(() => useFlowNaming(undefined));

      expect(result.current.flowName).toBe('Login Flow');
      expect(result.current.flowHandle).toBe('login-flow');
    });
  });
});
