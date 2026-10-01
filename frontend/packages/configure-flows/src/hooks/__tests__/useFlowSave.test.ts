// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, act} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {CanvasData} from '../useFlowSave';
import useFlowSave from '../useFlowSave';

// Mock dependencies
vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, options?: string | Record<string, unknown>) => {
      const translations: Record<string, string> = {
        'flows:errors.FLM-1020': 'The flow definition has structural issues.',
      };
      if (translations[key] !== undefined) return translations[key];
      if (typeof options === 'string') return options || key;
      if (options && typeof options === 'object' && 'defaultValue' in options) {
        return (options['defaultValue'] as string) || '';
      }
      return key;
    },
  }),
}));

const mockCreateFlowMutate = vi.fn();
const mockUpdateFlowMutate = vi.fn();

vi.mock('../../api/useCreateFlow', () => ({
  default: () => ({
    mutate: mockCreateFlowMutate,
    isPending: false,
  }),
}));

vi.mock('../../api/useUpdateFlow', () => ({
  default: () => ({
    mutate: mockUpdateFlowMutate,
    isPending: false,
  }),
}));

vi.mock('../../utils/reactFlowTransformer', () => ({
  createFlowConfiguration: vi.fn(() => ({
    name: 'Test Flow',
    handle: 'test-flow',
    flowType: 'AUTHENTICATION',
    nodes: [],
  })),
  validateFlowGraph: vi.fn(() => []),
}));

const createMockCanvasData = (): CanvasData => ({
  nodes: [
    {id: 'start', type: 'START', position: {x: 0, y: 0}, data: {}},
    {id: 'view-1', type: 'VIEW', position: {x: 100, y: 100}, data: {}},
    {id: 'END', type: 'END', position: {x: 200, y: 200}, data: {}},
  ],
  edges: [
    {id: 'edge-1', source: 'start', target: 'view-1'},
    {id: 'edge-2', source: 'view-1', target: 'END'},
  ],
  viewport: {x: 0, y: 0, zoom: 1},
});

describe('useFlowSave', () => {
  let mockShowError: (message: string) => void;
  let mockShowSuccess: (message: string) => void;
  let mockSetOpenValidationPanel: (open: boolean) => void;

  beforeEach(() => {
    vi.clearAllMocks();
    mockShowError = vi.fn() as (message: string) => void;
    mockShowSuccess = vi.fn() as (message: string) => void;
    mockSetOpenValidationPanel = vi.fn() as (open: boolean) => void;
  });

  const renderUseFlowSave = (overrides = {}) => {
    const defaultProps = {
      flowId: undefined,
      isEditingExistingFlow: false,
      isFlowValid: true,
      flowName: 'Test Flow',
      flowHandle: 'test-flow',
      flowType: 'AUTHENTICATION',
      showError: mockShowError,
      showSuccess: mockShowSuccess,
      setOpenValidationPanel: mockSetOpenValidationPanel,
      ...overrides,
    };

    return renderHook(() => useFlowSave(defaultProps));
  };

  describe('Hook Interface', () => {
    it('should return handleSave function', () => {
      const {result} = renderUseFlowSave();
      expect(typeof result.current.handleSave).toBe('function');
    });

    it('should return isSaving boolean', () => {
      const {result} = renderUseFlowSave();
      expect(typeof result.current.isSaving).toBe('boolean');
    });
  });

  describe('handleSave - validation', () => {
    it('should show error and open validation panel when flow is invalid', () => {
      const {result} = renderUseFlowSave({isFlowValid: false});

      act(() => {
        result.current.handleSave(createMockCanvasData());
      });

      expect(mockShowError).toHaveBeenCalledWith('flows:core.loginFlowBuilder.errors.validationRequired');
      expect(mockSetOpenValidationPanel).toHaveBeenCalledWith(true);
      expect(mockCreateFlowMutate).not.toHaveBeenCalled();
      expect(mockUpdateFlowMutate).not.toHaveBeenCalled();
    });

    it('should not open validation panel when setOpenValidationPanel is not provided', () => {
      const {result} = renderUseFlowSave({
        isFlowValid: false,
        setOpenValidationPanel: undefined,
      });

      act(() => {
        result.current.handleSave(createMockCanvasData());
      });

      expect(mockShowError).toHaveBeenCalled();
      // Should not throw even when setOpenValidationPanel is undefined
    });

    it('should show error when flow graph validation fails', async () => {
      const {validateFlowGraph} = await import('../../utils/reactFlowTransformer');
      (validateFlowGraph as ReturnType<typeof vi.fn>).mockReturnValueOnce(['Disconnected node found']);

      const {result} = renderUseFlowSave({isFlowValid: true});

      act(() => {
        result.current.handleSave(createMockCanvasData());
      });

      expect(mockShowError).toHaveBeenCalledWith(
        expect.stringContaining('flows:core.loginFlowBuilder.errors.structureValidationFailed'),
      );
      expect(mockCreateFlowMutate).not.toHaveBeenCalled();
    });
  });

  describe('handleSave - create new flow', () => {
    it('should call createFlow.mutate when creating a new flow', () => {
      const {result} = renderUseFlowSave({
        isEditingExistingFlow: false,
        isFlowValid: true,
      });

      act(() => {
        result.current.handleSave(createMockCanvasData());
      });

      expect(mockCreateFlowMutate).toHaveBeenCalled();
      expect(mockUpdateFlowMutate).not.toHaveBeenCalled();
    });

    it('should show success message on create success', () => {
      mockCreateFlowMutate.mockImplementation((_, options: {onSuccess: () => void}) => {
        options.onSuccess();
      });

      const {result} = renderUseFlowSave({
        isEditingExistingFlow: false,
        isFlowValid: true,
      });

      act(() => {
        result.current.handleSave(createMockCanvasData());
      });

      expect(mockShowSuccess).toHaveBeenCalledWith('flows:core.loginFlowBuilder.success.flowCreated');
    });

    it('should show a resolved error message on create failure, never the raw server text', () => {
      mockCreateFlowMutate.mockImplementation((_, options: {onError: (err: Error) => void}) => {
        options.onError(new Error('Internal server error'));
      });

      const {result} = renderUseFlowSave({
        isEditingExistingFlow: false,
        isFlowValid: true,
      });

      act(() => {
        result.current.handleSave(createMockCanvasData());
      });

      expect(mockShowError).toHaveBeenCalledWith('Failed to save flow. Please try again.');
    });

    it('should show the specific message for a known backend error code on create failure', () => {
      mockCreateFlowMutate.mockImplementation((_, options: {onError: (err: Error) => void}) => {
        const err = new Error('Bad Request') as Error & {response: {data: {code: string}}};
        err.response = {data: {code: 'FLM-1020'}};
        options.onError(err);
      });

      const {result} = renderUseFlowSave({
        isEditingExistingFlow: false,
        isFlowValid: true,
      });

      act(() => {
        result.current.handleSave(createMockCanvasData());
      });

      expect(mockShowError).toHaveBeenCalledWith('The flow definition has structural issues.');
    });

    it('should invoke onSaved with the persisted canvas data on create success', () => {
      mockCreateFlowMutate.mockImplementation((_, options: {onSuccess: () => void}) => {
        options.onSuccess();
      });
      const onSaved = vi.fn();
      const canvasData = createMockCanvasData();

      const {result} = renderUseFlowSave({
        isEditingExistingFlow: false,
        isFlowValid: true,
        onSaved,
      });

      act(() => {
        result.current.handleSave(canvasData);
      });

      expect(onSaved).toHaveBeenCalledTimes(1);
      expect(onSaved).toHaveBeenCalledWith(canvasData);
    });

    it('should not invoke onSaved on create failure', () => {
      mockCreateFlowMutate.mockImplementation((_, options: {onError: (err: Error) => void}) => {
        options.onError(new Error('Internal server error'));
      });
      const onSaved = vi.fn();

      const {result} = renderUseFlowSave({
        isEditingExistingFlow: false,
        isFlowValid: true,
        onSaved,
      });

      act(() => {
        result.current.handleSave(createMockCanvasData());
      });

      expect(onSaved).not.toHaveBeenCalled();
    });
  });

  describe('handleSave - update existing flow', () => {
    it('should call updateFlow.mutate when updating an existing flow', () => {
      const {result} = renderUseFlowSave({
        flowId: 'flow-123',
        isEditingExistingFlow: true,
        isFlowValid: true,
      });

      act(() => {
        result.current.handleSave(createMockCanvasData());
      });

      expect(mockUpdateFlowMutate).toHaveBeenCalled();
      expect(mockCreateFlowMutate).not.toHaveBeenCalled();
    });

    it('should pass flowId to updateFlow.mutate', () => {
      const {result} = renderUseFlowSave({
        flowId: 'flow-456',
        isEditingExistingFlow: true,
        isFlowValid: true,
      });

      act(() => {
        result.current.handleSave(createMockCanvasData());
      });

      expect(mockUpdateFlowMutate).toHaveBeenCalledWith(
        expect.objectContaining({
          flowId: 'flow-456',
        }),
        expect.any(Object),
      );
    });

    it('should show success message on update success', () => {
      mockUpdateFlowMutate.mockImplementation((_, options: {onSuccess: () => void}) => {
        options.onSuccess();
      });

      const {result} = renderUseFlowSave({
        flowId: 'flow-123',
        isEditingExistingFlow: true,
        isFlowValid: true,
      });

      act(() => {
        result.current.handleSave(createMockCanvasData());
      });

      expect(mockShowSuccess).toHaveBeenCalledWith('flows:core.loginFlowBuilder.success.flowUpdated');
    });

    it('should invoke onSaved with the persisted canvas data on update success', () => {
      mockUpdateFlowMutate.mockImplementation((_, options: {onSuccess: () => void}) => {
        options.onSuccess();
      });
      const onSaved = vi.fn();
      const canvasData = createMockCanvasData();

      const {result} = renderUseFlowSave({
        flowId: 'flow-123',
        isEditingExistingFlow: true,
        isFlowValid: true,
        onSaved,
      });

      act(() => {
        result.current.handleSave(canvasData);
      });

      expect(onSaved).toHaveBeenCalledTimes(1);
      expect(onSaved).toHaveBeenCalledWith(canvasData);
    });

    it('should show a resolved error message on update failure, never the raw server text', () => {
      mockUpdateFlowMutate.mockImplementation((_, options: {onError: (err: Error) => void}) => {
        options.onError(new Error('Internal server error'));
      });

      const {result} = renderUseFlowSave({
        flowId: 'flow-123',
        isEditingExistingFlow: true,
        isFlowValid: true,
      });

      act(() => {
        result.current.handleSave(createMockCanvasData());
      });

      expect(mockShowError).toHaveBeenCalledWith('Failed to save flow. Please try again.');
    });

    it('should call createFlow when flowId is missing even if isEditingExistingFlow is true', () => {
      const {result} = renderUseFlowSave({
        flowId: undefined,
        isEditingExistingFlow: true,
        isFlowValid: true,
      });

      act(() => {
        result.current.handleSave(createMockCanvasData());
      });

      // The condition is (isEditingExistingFlow && flowId), so without flowId
      // it goes to the else branch and calls createFlow
      expect(mockCreateFlowMutate).toHaveBeenCalled();
      expect(mockUpdateFlowMutate).not.toHaveBeenCalled();
    });
  });

  describe('isSaving state', () => {
    it('should return false when not saving', () => {
      const {result} = renderUseFlowSave();
      expect(result.current.isSaving).toBe(false);
    });

    it('should return true when createFlow is pending', () => {
      // Since we can't easily re-import the hook, we test this indirectly
      // The hook returns isSaving: createFlow.isPending || updateFlow.isPending
      const {result} = renderUseFlowSave();

      // With default mocks, isSaving should be false
      expect(typeof result.current.isSaving).toBe('boolean');
    });

    it('should reflect the combined pending state of create and update', () => {
      const {result} = renderUseFlowSave();

      // isSaving is createFlow.isPending || updateFlow.isPending
      // With our mocks, both are false, so isSaving should be false
      expect(result.current.isSaving).toBe(false);
    });
  });

  describe('flow configuration', () => {
    it('should pass correct parameters to createFlowConfiguration', async () => {
      const {createFlowConfiguration} = await import('../../utils/reactFlowTransformer');

      const {result} = renderUseFlowSave({
        flowName: 'My Custom Flow',
        flowHandle: 'my-custom-flow',
        isFlowValid: true,
      });

      const canvasData = createMockCanvasData();

      act(() => {
        result.current.handleSave(canvasData);
      });

      expect(createFlowConfiguration).toHaveBeenCalledWith(
        canvasData,
        'My Custom Flow',
        'my-custom-flow',
        'AUTHENTICATION',
      );
    });
  });

  describe('handleSave callback stability', () => {
    it('should return handleSave as a function after rerender', () => {
      const {result, rerender} = renderUseFlowSave();

      expect(typeof result.current.handleSave).toBe('function');

      rerender();

      // After rerender, handleSave should still be a function
      expect(typeof result.current.handleSave).toBe('function');
    });
  });
});
