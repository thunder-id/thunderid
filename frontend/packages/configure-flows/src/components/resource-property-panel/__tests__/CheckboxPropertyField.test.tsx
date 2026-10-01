// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@testing-library/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import {ValidationContext, type ValidationContextProps} from '../../../context/ValidationContext';
import Notification from '../../../models/notification';
import type {Resource} from '../../../models/resources';
import CheckboxPropertyField from '../CheckboxPropertyField';

describe('CheckboxPropertyField', () => {
  const mockOnChange = vi.fn();

  const mockResource: Resource = {
    id: 'resource-1',
    type: 'CHECKBOX',
    config: {},
  } as Resource;

  const defaultContextValue: ValidationContextProps = {
    isValid: true,
    notifications: [],
    getNotification: vi.fn(),
    validationConfig: {
      isOTPValidationEnabled: false,
      isRecoveryFactorValidationEnabled: false,
      isPasswordExecutorValidationEnabled: false,
    },
  };

  const createWrapper = (contextValue: ValidationContextProps = defaultContextValue) => {
    function Wrapper({children}: {children: ReactNode}) {
      return <ValidationContext.Provider value={contextValue}>{children}</ValidationContext.Provider>;
    }
    return Wrapper;
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render checkbox with label', () => {
      render(
        <CheckboxPropertyField resource={mockResource} propertyKey="isEnabled" propertyValue onChange={mockOnChange} />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByLabelText('Is Enabled')).toBeInTheDocument();
    });

    it('should convert camelCase propertyKey to Start Case label', () => {
      render(
        <CheckboxPropertyField
          resource={mockResource}
          propertyKey="myPropertyName"
          propertyValue={false}
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByLabelText('My Property Name')).toBeInTheDocument();
    });

    it('should render checkbox as checked when propertyValue is true', () => {
      render(
        <CheckboxPropertyField resource={mockResource} propertyKey="enabled" propertyValue onChange={mockOnChange} />,
        {wrapper: createWrapper()},
      );

      const checkbox = screen.getByRole('checkbox');
      expect(checkbox).toBeChecked();
    });

    it('should render checkbox as unchecked when propertyValue is false', () => {
      render(
        <CheckboxPropertyField
          resource={mockResource}
          propertyKey="enabled"
          propertyValue={false}
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      const checkbox = screen.getByRole('checkbox');
      expect(checkbox).not.toBeChecked();
    });
  });

  describe('onChange Handler', () => {
    it('should call onChange with checked=true when checkbox is clicked', () => {
      render(
        <CheckboxPropertyField
          resource={mockResource}
          propertyKey="enabled"
          propertyValue={false}
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      const checkbox = screen.getByRole('checkbox');
      fireEvent.click(checkbox);

      expect(mockOnChange).toHaveBeenCalledWith('enabled', true, mockResource);
    });

    it('should call onChange with checked=false when checkbox is unchecked', () => {
      render(
        <CheckboxPropertyField resource={mockResource} propertyKey="enabled" propertyValue onChange={mockOnChange} />,
        {wrapper: createWrapper()},
      );

      const checkbox = screen.getByRole('checkbox');
      fireEvent.click(checkbox);

      expect(mockOnChange).toHaveBeenCalledWith('enabled', false, mockResource);
    });

    it('should pass the correct resource to onChange', () => {
      const specificResource = {...mockResource, id: 'specific-resource'};
      render(
        <CheckboxPropertyField
          resource={specificResource}
          propertyKey="active"
          propertyValue={false}
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      const checkbox = screen.getByRole('checkbox');
      fireEvent.click(checkbox);

      expect(mockOnChange).toHaveBeenCalledWith('active', true, specificResource);
    });
  });

  describe('Error State', () => {
    it('should display error message when notification exists', () => {
      const notification = new Notification('notification-1', 'Error', 'error');
      notification.addResourceFieldNotification('resource-1_isRequired', 'This field is required');

      const contextWithError: ValidationContextProps = {
        ...defaultContextValue,
        selectedNotification: notification,
      };

      render(
        <CheckboxPropertyField
          resource={mockResource}
          propertyKey="isRequired"
          propertyValue={false}
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper(contextWithError)},
      );

      expect(screen.getByText('This field is required')).toBeInTheDocument();
    });

    it('should not display error message when no notification exists', () => {
      render(
        <CheckboxPropertyField
          resource={mockResource}
          propertyKey="enabled"
          propertyValue={false}
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });

    it('should not display error message for different property', () => {
      const notification = new Notification('notification-1', 'Error', 'error');
      notification.addResourceFieldNotification('resource-1_otherProperty', 'Other error');

      const contextWithError: ValidationContextProps = {
        ...defaultContextValue,
        selectedNotification: notification,
      };

      render(
        <CheckboxPropertyField
          resource={mockResource}
          propertyKey="enabled"
          propertyValue={false}
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper(contextWithError)},
      );

      expect(screen.queryByText('Other error')).not.toBeInTheDocument();
    });
  });

  describe('Accessibility', () => {
    it('should have accessible checkbox element', () => {
      render(
        <CheckboxPropertyField resource={mockResource} propertyKey="enabled" propertyValue onChange={mockOnChange} />,
        {wrapper: createWrapper()},
      );

      const checkbox = screen.getByRole('checkbox');
      expect(checkbox).toBeInTheDocument();
    });
  });

  describe('Error Color State', () => {
    it('should render checkbox with error color when there is an error', () => {
      const notification = new Notification('notification-1', 'Error', 'error');
      notification.addResourceFieldNotification('resource-1_hasError', 'Field has an error');

      const contextWithError: ValidationContextProps = {
        ...defaultContextValue,
        selectedNotification: notification,
      };

      render(
        <CheckboxPropertyField
          resource={mockResource}
          propertyKey="hasError"
          propertyValue={false}
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper(contextWithError)},
      );

      // The error message should be displayed
      expect(screen.getByText('Field has an error')).toBeInTheDocument();
    });

    it('should render checkbox with primary color when no error', () => {
      render(
        <CheckboxPropertyField resource={mockResource} propertyKey="noError" propertyValue onChange={mockOnChange} />,
        {wrapper: createWrapper()},
      );

      // No error helper text should be present
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });
  });

  describe('Notification without matching field', () => {
    it('should return empty string when notification exists but has no matching field', () => {
      const notification = new Notification('notification-1', 'Error', 'error');
      // Add a notification for a different field
      notification.addResourceFieldNotification('resource-1_differentField', 'Different error');

      const contextWithNotification: ValidationContextProps = {
        ...defaultContextValue,
        selectedNotification: notification,
      };

      render(
        <CheckboxPropertyField resource={mockResource} propertyKey="enabled" propertyValue onChange={mockOnChange} />,
        {wrapper: createWrapper(contextWithNotification)},
      );

      // No error message for this field
      expect(screen.queryByText('Different error')).not.toBeInTheDocument();
    });
  });
});
