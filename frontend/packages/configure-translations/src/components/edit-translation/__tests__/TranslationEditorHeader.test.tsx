// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, screen} from '@thunderid/test-utils';
import {describe, expect, it, vi, beforeEach} from 'vitest';
import TranslationEditorHeader from '@/components/edit-translation/TranslationEditorHeader';

vi.mock('@thunderid/i18n', () => ({
  getDisplayNameForCode: (code: string) => `Language(${code})`,
  toFlagEmoji: (code: string) => `Flag(${code})`,
}));

const defaultProps = {
  selectedLanguage: null,
  isSaving: false,
  isFallbackLanguage: false,
  hasNamespace: true,
  onBack: vi.fn(),
  onResetToDefault: vi.fn(),
};

describe('TranslationEditorHeader', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('shows page title when no language is selected', () => {
      render(<TranslationEditorHeader {...defaultProps} selectedLanguage={null} />);

      expect(screen.getByText('Translations')).toBeInTheDocument();
    });

    it('shows flag and display name when a language is selected', () => {
      render(<TranslationEditorHeader {...defaultProps} selectedLanguage="fr-FR" />);

      expect(screen.getByText('Flag(fr-FR)')).toBeInTheDocument();
      expect(screen.getByText('Language(fr-FR)')).toBeInTheDocument();
    });

    it('renders the back button', () => {
      render(<TranslationEditorHeader {...defaultProps} />);

      expect(screen.getByText('Back to translations')).toBeInTheDocument();
    });

    it('renders the reset-to-default action button', () => {
      render(<TranslationEditorHeader {...defaultProps} isFallbackLanguage={false} />);

      expect(screen.getByText('Reset to default')).toBeInTheDocument();
    });

    it('hides Reset to Default button when isFallbackLanguage is true', () => {
      render(<TranslationEditorHeader {...defaultProps} isFallbackLanguage />);

      expect(screen.queryByText('Reset to default')).not.toBeInTheDocument();
    });

    it('shows Reset to Default button when isFallbackLanguage is false', () => {
      render(<TranslationEditorHeader {...defaultProps} isFallbackLanguage={false} />);

      expect(screen.getByText('Reset to default')).toBeInTheDocument();
    });
  });

  describe('Button disabled states', () => {
    it('disables Reset to Default when hasNamespace is false', () => {
      render(<TranslationEditorHeader {...defaultProps} isFallbackLanguage={false} hasNamespace={false} />);

      expect(screen.getByText('Reset to default').closest('button')).toBeDisabled();
    });

    it('enables Reset to Default when hasNamespace is true and not saving', () => {
      render(<TranslationEditorHeader {...defaultProps} isFallbackLanguage={false} hasNamespace isSaving={false} />);

      expect(screen.getByText('Reset to default').closest('button')).not.toBeDisabled();
    });

    it('disables Reset to Default while saving', () => {
      render(<TranslationEditorHeader {...defaultProps} isFallbackLanguage={false} hasNamespace isSaving />);

      expect(screen.getByText('Reset to default').closest('button')).toBeDisabled();
    });
  });

  describe('Callbacks', () => {
    it('calls onBack when the back button is clicked', async () => {
      const onBack = vi.fn();
      const user = userEvent.setup();

      render(<TranslationEditorHeader {...defaultProps} onBack={onBack} />);

      await user.click(screen.getByText('Back to translations'));

      expect(onBack).toHaveBeenCalledTimes(1);
    });

    it('calls onResetToDefault when Reset to Default button is clicked', async () => {
      const onResetToDefault = vi.fn();
      const user = userEvent.setup();

      render(
        <TranslationEditorHeader
          {...defaultProps}
          isFallbackLanguage={false}
          hasNamespace
          onResetToDefault={onResetToDefault}
        />,
      );

      await user.click(screen.getByText('Reset to default'));

      expect(onResetToDefault).toHaveBeenCalledTimes(1);
    });
  });
});
