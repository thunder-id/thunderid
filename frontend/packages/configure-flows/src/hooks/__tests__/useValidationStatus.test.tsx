// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@testing-library/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi} from 'vitest';
import {ValidationContext, type ValidationContextProps} from '../../context/ValidationContext';
import Notification, {NotificationType} from '../../models/notification';
import useValidationStatus from '../useValidationStatus';

describe('useValidationStatus', () => {
  const mockNotification = new Notification('notification-1', 'Test Notification', NotificationType.WARNING);

  const mockContextValue: ValidationContextProps = {
    isValid: true,
    notifications: [],
    getNotification: vi.fn(),
    validationConfig: {
      isOTPValidationEnabled: false,
      isRecoveryFactorValidationEnabled: false,
      isPasswordExecutorValidationEnabled: false,
    },
  };

  const createWrapper = (contextValue: ValidationContextProps) => {
    function Wrapper({children}: {children: ReactNode}) {
      return <ValidationContext.Provider value={contextValue}>{children}</ValidationContext.Provider>;
    }
    return Wrapper;
  };

  it('should return context values when used within provider', () => {
    const {result} = renderHook(() => useValidationStatus(), {
      wrapper: createWrapper(mockContextValue),
    });

    expect(result.current.isValid).toBe(true);
    expect(result.current.notifications).toEqual([]);
  });

  it('should return default context values when used without explicit provider', () => {
    // When no provider is present, React returns the default context value
    // The hook checks for falsy context, but createContext provides defaults
    const {result} = renderHook(() => useValidationStatus());

    // Default context values should be returned
    expect(result.current.isValid).toBe(true);
    expect(result.current.notifications).toEqual([]);
  });

  it('should return notifications array', () => {
    const contextWithNotifications: ValidationContextProps = {
      ...mockContextValue,
      notifications: [mockNotification],
    };

    const {result} = renderHook(() => useValidationStatus(), {
      wrapper: createWrapper(contextWithNotifications),
    });

    expect(result.current.notifications).toHaveLength(1);
    expect(result.current.notifications[0].getId()).toBe('notification-1');
  });

  it('should return isValid as false when validation fails', () => {
    const invalidContext: ValidationContextProps = {
      ...mockContextValue,
      isValid: false,
      notifications: [mockNotification],
    };

    const {result} = renderHook(() => useValidationStatus(), {
      wrapper: createWrapper(invalidContext),
    });

    expect(result.current.isValid).toBe(false);
  });

  it('should return addNotification function when provided', () => {
    const mockAddNotification = vi.fn();
    const contextWithAdd: ValidationContextProps = {
      ...mockContextValue,
      addNotification: mockAddNotification,
    };

    const {result} = renderHook(() => useValidationStatus(), {
      wrapper: createWrapper(contextWithAdd),
    });

    expect(result.current.addNotification).toBe(mockAddNotification);
  });

  it('should return removeNotification function when provided', () => {
    const mockRemoveNotification = vi.fn();
    const contextWithRemove: ValidationContextProps = {
      ...mockContextValue,
      removeNotification: mockRemoveNotification,
    };

    const {result} = renderHook(() => useValidationStatus(), {
      wrapper: createWrapper(contextWithRemove),
    });

    expect(result.current.removeNotification).toBe(mockRemoveNotification);
  });

  it('should return getNotification function', () => {
    const mockGetNotification = vi.fn().mockReturnValue(mockNotification);
    const contextWithGet: ValidationContextProps = {
      ...mockContextValue,
      getNotification: mockGetNotification,
    };

    const {result} = renderHook(() => useValidationStatus(), {
      wrapper: createWrapper(contextWithGet),
    });

    expect(result.current.getNotification).toBe(mockGetNotification);
    expect(result.current.getNotification('notification-1')).toBe(mockNotification);
  });

  it('should return selectedNotification when provided', () => {
    const contextWithSelected: ValidationContextProps = {
      ...mockContextValue,
      selectedNotification: mockNotification,
    };

    const {result} = renderHook(() => useValidationStatus(), {
      wrapper: createWrapper(contextWithSelected),
    });

    expect(result.current.selectedNotification).toBe(mockNotification);
  });

  it('should return openValidationPanel state', () => {
    const contextWithPanel: ValidationContextProps = {
      ...mockContextValue,
      openValidationPanel: true,
    };

    const {result} = renderHook(() => useValidationStatus(), {
      wrapper: createWrapper(contextWithPanel),
    });

    expect(result.current.openValidationPanel).toBe(true);
  });

  it('should return setOpenValidationPanel function when provided', () => {
    const mockSetOpen = vi.fn();
    const contextWithSetPanel: ValidationContextProps = {
      ...mockContextValue,
      setOpenValidationPanel: mockSetOpen,
    };

    const {result} = renderHook(() => useValidationStatus(), {
      wrapper: createWrapper(contextWithSetPanel),
    });

    expect(result.current.setOpenValidationPanel).toBe(mockSetOpen);
  });

  it('should return currentActiveTab state', () => {
    const contextWithTab: ValidationContextProps = {
      ...mockContextValue,
      currentActiveTab: 2,
    };

    const {result} = renderHook(() => useValidationStatus(), {
      wrapper: createWrapper(contextWithTab),
    });

    expect(result.current.currentActiveTab).toBe(2);
  });

  it('should return validationConfig', () => {
    const contextWithConfig: ValidationContextProps = {
      ...mockContextValue,
      validationConfig: {
        isOTPValidationEnabled: true,
        isRecoveryFactorValidationEnabled: true,
        isPasswordExecutorValidationEnabled: false,
      },
    };

    const {result} = renderHook(() => useValidationStatus(), {
      wrapper: createWrapper(contextWithConfig),
    });

    expect(result.current.validationConfig?.isOTPValidationEnabled).toBe(true);
    expect(result.current.validationConfig?.isRecoveryFactorValidationEnabled).toBe(true);
  });

  it('should throw an error when used outside of ValidationProvider with falsy context', () => {
    // Create a wrapper that provides null/undefined as the context value
    function NullContextWrapper({children}: {children: ReactNode}) {
      return (
        <ValidationContext.Provider value={null as unknown as ValidationContextProps}>
          {children}
        </ValidationContext.Provider>
      );
    }

    expect(() => {
      renderHook(() => useValidationStatus(), {
        wrapper: NullContextWrapper,
      });
    }).toThrow('useValidationStatus must be used within a ValidationProvider');
  });
});
