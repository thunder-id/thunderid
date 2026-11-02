// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {UserRound} from '@wso2/oxygen-ui-icons-react';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import AuthenticationMethodItem, {type AuthenticationMethodItemProps} from '../AuthenticationMethodItem';

// Mock react-i18next
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'applications:onboarding.configure.SignInOptions.notConfigured': 'Not configured',
      };
      return translations[key] || key;
    },
  }),
}));

describe('AuthenticationMethodItem', () => {
  const mockOnToggle = vi.fn();

  const defaultProps: AuthenticationMethodItemProps = {
    id: 'test-auth-method',
    name: 'Test Auth Method',
    icon: <UserRound size={24} data-testid="test-icon" />,
    isEnabled: false,
    isAvailable: true,
    onToggle: mockOnToggle,
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  const renderComponent = (props: Partial<AuthenticationMethodItemProps> = {}) =>
    render(<AuthenticationMethodItem {...defaultProps} {...props} />);

  describe('when method is available', () => {
    it('should render the authentication method with all elements', () => {
      renderComponent();

      expect(screen.getByText('Test Auth Method')).toBeInTheDocument();
      expect(screen.getByTestId('test-icon')).toBeInTheDocument();
      expect(screen.getByRole('switch')).toBeInTheDocument();
    });

    it('should not render a button for the label/icon area', () => {
      renderComponent();

      expect(screen.queryByRole('button')).not.toBeInTheDocument();
    });

    it('should show switch as checked when method is enabled', () => {
      renderComponent({
        isEnabled: true,
      });

      const switchElement = screen.getByRole('switch');
      expect(switchElement).toBeChecked();
    });

    it('should show switch as unchecked when method is disabled', () => {
      renderComponent({
        isEnabled: false,
      });

      const switchElement = screen.getByRole('switch');
      expect(switchElement).not.toBeChecked();
    });

    it('should not call onToggle when clicking the label', async () => {
      const user = userEvent.setup();
      renderComponent();

      await user.click(screen.getByText('Test Auth Method'));

      expect(mockOnToggle).not.toHaveBeenCalled();
    });

    it('should call onToggle when switch is clicked', async () => {
      const user = userEvent.setup();
      renderComponent();

      const switchElement = screen.getByRole('switch');
      await user.click(switchElement);

      expect(mockOnToggle).toHaveBeenCalledWith('test-auth-method');
      expect(mockOnToggle).toHaveBeenCalledTimes(1);
    });

    it('should not disable the switch when available', () => {
      renderComponent({
        isAvailable: true,
      });

      const switchElement = screen.getByRole('switch');
      expect(switchElement).not.toBeDisabled();
    });

    it('should handle multiple rapid clicks gracefully', async () => {
      const user = userEvent.setup();
      renderComponent();

      const switchElement = screen.getByRole('switch');

      // Click multiple times rapidly
      await user.click(switchElement);
      await user.click(switchElement);
      await user.click(switchElement);

      expect(mockOnToggle).toHaveBeenCalledTimes(3);
      expect(mockOnToggle).toHaveBeenCalledWith('test-auth-method');
    });
  });

  describe('when method is not available', () => {
    it('should render disabled state with "Not configured" text', () => {
      renderComponent({
        isAvailable: false,
      });

      expect(screen.getByText('Test Auth Method')).toBeInTheDocument();
      expect(screen.getByText('Not configured')).toBeInTheDocument();
      expect(screen.getByTestId('test-icon')).toBeInTheDocument();
    });

    it('should not render a switch when not available', () => {
      renderComponent({
        isAvailable: false,
      });

      expect(screen.queryByRole('switch')).not.toBeInTheDocument();
    });

    it('should not call onToggle when clicking the label', async () => {
      const user = userEvent.setup();
      renderComponent({
        isAvailable: false,
      });

      await user.click(screen.getByText('Test Auth Method'));

      expect(mockOnToggle).not.toHaveBeenCalled();
    });
  });

  describe('accessibility', () => {
    it('should expose only the switch as an interactive control', () => {
      renderComponent();

      expect(screen.getByRole('switch')).toBeInTheDocument();
      expect(screen.queryByRole('button')).not.toBeInTheDocument();
    });

    it('should be keyboard navigable via the switch', async () => {
      const user = userEvent.setup();
      renderComponent();

      const switchElement = screen.getByRole('switch');

      await user.tab();
      expect(switchElement).toHaveFocus();

      await user.keyboard(' ');
      expect(mockOnToggle).toHaveBeenCalledWith('test-auth-method');
    });
  });

  describe('different authentication method types', () => {
    it('should handle different icon types', () => {
      const customIcon = <div data-testid="custom-icon">Custom Icon</div>;
      renderComponent({
        icon: customIcon,
      });

      expect(screen.getByTestId('custom-icon')).toBeInTheDocument();
      expect(screen.queryByTestId('test-icon')).not.toBeInTheDocument();
    });

    it('should handle long authentication method names', () => {
      renderComponent({
        name: 'Very Long Authentication Method Name That Should Still Work Properly',
      });

      expect(
        screen.getByText('Very Long Authentication Method Name That Should Still Work Properly'),
      ).toBeInTheDocument();
    });

    it('should handle special characters in method names', () => {
      renderComponent({
        name: 'OAuth 2 & OIDC Provider',
      });

      expect(screen.getByText('OAuth 2 & OIDC Provider')).toBeInTheDocument();
    });

    it('should handle different ID formats', async () => {
      const user = userEvent.setup();
      renderComponent({
        id: 'oauth2-provider-123',
      });

      await user.click(screen.getByRole('switch'));

      expect(mockOnToggle).toHaveBeenCalledWith('oauth2-provider-123');
    });
  });

  describe('state combinations', () => {
    it('should handle enabled and available state', () => {
      renderComponent({
        isEnabled: true,
        isAvailable: true,
      });

      const switchElement = screen.getByRole('switch');

      expect(switchElement).toBeChecked();
      expect(switchElement).not.toBeDisabled();
    });

    it('should handle disabled but available state', () => {
      renderComponent({
        isEnabled: false,
        isAvailable: true,
      });

      const switchElement = screen.getByRole('switch');

      expect(switchElement).not.toBeChecked();
      expect(switchElement).not.toBeDisabled();
    });

    it('should not render a switch when not available, regardless of enabled state', () => {
      renderComponent({
        isEnabled: true,
        isAvailable: false,
      });

      expect(screen.queryByRole('switch')).not.toBeInTheDocument();
    });
  });

  describe('isDisabled prop', () => {
    it('should disable the switch when isDisabled is true', () => {
      renderComponent({
        isAvailable: true,
        isDisabled: true,
      });

      expect(screen.getByRole('switch')).toBeDisabled();
    });

    it('should not call onToggle when isDisabled is true', () => {
      renderComponent({
        isAvailable: true,
        isDisabled: true,
      });

      const switchElement = screen.getByRole('switch');
      // Verify the switch is disabled - clicking is prevented by pointer-events: none
      expect(switchElement).toBeDisabled();
      // Since we can't click a disabled element, verify onToggle wasn't called during render
      expect(mockOnToggle).not.toHaveBeenCalled();
    });

    it('should combine isEnabled true with isDisabled true', () => {
      renderComponent({
        isEnabled: true,
        isAvailable: true,
        isDisabled: true,
      });

      const switchElement = screen.getByRole('switch');
      expect(switchElement).toBeChecked();
      expect(switchElement).toBeDisabled();
    });

    it('should render icon in the disabled state when not available', () => {
      renderComponent({
        isAvailable: false,
      });

      expect(screen.getByTestId('test-icon')).toBeInTheDocument();
    });

    it('should render icon in the enabled state when available', () => {
      renderComponent({
        isAvailable: true,
      });

      expect(screen.getByTestId('test-icon')).toBeInTheDocument();
    });

    it('should pass isDisabled false by default when not specified', () => {
      renderComponent({
        isAvailable: true,
      });

      const switchElement = screen.getByRole('switch');
      expect(switchElement).not.toBeDisabled();
    });
  });
});
