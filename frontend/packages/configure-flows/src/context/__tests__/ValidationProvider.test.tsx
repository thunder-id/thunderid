// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, act} from '@testing-library/react';
import {useContext} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import Notification, {NotificationType} from '../../models/notification';
import {ValidationContext} from '../ValidationContext';
import ValidationProvider from '../ValidationProvider';

// Mock useFlowConfig — returns empty flowNodes by default (no validation notifications)
vi.mock('../../hooks/useFlowConfig', () => ({
  default: () => ({
    flowNodes: [],
  }),
}));

// Mock react-i18next
vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

// Mock useUIPanelState
const mockSetIsOpenResourcePropertiesPanel = vi.fn();
const mockRegisterCloseValidationPanel = vi.fn();

vi.mock('../../hooks/useUIPanelState', () => ({
  default: () => ({
    setIsOpenResourcePropertiesPanel: mockSetIsOpenResourcePropertiesPanel,
    registerCloseValidationPanel: mockRegisterCloseValidationPanel,
  }),
}));

// Test component to access context
function TestConsumer() {
  const context = useContext(ValidationContext);
  return (
    <div>
      <span data-testid="is-valid">{context.isValid.toString()}</span>
      <span data-testid="notifications-count">{context.notifications.length}</span>
      <span data-testid="open-validation-panel">{context.openValidationPanel?.toString()}</span>
      <span data-testid="current-tab">{context.currentActiveTab}</span>
      <button
        type="button"
        data-testid="add-notification"
        onClick={() => {
          const notification = new Notification('test-id', 'Test Error', NotificationType.ERROR);
          context.addNotification?.(notification);
        }}
      >
        Add Notification
      </button>
      <button
        type="button"
        data-testid="add-warning"
        onClick={() => {
          const notification = new Notification('warning-id', 'Test Warning', NotificationType.WARNING);
          context.addNotification?.(notification);
        }}
      >
        Add Warning
      </button>
      <button type="button" data-testid="remove-notification" onClick={() => context.removeNotification?.('test-id')}>
        Remove Notification
      </button>
      <button type="button" data-testid="open-panel" onClick={() => context.setOpenValidationPanel?.(true)}>
        Open Panel
      </button>
      <button type="button" data-testid="close-panel" onClick={() => context.setOpenValidationPanel?.(false)}>
        Close Panel
      </button>
      <button type="button" data-testid="set-tab" onClick={() => context.setCurrentActiveTab?.(2)}>
        Set Tab
      </button>
    </div>
  );
}

describe('ValidationProvider', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Initial State', () => {
    it('should provide initial valid state as true', () => {
      render(
        <ValidationProvider>
          <TestConsumer />
        </ValidationProvider>,
      );

      expect(screen.getByTestId('is-valid')).toHaveTextContent('true');
    });

    it('should provide empty notifications list initially', () => {
      render(
        <ValidationProvider>
          <TestConsumer />
        </ValidationProvider>,
      );

      expect(screen.getByTestId('notifications-count')).toHaveTextContent('0');
    });

    it('should have validation panel closed initially', () => {
      render(
        <ValidationProvider>
          <TestConsumer />
        </ValidationProvider>,
      );

      expect(screen.getByTestId('open-validation-panel')).toHaveTextContent('false');
    });

    it('should have current active tab set to 0 initially', () => {
      render(
        <ValidationProvider>
          <TestConsumer />
        </ValidationProvider>,
      );

      expect(screen.getByTestId('current-tab')).toHaveTextContent('0');
    });
  });

  describe('Validation Config', () => {
    it('should use default validation config when not provided', () => {
      render(
        <ValidationProvider>
          <TestConsumer />
        </ValidationProvider>,
      );

      expect(screen.getByTestId('is-valid')).toBeInTheDocument();
    });

    it('should accept custom validation config', () => {
      render(
        <ValidationProvider validationConfig={{isOTPValidationEnabled: true, isRecoveryFactorValidationEnabled: true}}>
          <TestConsumer />
        </ValidationProvider>,
      );

      expect(screen.getByTestId('is-valid')).toBeInTheDocument();
    });
  });

  describe('Notification Management', () => {
    it('should add notification', () => {
      render(
        <ValidationProvider>
          <TestConsumer />
        </ValidationProvider>,
      );

      act(() => {
        screen.getByTestId('add-notification').click();
      });

      expect(screen.getByTestId('notifications-count')).toHaveTextContent('1');
    });

    it('should set isValid to false when error notification is added', () => {
      render(
        <ValidationProvider>
          <TestConsumer />
        </ValidationProvider>,
      );

      expect(screen.getByTestId('is-valid')).toHaveTextContent('true');

      act(() => {
        screen.getByTestId('add-notification').click();
      });

      expect(screen.getByTestId('is-valid')).toHaveTextContent('false');
    });

    it('should remain valid when only warning notification is added', () => {
      render(
        <ValidationProvider>
          <TestConsumer />
        </ValidationProvider>,
      );

      act(() => {
        screen.getByTestId('add-warning').click();
      });

      expect(screen.getByTestId('is-valid')).toHaveTextContent('true');
    });

    it('should remove notification', () => {
      render(
        <ValidationProvider>
          <TestConsumer />
        </ValidationProvider>,
      );

      act(() => {
        screen.getByTestId('add-notification').click();
      });

      expect(screen.getByTestId('notifications-count')).toHaveTextContent('1');

      act(() => {
        screen.getByTestId('remove-notification').click();
      });

      expect(screen.getByTestId('notifications-count')).toHaveTextContent('0');
    });

    it('should restore isValid to true when error notification is removed', () => {
      render(
        <ValidationProvider>
          <TestConsumer />
        </ValidationProvider>,
      );

      act(() => {
        screen.getByTestId('add-notification').click();
      });

      expect(screen.getByTestId('is-valid')).toHaveTextContent('false');

      act(() => {
        screen.getByTestId('remove-notification').click();
      });

      expect(screen.getByTestId('is-valid')).toHaveTextContent('true');
    });
  });

  describe('Validation Panel', () => {
    it('should open validation panel', () => {
      render(
        <ValidationProvider>
          <TestConsumer />
        </ValidationProvider>,
      );

      act(() => {
        screen.getByTestId('open-panel').click();
      });

      expect(screen.getByTestId('open-validation-panel')).toHaveTextContent('true');
    });

    it('should close resource properties panel when opening validation panel', () => {
      render(
        <ValidationProvider>
          <TestConsumer />
        </ValidationProvider>,
      );

      act(() => {
        screen.getByTestId('open-panel').click();
      });

      expect(mockSetIsOpenResourcePropertiesPanel).toHaveBeenCalledWith(false);
    });

    it('should close validation panel', () => {
      render(
        <ValidationProvider>
          <TestConsumer />
        </ValidationProvider>,
      );

      act(() => {
        screen.getByTestId('open-panel').click();
      });

      act(() => {
        screen.getByTestId('close-panel').click();
      });

      expect(screen.getByTestId('open-validation-panel')).toHaveTextContent('false');
    });
  });

  describe('Tab Management', () => {
    it('should update current active tab', () => {
      render(
        <ValidationProvider>
          <TestConsumer />
        </ValidationProvider>,
      );

      act(() => {
        screen.getByTestId('set-tab').click();
      });

      expect(screen.getByTestId('current-tab')).toHaveTextContent('2');
    });
  });

  describe('Registration with FlowBuilderCore', () => {
    it('should register close validation panel callback', () => {
      render(
        <ValidationProvider>
          <TestConsumer />
        </ValidationProvider>,
      );

      expect(mockRegisterCloseValidationPanel).toHaveBeenCalledWith(expect.any(Function));
    });

    it('should unregister callback on unmount', () => {
      const {unmount} = render(
        <ValidationProvider>
          <TestConsumer />
        </ValidationProvider>,
      );

      unmount();

      // The cleanup function should register an empty callback
      expect(mockRegisterCloseValidationPanel).toHaveBeenLastCalledWith(expect.any(Function));
    });
  });

  describe('Children Rendering', () => {
    it('should render children', () => {
      render(
        <ValidationProvider>
          <div data-testid="child">Child Content</div>
        </ValidationProvider>,
      );

      expect(screen.getByTestId('child')).toHaveTextContent('Child Content');
    });

    it('should render without children', () => {
      const {container} = render(<ValidationProvider />);

      expect(container).toBeInTheDocument();
    });
  });
});
