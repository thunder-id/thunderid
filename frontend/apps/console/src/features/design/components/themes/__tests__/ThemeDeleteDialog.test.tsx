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

import userEvent from '@testing-library/user-event';
import {render, screen, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import ThemeDeleteDialog from '../ThemeDeleteDialog';

const mockMutate = vi.fn();

vi.mock('react-i18next', async () => {
  const actual = await vi.importActual<typeof import('react-i18next')>('react-i18next');
  return {
    ...actual,
    useTranslation: () => ({
      t: (key: string, opts?: string | Record<string, unknown>) =>
        typeof opts === 'object' && opts !== null && 'name' in opts ? String(opts.name) : key,
    }),
  };
});

vi.mock('@thunderid/design', () => ({
  useDeleteTheme: vi.fn(() => ({
    mutate: mockMutate,
    isPending: false,
  })),
  useGetThemeUsages: vi.fn(() => ({
    data: undefined,
    isLoading: false,
  })),
}));

describe('ThemeDeleteDialog', () => {
  beforeEach(() => {
    mockMutate.mockReset();
  });

  describe('Rendering', () => {
    it('renders Dialog when open is true', () => {
      render(<ThemeDeleteDialog themeName="Ocean Blue" open themeId="theme-1" onClose={vi.fn()} />);
      // Dialog is open — some content is visible
      expect(screen.getByRole('dialog')).toBeInTheDocument();
    });

    it('does not render dialog content when open is false', () => {
      render(<ThemeDeleteDialog themeName="Ocean Blue" open={false} themeId="theme-1" onClose={vi.fn()} />);
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });

    it('renders the theme name in the dialog', () => {
      render(<ThemeDeleteDialog themeName="Ocean Blue" open themeId="theme-1" onClose={vi.fn()} />);
      expect(screen.getByText(/Ocean Blue/)).toBeInTheDocument();
    });

    it('renders without crashing when themeName is undefined', () => {
      render(<ThemeDeleteDialog themeName={null} open themeId="theme-1" onClose={vi.fn()} />);
      expect(screen.getByRole('dialog')).toBeInTheDocument();
    });

    it('renders a delete button', () => {
      render(<ThemeDeleteDialog open themeId="theme-1" themeName="Test" onClose={vi.fn()} />);
      // Should contain a destructive/delete action button (label comes from i18n key)
      expect(screen.getAllByRole('button').length).toBeGreaterThan(0);
    });
  });

  describe('Cancel behaviour', () => {
    it('calls onClose when the cancel button is clicked', async () => {
      const onClose = vi.fn();
      const user = userEvent.setup();
      render(<ThemeDeleteDialog open themeId="theme-1" themeName="My Theme" onClose={onClose} />);

      // Find cancel button by its translation key text
      const cancelBtn = screen.getByText('common:actions.cancel');
      await user.click(cancelBtn);

      expect(onClose).toHaveBeenCalledOnce();
    });

    it('does not call mutate when cancel is clicked', async () => {
      const user = userEvent.setup();
      render(<ThemeDeleteDialog open themeId="theme-1" themeName="My Theme" onClose={vi.fn()} />);

      const cancelBtn = screen.getByText('common:actions.cancel');
      await user.click(cancelBtn);

      expect(mockMutate).not.toHaveBeenCalled();
    });
  });

  describe('Delete behaviour', () => {
    it('calls mutate with the themeId when delete is confirmed', async () => {
      const user = userEvent.setup();
      render(<ThemeDeleteDialog open themeId="theme-abc" themeName="My Theme" onClose={vi.fn()} />);

      const deleteBtn = screen.getByText('common:actions.delete');
      await user.click(deleteBtn);

      expect(mockMutate).toHaveBeenCalledWith('theme-abc', expect.any(Object));
    });

    it('does not call mutate when themeId is undefined', () => {
      render(<ThemeDeleteDialog themeId={null} themeName="My Theme" open onClose={vi.fn()} />);

      // When themeId is null the delete button is disabled, preventing any click
      const deleteBtn = screen.getByRole('button', {name: 'common:actions.delete'});
      expect(deleteBtn).toBeDisabled();
      expect(mockMutate).not.toHaveBeenCalled();
    });
  });

  describe('Loading state', () => {
    it('disables buttons when isPending is true', async () => {
      const {useDeleteTheme} = await import('@thunderid/design');
      (useDeleteTheme as ReturnType<typeof vi.fn>).mockReturnValueOnce({
        mutate: mockMutate,
        isPending: true,
      });

      render(<ThemeDeleteDialog open themeId="theme-1" themeName="Test" onClose={vi.fn()} />);

      const buttons = screen.getAllByRole('button');
      const disabledButtons = buttons.filter((btn) => btn.hasAttribute('disabled'));
      expect(disabledButtons.length).toBeGreaterThan(0);
    });
  });

  describe('Success callback', () => {
    it('calls onSuccess after successful deletion', async () => {
      const onSuccess = vi.fn();
      const onClose = vi.fn();

      // Simulate mutate calling onSuccess callback
      mockMutate.mockImplementation((_: unknown, callbacks: {onSuccess?: () => void}) => {
        callbacks?.onSuccess?.();
      });

      const user = userEvent.setup();
      render(<ThemeDeleteDialog open themeId="theme-1" themeName="My Theme" onClose={onClose} onSuccess={onSuccess} />);

      const deleteBtn = screen.getByText('common:actions.delete');
      await user.click(deleteBtn);

      await waitFor(() => {
        expect(onSuccess).toHaveBeenCalledOnce();
        expect(onClose).toHaveBeenCalledOnce();
      });
    });
  });
});
