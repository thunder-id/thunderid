// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, screen, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import ShowClientSecret, {type ShowClientSecretProps} from '../ShowClientSecret';

// Mock the useCopyToClipboard hook
vi.mock('@thunderid/hooks', () => ({
  useCopyToClipboard: vi.fn(),
}));

const {useCopyToClipboard} = await import('@thunderid/hooks');

describe('ShowClientSecret', () => {
  const mockOnContinue = vi.fn();
  const mockCopy = vi.fn().mockResolvedValue(undefined);

  const defaultProps: ShowClientSecretProps = {
    agentName: 'Test Agent',
    clientId: 'client-id-xyz',
    clientSecret: 'test_secret_12345',
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
    it('should render the title and subtitle', () => {
      renderComponent();

      expect(screen.getByRole('heading', {level: 1, name: /save your client secret/i})).toBeInTheDocument();
      expect(screen.getByText(/store it somewhere safe/i)).toBeInTheDocument();
    });

    it('should display the agent name', () => {
      renderComponent();

      expect(screen.getByText('Agent name')).toBeInTheDocument();
      expect(screen.getByText('Test Agent')).toBeInTheDocument();
    });

    it('should display the clientId when provided', () => {
      renderComponent();

      expect(screen.getByText('Client ID')).toBeInTheDocument();
      expect(screen.getByText('client-id-xyz')).toBeInTheDocument();
    });

    it('should not display the clientId field when not provided', () => {
      renderComponent({clientId: undefined});

      expect(screen.queryByText('Client ID')).not.toBeInTheDocument();
    });

    it('should render the client secret field as masked password', () => {
      renderComponent();

      const input = screen.getByDisplayValue('test_secret_12345');
      expect(input).toBeInTheDocument();
      expect(input).toHaveAttribute('type', 'password');
      expect(input).toHaveAttribute('readonly');
    });

    it('should render security reminder alert', () => {
      renderComponent();

      expect(screen.getByText(/you won't be able to see this secret again/i)).toBeInTheDocument();
      expect(screen.getByText(/store the client secret somewhere safe/i)).toBeInTheDocument();
    });

    it('should render action buttons', () => {
      renderComponent();

      expect(screen.getByRole('button', {name: /copy client secret/i})).toBeInTheDocument();
      expect(screen.getByTestId('agent-client-secret-continue')).toBeInTheDocument();
    });
  });

  describe('visibility toggle', () => {
    it('should toggle client secret visibility when eye icon is clicked', async () => {
      const user = userEvent.setup();
      renderComponent();

      const input = screen.getByDisplayValue('test_secret_12345');
      expect(input).toHaveAttribute('type', 'password');

      const allButtons = screen.getAllByRole('button');
      const iconButtons = allButtons.filter(
        (btn) => btn.querySelector('svg') && !/copy|continue/i.exec(btn.textContent),
      );
      const visibilityButton = iconButtons[0];

      await user.click(visibilityButton);

      expect(input).toHaveAttribute('type', 'text');

      await user.click(visibilityButton);

      expect(input).toHaveAttribute('type', 'password');
    });
  });

  describe('copy functionality', () => {
    it('should call copy function when main copy button is clicked', async () => {
      const user = userEvent.setup();
      renderComponent();

      const mainCopyButton = screen.getByRole('button', {name: /copy client secret/i});
      await user.click(mainCopyButton);

      await waitFor(() => {
        expect(mockCopy).toHaveBeenCalledWith('test_secret_12345');
      });
    });

    it('should show copied state when copied is true', () => {
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

      expect(screen.getByRole('button', {name: /copied/i})).toBeDisabled();
    });

    it('should configure useCopyToClipboard with resetDelay 2000', () => {
      renderComponent();

      const hookCall = vi.mocked(useCopyToClipboard).mock.calls[0][0];
      expect(hookCall).toHaveProperty('resetDelay', 2000);
    });
  });

  describe('continue action', () => {
    it('should call onContinue when continue button is clicked', async () => {
      const user = userEvent.setup();
      renderComponent();

      const continueButton = screen.getByTestId('agent-client-secret-continue');
      await user.click(continueButton);

      expect(mockOnContinue).toHaveBeenCalledTimes(1);
    });
  });
});
