// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {MutateOptions, MutationFunctionContext} from '@tanstack/react-query';
import userEvent from '@testing-library/user-event';
import {render, screen, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import type {RegenerateSecretVariables, RegenerateSecretResult} from '../../api/useRegenerateClientSecret';
import RegenerateSecretDialog from '../RegenerateSecretDialog';
import type {RegenerateSecretDialogProps} from '../RegenerateSecretDialog';

// Mock the logger
vi.mock('@thunderid/logger', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/logger')>();
  return {
    ...actual,
    useLogger: () => ({
      info: vi.fn(),
      error: vi.fn(),
      warn: vi.fn(),
      debug: vi.fn(),
    }),
  };
});

// Create a mock mutate function
const mockMutate = vi.fn();
const mockRegenerateClientSecret = {
  mutate: mockMutate,
  isPending: false,
};

// Mock useRegenerateClientSecret hook
vi.mock('../../api/useRegenerateClientSecret', () => ({
  default: () => mockRegenerateClientSecret,
}));

// Mock translations
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallbackOrOptions?: string | {defaultValue?: string}) => {
      const translations: Record<string, string> = {
        'regenerateSecret.dialog.title': 'Regenerate Client Secret',
        'regenerateSecret.dialog.message':
          'Are you sure you want to regenerate the client secret for this application? This will immediately invalidate the current client secret and generate a new one.',
        'regenerateSecret.dialog.disclaimer':
          'Warning: Regenerating the client secret will invalidate the current secret and the application may stop working until the new client secret is updated in its configuration.',
        'regenerateSecret.dialog.confirmButton': 'Regenerate',
        'regenerateSecret.dialog.regenerating': 'Regenerating...',
        'regenerateSecret.dialog.error': 'Failed to regenerate client secret. Please try again.',
        'errors.APP-1030': 'This application is managed declaratively and cannot be edited or deleted.',
        'common:actions.cancel': 'Cancel',
      };
      if (translations[key] !== undefined) return translations[key];
      if (typeof fallbackOrOptions === 'string') return fallbackOrOptions;
      if (fallbackOrOptions && 'defaultValue' in fallbackOrOptions) return fallbackOrOptions.defaultValue ?? key;
      return key;
    },
  }),
}));

describe('RegenerateSecretDialog', () => {
  const mockOnClose = vi.fn();
  const mockOnSuccess = vi.fn();
  const mockOnError = vi.fn();

  const defaultProps: RegenerateSecretDialogProps = {
    open: true,
    applicationId: 'test-app-id',
    onClose: mockOnClose,
    onSuccess: mockOnSuccess,
    onError: mockOnError,
  };

  const renderDialog = (props: RegenerateSecretDialogProps = defaultProps) =>
    render(<RegenerateSecretDialog {...props} />);

  beforeEach(() => {
    vi.clearAllMocks();
    mockRegenerateClientSecret.isPending = false;
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render the dialog when open is true', () => {
      renderDialog();

      expect(screen.getByRole('dialog')).toBeInTheDocument();
      expect(screen.getByText('Regenerate Client Secret')).toBeInTheDocument();
      expect(
        screen.getByText(
          'Are you sure you want to regenerate the client secret for this application? This will immediately invalidate the current client secret and generate a new one.',
        ),
      ).toBeInTheDocument();
    });

    it('should show warning disclaimer', () => {
      renderDialog();

      expect(
        screen.getByText(
          'Warning: Regenerating the client secret will invalidate the current secret and the application may stop working until the new client secret is updated in its configuration.',
        ),
      ).toBeInTheDocument();
    });

    it('should not render dialog content when open is false', () => {
      renderDialog({...defaultProps, open: false});

      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });

    it('should render Cancel and Regenerate buttons', () => {
      renderDialog();

      expect(screen.getByRole('button', {name: 'Cancel'})).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Regenerate'})).toBeInTheDocument();
    });
  });

  describe('User Interactions', () => {
    it('should call onClose when Cancel button is clicked', async () => {
      const user = userEvent.setup();
      renderDialog();

      const cancelButton = screen.getByRole('button', {name: 'Cancel'});
      await user.click(cancelButton);

      expect(mockOnClose).toHaveBeenCalledTimes(1);
    });

    it('should call onClose when Escape key is pressed', async () => {
      const user = userEvent.setup();
      renderDialog();

      await user.keyboard('{Escape}');

      expect(mockOnClose).toHaveBeenCalledTimes(1);
    });

    it('should call mutate when Regenerate button is clicked', async () => {
      const user = userEvent.setup();
      renderDialog();

      const regenerateButton = screen.getByRole('button', {name: 'Regenerate'});
      await user.click(regenerateButton);

      expect(mockMutate).toHaveBeenCalledWith(
        {applicationId: 'test-app-id'},
        expect.objectContaining({
          // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
          onSuccess: expect.any(Function),
          // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
          onError: expect.any(Function),
        }),
      );
    });

    it('should not initiate regeneration when applicationId is null', () => {
      renderDialog({...defaultProps, applicationId: null});

      const regenerateButton = screen.getByRole('button', {name: 'Regenerate'});

      expect(regenerateButton).toBeDisabled();
    });
  });

  describe('Success Flow', () => {
    it('should call onSuccess with new client secret after successful regeneration', async () => {
      // Mock mutate to immediately call onSuccess
      mockMutate.mockImplementation(
        (
          vars: RegenerateSecretVariables,
          options?: MutateOptions<RegenerateSecretResult, Error, RegenerateSecretVariables>,
        ) => {
          const mockContext = {} as MutationFunctionContext;
          options?.onSuccess?.(
            {clientSecret: 'new-test-secret-123'} as RegenerateSecretResult,
            vars,
            undefined,
            mockContext,
          );
        },
      );

      const user = userEvent.setup();
      renderDialog();

      const regenerateButton = screen.getByRole('button', {name: 'Regenerate'});
      await user.click(regenerateButton);

      await waitFor(() => {
        expect(mockOnClose).toHaveBeenCalled();
        expect(mockOnSuccess).toHaveBeenCalledWith('new-test-secret-123');
      });
    });
  });

  describe('Error Handling', () => {
    it('should display error message when regeneration fails', async () => {
      // Mock mutate to immediately call onError
      mockMutate.mockImplementation(
        (
          vars: RegenerateSecretVariables,
          options?: MutateOptions<RegenerateSecretResult, Error, RegenerateSecretVariables>,
        ) => {
          const mockContext = {} as MutationFunctionContext;
          options?.onError?.(
            new Error('Failed to regenerate client secret. Please try again.'),
            vars,
            undefined,
            mockContext,
          );
        },
      );

      const user = userEvent.setup();
      renderDialog();

      const regenerateButton = screen.getByRole('button', {name: 'Regenerate'});
      await user.click(regenerateButton);

      await waitFor(() => {
        expect(screen.getByText('Failed to regenerate client secret. Please try again.')).toBeInTheDocument();
      });
    });

    it('should display the mapped error message when the API returns a known error code', async () => {
      mockMutate.mockImplementation(
        (
          vars: RegenerateSecretVariables,
          options?: MutateOptions<RegenerateSecretResult, Error, RegenerateSecretVariables>,
        ) => {
          const mockContext = {} as MutationFunctionContext;
          const error = new Error('Request failed') as Error & {response?: {data?: {code: string}}};
          error.response = {data: {code: 'APP-1030'}};
          options?.onError?.(error, vars, undefined, mockContext);
        },
      );

      const user = userEvent.setup();
      renderDialog();

      const regenerateButton = screen.getByRole('button', {name: 'Regenerate'});
      await user.click(regenerateButton);

      await waitFor(() => {
        expect(
          screen.getByText('This application is managed declaratively and cannot be edited or deleted.'),
        ).toBeInTheDocument();
      });
    });

    it('should call onError callback when regeneration fails', async () => {
      // Mock mutate to immediately call onError
      mockMutate.mockImplementation(
        (
          vars: RegenerateSecretVariables,
          options?: MutateOptions<RegenerateSecretResult, Error, RegenerateSecretVariables>,
        ) => {
          const mockContext = {} as MutationFunctionContext;
          options?.onError?.(
            new Error('Failed to regenerate client secret. Please try again.'),
            vars,
            undefined,
            mockContext,
          );
        },
      );

      const user = userEvent.setup();
      renderDialog();

      const regenerateButton = screen.getByRole('button', {name: 'Regenerate'});
      await user.click(regenerateButton);

      await waitFor(() => {
        expect(mockOnError).toHaveBeenCalledWith('Failed to regenerate client secret. Please try again.');
      });
    });
  });
});
