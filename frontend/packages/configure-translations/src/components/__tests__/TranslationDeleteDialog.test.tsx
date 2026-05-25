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
import {render, screen} from '@thunderid/test-utils';
import {describe, expect, it, vi, beforeEach} from 'vitest';
import TranslationDeleteDialog from '@/components/TranslationDeleteDialog';

const mockMutate = vi.fn();
vi.mock('@thunderid/i18n', () => ({
  useDeleteTranslations: () => ({mutate: mockMutate, isPending: false}),
  getDisplayNameForCode: (code: string) => `DisplayName(${code})`,
}));

const defaultProps = {
  open: true,
  language: 'fr-FR',
  onClose: vi.fn(),
  onSuccess: vi.fn(),
};

describe('TranslationDeleteDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('renders the dialog title', () => {
      render(<TranslationDeleteDialog {...defaultProps} />);

      expect(screen.getByText('Delete Language')).toBeInTheDocument();
    });

    it('renders the confirmation message', () => {
      render(<TranslationDeleteDialog {...defaultProps} />);

      expect(screen.getByText(/Are you sure you want to delete all custom translations/)).toBeInTheDocument();
    });

    it('renders the warning disclaimer', () => {
      render(<TranslationDeleteDialog {...defaultProps} />);

      expect(
        screen.getByText(
          'All custom translations for this language will be permanently removed and reset to defaults.',
        ),
      ).toBeInTheDocument();
    });

    it('renders cancel and delete buttons', () => {
      render(<TranslationDeleteDialog {...defaultProps} />);

      expect(screen.getByText('Cancel')).toBeInTheDocument();
      expect(screen.getByText('Delete')).toBeInTheDocument();
    });
  });

  describe('Cancel', () => {
    it('calls onClose when cancel is clicked', async () => {
      const onClose = vi.fn();
      const user = userEvent.setup();
      render(<TranslationDeleteDialog {...defaultProps} onClose={onClose} />);

      await user.click(screen.getByText('Cancel'));

      expect(onClose).toHaveBeenCalled();
    });
  });

  describe('Delete', () => {
    it('calls deleteTranslations.mutate with the language when delete is clicked', async () => {
      const user = userEvent.setup();
      render(<TranslationDeleteDialog {...defaultProps} />);

      await user.click(screen.getByText('Delete'));

      expect(mockMutate).toHaveBeenCalledWith(
        'fr-FR',
        expect.objectContaining({
          onSuccess: expect.any(Function) as unknown as () => void,
          onError: expect.any(Function) as unknown as () => void,
        }),
      );
    });

    it('does not call mutate when language is null', async () => {
      const user = userEvent.setup();
      render(<TranslationDeleteDialog {...defaultProps} language={null} />);

      await user.click(screen.getByText('Delete'));

      expect(mockMutate).not.toHaveBeenCalled();
    });

    it('calls onClose and onSuccess on successful deletion', async () => {
      const onClose = vi.fn();
      const onSuccess = vi.fn();
      mockMutate.mockImplementation((_lang: string, opts: {onSuccess: () => void}) => {
        opts.onSuccess();
      });
      const user = userEvent.setup();
      render(<TranslationDeleteDialog {...defaultProps} onClose={onClose} onSuccess={onSuccess} />);

      await user.click(screen.getByText('Delete'));

      expect(onClose).toHaveBeenCalled();
      expect(onSuccess).toHaveBeenCalled();
    });

    it('shows an error alert on deletion failure', async () => {
      mockMutate.mockImplementation((_lang: string, opts: {onError: () => void}) => {
        opts.onError();
      });
      const user = userEvent.setup();
      render(<TranslationDeleteDialog {...defaultProps} />);

      await user.click(screen.getByText('Delete'));

      expect(screen.getByText('Failed to delete translations. Please try again.')).toBeInTheDocument();
    });
  });
});
