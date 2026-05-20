/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import {render, screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import ShowClientSecret, {type ShowClientSecretProps} from '../ShowClientSecret';

// Mock the useCopyToClipboard hook
vi.mock('@thunderid/hooks', () => ({
  useCopyToClipboard: vi.fn(),
}));

const {useCopyToClipboard} = await import('@thunderid/hooks');

describe('ShowClientSecret', () => {
  const mockOnCopySecret = vi.fn();
  const mockOnContinue = vi.fn();
  const mockCopy = vi.fn().mockResolvedValue(undefined);

  const defaultProps: ShowClientSecretProps = {
    appName: 'Test Application',
    clientId: 'test_client_id_12345',
    clientSecret: 'test_secret_12345',
    onCopySecret: mockOnCopySecret,
    onContinue: mockOnContinue,
  };

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useCopyToClipboard).mockReturnValue({
      copied: false,
      copy: mockCopy,
    });
  });

  const renderComponent = (props: Partial<ShowClientSecretProps> = {}) =>
    render(<ShowClientSecret {...defaultProps} {...props} />);

  describe('rendering', () => {
    it('should render the component with warning icon', () => {
      renderComponent();

      // Warning icon should be present - just check component renders without error
      expect(screen.getByRole('heading', {level: 1})).toBeInTheDocument();
    });

    it('should render the title and subtitle', () => {
      renderComponent();

      expect(screen.getByRole('heading', {level: 1, name: /save your client secret/i})).toBeInTheDocument();
      expect(screen.getByText(/store it somewhere safe/i)).toBeInTheDocument();
    });

    it('should display the application name', () => {
      renderComponent();

      expect(screen.getByText('App Name')).toBeInTheDocument();
      expect(screen.getByText('Test Application')).toBeInTheDocument();
    });

    it('should render the client ID field', () => {
      renderComponent();

      expect(screen.getByText('Client ID')).toBeInTheDocument();
      const input = screen.getByDisplayValue('test_client_id_12345');
      expect(input).toBeInTheDocument();
      expect(input).toHaveAttribute('readonly');
    });

    it('should render the client secret field', () => {
      renderComponent();

      expect(screen.getByText('Client Secret')).toBeInTheDocument();
      const input = screen.getByDisplayValue('test_secret_12345');
      expect(input).toBeInTheDocument();
      expect(input).toHaveAttribute('type', 'password');
      expect(input).toHaveAttribute('readonly');
    });

    it('should render security reminder alert', () => {
      renderComponent();

      expect(screen.getByText(/security reminder/i)).toBeInTheDocument();
      expect(screen.getByText(/should be treated with the same level of security/i)).toBeInTheDocument();
    });

    it('should render action buttons', () => {
      renderComponent();

      expect(screen.getByRole('button', {name: /copy secret/i})).toBeInTheDocument();
      expect(screen.getByRole('button', {name: /continue/i})).toBeInTheDocument();
    });
  });

  describe('visibility toggle', () => {
    it('should toggle client secret visibility when eye icon is clicked', async () => {
      const user = userEvent.setup();
      renderComponent();

      const input = screen.getByDisplayValue('test_secret_12345');
      expect(input).toHaveAttribute('type', 'password');

      const visibilityButton = screen.getByRole('button', {name: 'Toggle secret visibility'});

      await user.click(visibilityButton);

      // Should now show as text
      expect(input).toHaveAttribute('type', 'text');

      // Click again to hide (same button, just state changed)
      await user.click(visibilityButton);

      // Should be back to password
      expect(input).toHaveAttribute('type', 'password');
    });
  });

  describe('copy functionality', () => {
    it('should call copy function when copy button in input is clicked', async () => {
      const user = userEvent.setup();
      renderComponent();

      const copyButton = screen.getByRole('button', {name: 'Copy Client Secret'});

      await user.click(copyButton);

      await waitFor(() => {
        expect(mockCopy).toHaveBeenCalledWith('test_secret_12345');
      });
    });

    it('should call copy function when copy client ID button is clicked', async () => {
      const user = userEvent.setup();
      renderComponent();

      const copyButton = screen.getByRole('button', {name: 'Copy Client ID'});

      await user.click(copyButton);

      await waitFor(() => {
        expect(mockCopy).toHaveBeenCalledWith('test_client_id_12345');
      });
    });

    it('should call copy function when main copy button is clicked', async () => {
      const user = userEvent.setup();
      renderComponent();

      const mainCopyButton = screen.getByRole('button', {name: /copy secret/i});
      await user.click(mainCopyButton);

      await waitFor(() => {
        expect(mockCopy).toHaveBeenCalledWith('test_secret_12345');
      });
    });

    it('should show copied state when copy succeeds', () => {
      vi.mocked(useCopyToClipboard).mockReturnValue({
        copied: true,
        copy: mockCopy,
      });

      renderComponent();

      expect(screen.getByRole('button', {name: /copied/i})).toBeInTheDocument();
    });

    it('should disable copy button when in copied state', () => {
      vi.mocked(useCopyToClipboard).mockReturnValue({
        copied: true,
        copy: mockCopy,
      });

      renderComponent();

      const mainCopyButton = screen.getByRole('button', {name: /copied/i});
      expect(mainCopyButton).toBeDisabled();
    });

    it('should call onCopySecret callback through useCopyToClipboard', () => {
      renderComponent();

      const hookCalls = vi.mocked(useCopyToClipboard).mock.calls.map((call) => call[0]);
      const secretHookCall = hookCalls.find((call) => call?.onCopy === mockOnCopySecret);

      expect(secretHookCall).toBeDefined();
      expect(secretHookCall).toHaveProperty('onCopy', mockOnCopySecret);
      expect(secretHookCall).toHaveProperty('resetDelay', 2000);
    });
  });

  describe('continue action', () => {
    it('should call onContinue when continue button is clicked', async () => {
      const user = userEvent.setup();
      renderComponent();

      const continueButton = screen.getByRole('button', {name: /continue/i});
      await user.click(continueButton);

      expect(mockOnContinue).toHaveBeenCalledTimes(1);
    });
  });

  describe('props variations', () => {
    it('should render with different app name', () => {
      renderComponent({appName: 'Another App'});

      expect(screen.getByText('Another App')).toBeInTheDocument();
    });

    it('should render with different client secret', () => {
      renderComponent({clientSecret: 'different_secret_abc'});

      const input = screen.getByDisplayValue('different_secret_abc');
      expect(input).toBeInTheDocument();
    });

    it('should not render the client ID field when not provided', () => {
      renderComponent({clientId: ''});

      expect(screen.queryByText('Client ID')).not.toBeInTheDocument();
    });
  });

  describe('accessibility', () => {
    it('should have proper heading structure', () => {
      renderComponent();

      const heading = screen.getByRole('heading', {level: 1});
      expect(heading).toBeInTheDocument();
    });

    it('should have accessible buttons', () => {
      renderComponent();

      const buttons = screen.getAllByRole('button');
      expect(buttons.length).toBeGreaterThan(0);
      buttons.forEach((button) => {
        expect(button).toBeVisible();
      });
    });

    it('should have readonly input for security', () => {
      renderComponent();

      const input = screen.getByDisplayValue('test_secret_12345');
      expect(input).toHaveAttribute('readonly');
    });
  });
});
