// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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

      expect(screen.getByTestId('application-copy-secret-button')).toHaveTextContent(/copy client secret/i);
      expect(screen.getByRole('button', {name: /continue/i})).toBeInTheDocument();
    });
  });

  describe('flow secret copy', () => {
    it('should name the Flow Secret when only a Flow Secret is issued', () => {
      renderComponent({clientSecret: '', flowSecret: 'flow_secret_12345'});

      expect(screen.getByRole('heading', {level: 1, name: /save your flow secret/i})).toBeInTheDocument();
      expect(screen.getByText(/only time you'll see this secret\. Store it somewhere safe\./i)).toBeInTheDocument();
      expect(screen.getByText(/your flow secret is a confidential key/i)).toBeInTheDocument();
      expect(screen.getByTestId('application-copy-secret-button')).toHaveTextContent(/copy flow secret/i);
      expect(screen.queryByText(/your client secret is a confidential key/i)).not.toBeInTheDocument();
    });

    it('should copy the Flow Secret from the main copy button when it is the only secret', async () => {
      const user = userEvent.setup();
      renderComponent({clientSecret: '', flowSecret: 'flow_secret_12345'});

      await user.click(screen.getByTestId('application-copy-secret-button'));

      await waitFor(() => {
        expect(mockCopy).toHaveBeenCalledWith('flow_secret_12345');
      });
    });

    it('should use neutral copy when both secrets are issued', () => {
      renderComponent({flowSecret: 'flow_secret_12345'});

      expect(screen.getByRole('heading', {level: 1, name: /save your secrets/i})).toBeInTheDocument();
      expect(screen.getByText(/only time you'll see these secrets\. Store them somewhere safe\./i)).toBeInTheDocument();
      expect(screen.getByText(/these secrets are confidential keys/i)).toBeInTheDocument();
      expect(screen.getByTestId('application-copy-secret-button')).toHaveTextContent(/copy client secret/i);
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

      const copyButton = screen.getAllByRole('button', {name: 'Copy Client Secret'})[0];

      await user.click(copyButton);

      await waitFor(() => {
        expect(mockCopy).toHaveBeenCalledWith('test_secret_12345');
      });
    });

    it('should call copy function when main copy button is clicked', async () => {
      const user = userEvent.setup();
      renderComponent();

      const mainCopyButton = screen.getByTestId('application-copy-secret-button');
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
    it('should render with different client secret', () => {
      renderComponent({clientSecret: 'different_secret_abc'});

      const input = screen.getByDisplayValue('different_secret_abc');
      expect(input).toBeInTheDocument();
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
