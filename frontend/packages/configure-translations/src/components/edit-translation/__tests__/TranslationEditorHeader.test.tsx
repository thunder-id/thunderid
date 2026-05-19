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
import TranslationEditorHeader from '@/components/edit-translation/TranslationEditorHeader';

vi.mock('@thunderid/i18n', () => ({
  getDisplayNameForCode: (code: string) => `Language(${code})`,
  toFlagEmoji: (code: string) => `Flag(${code})`,
}));

const defaultProps = {
  selectedLanguage: null,
  hasDirtyChanges: false,
  dirtyCount: 0,
  isSaving: false,
  isFallbackLanguage: false,
  hasNamespace: true,
  onBack: vi.fn(),
  onDiscard: vi.fn(),
  onResetToDefault: vi.fn(),
  onSave: vi.fn(),
};

describe('TranslationEditorHeader', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('shows page title key when no language is selected', () => {
      render(<TranslationEditorHeader {...defaultProps} selectedLanguage={null} />);

      expect(screen.getByText('Translations')).toBeInTheDocument();
    });

    it('shows flag and display name when a language is selected', () => {
      render(<TranslationEditorHeader {...defaultProps} selectedLanguage="fr-FR" />);

      expect(screen.getByText('Flag(fr-FR)')).toBeInTheDocument();
      expect(screen.getByText('Language(fr-FR)')).toBeInTheDocument();
    });

    it('renders discard, save, and reset-to-default action buttons', () => {
      render(<TranslationEditorHeader {...defaultProps} isFallbackLanguage={false} />);

      expect(screen.getByText('Discard Changes')).toBeInTheDocument();
      expect(screen.getByText('Reset to Default')).toBeInTheDocument();
      expect(screen.getByText('Save Changes')).toBeInTheDocument();
    });

    it('hides Reset to Default button when isFallbackLanguage is true', () => {
      render(<TranslationEditorHeader {...defaultProps} isFallbackLanguage />);

      expect(screen.queryByText('Reset to Default')).not.toBeInTheDocument();
    });

    it('shows Reset to Default button when isFallbackLanguage is false', () => {
      render(<TranslationEditorHeader {...defaultProps} isFallbackLanguage={false} />);

      expect(screen.getByText('Reset to Default')).toBeInTheDocument();
    });
  });

  describe('Dirty-changes indicator', () => {
    it('does not show unsaved count when there are no dirty changes', () => {
      render(<TranslationEditorHeader {...defaultProps} hasDirtyChanges={false} dirtyCount={0} />);

      expect(screen.queryByText(/unsaved change/)).not.toBeInTheDocument();
    });

    it('shows unsaved count label when there are dirty changes', () => {
      render(<TranslationEditorHeader {...defaultProps} hasDirtyChanges dirtyCount={3} />);

      expect(screen.getByText('3 unsaved change')).toBeInTheDocument();
    });
  });

  describe('Button disabled states', () => {
    it('disables Discard when no dirty changes', () => {
      render(<TranslationEditorHeader {...defaultProps} hasDirtyChanges={false} />);

      expect(screen.getByText('Discard Changes').closest('button')).toBeDisabled();
    });

    it('enables Discard when dirty changes exist', () => {
      render(<TranslationEditorHeader {...defaultProps} hasDirtyChanges dirtyCount={1} />);

      expect(screen.getByText('Discard Changes').closest('button')).not.toBeDisabled();
    });

    it('disables Save when no dirty changes', () => {
      render(<TranslationEditorHeader {...defaultProps} hasDirtyChanges={false} />);

      expect(screen.getByText('Save Changes').closest('button')).toBeDisabled();
    });

    it('enables Save when dirty changes exist', () => {
      render(<TranslationEditorHeader {...defaultProps} hasDirtyChanges dirtyCount={2} />);

      expect(screen.getByText('Save Changes').closest('button')).not.toBeDisabled();
    });

    it('disables all action buttons while saving', () => {
      render(<TranslationEditorHeader {...defaultProps} hasDirtyChanges dirtyCount={1} isSaving />);

      expect(screen.getByText('Discard Changes').closest('button')).toBeDisabled();
      expect(screen.getByText('Save Changes').closest('button')).toBeDisabled();
    });

    it('disables Reset to Default when hasNamespace is false', () => {
      render(<TranslationEditorHeader {...defaultProps} isFallbackLanguage={false} hasNamespace={false} />);

      expect(screen.getByText('Reset to Default').closest('button')).toBeDisabled();
    });

    it('enables Reset to Default when hasNamespace is true and not saving', () => {
      render(<TranslationEditorHeader {...defaultProps} isFallbackLanguage={false} hasNamespace isSaving={false} />);

      expect(screen.getByText('Reset to Default').closest('button')).not.toBeDisabled();
    });
  });

  describe('Callbacks', () => {
    it('calls onBack when the back button is clicked', async () => {
      const onBack = vi.fn();
      const user = userEvent.setup();

      render(<TranslationEditorHeader {...defaultProps} onBack={onBack} />);

      // The back button is an IconButton (first button rendered)
      await user.click(screen.getAllByRole('button')[0]);

      expect(onBack).toHaveBeenCalledTimes(1);
    });

    it('calls onDiscard when Discard button is clicked', async () => {
      const onDiscard = vi.fn();
      const user = userEvent.setup();

      render(<TranslationEditorHeader {...defaultProps} hasDirtyChanges dirtyCount={1} onDiscard={onDiscard} />);

      await user.click(screen.getByText('Discard Changes'));

      expect(onDiscard).toHaveBeenCalledTimes(1);
    });

    it('calls onSave when Save button is clicked', async () => {
      const onSave = vi.fn();
      const user = userEvent.setup();

      render(<TranslationEditorHeader {...defaultProps} hasDirtyChanges dirtyCount={1} onSave={onSave} />);

      await user.click(screen.getByText('Save Changes'));

      expect(onSave).toHaveBeenCalledTimes(1);
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

      await user.click(screen.getByText('Reset to Default'));

      expect(onResetToDefault).toHaveBeenCalledTimes(1);
    });
  });
});
