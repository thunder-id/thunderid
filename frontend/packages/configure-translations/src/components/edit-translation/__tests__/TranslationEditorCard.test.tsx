// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, renderHook, screen} from '@thunderid/test-utils';
import {useTranslation} from 'react-i18next';
import {describe, expect, it, vi, beforeAll, beforeEach} from 'vitest';
import TranslationEditorCard from '@/components/edit-translation/TranslationEditorCard';

vi.mock('@/components/edit-translation/TranslationFieldsView', () => ({
  default: () => <div data-testid="fields-view" />,
}));

vi.mock('@/components/edit-translation/TranslationJsonEditor', () => ({
  default: () => <div data-testid="json-editor" />,
}));

const defaultProps = {
  selectedLanguage: 'fr-FR',
  isLoading: false,
  editView: 'fields' as const,
  search: '',
  currentValues: {'actions.save': 'Enregistrer'},
  serverValues: {'actions.save': 'Enregistrer'},
  isKeyCreationAllowedNamespace: false,
  colorMode: 'light' as const,
  onTabChange: vi.fn(),
  onSearchChange: vi.fn(),
  onFieldChange: vi.fn(),
  onResetField: vi.fn(),
  onJsonChange: vi.fn(),
};

describe('TranslationEditorCard', () => {
  let t: (key: string) => string;

  beforeAll(() => {
    ({t} = renderHook(() => useTranslation('translations')).result.current);
  });

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Loading state', () => {
    it('shows a loading spinner and message while data is loading', () => {
      render(<TranslationEditorCard {...defaultProps} isLoading />);

      expect(screen.getByRole('progressbar')).toBeInTheDocument();
      expect(screen.getByText(t('editor.loading'))).toBeInTheDocument();
    });

    it('hides the spinner once loading is complete', () => {
      render(<TranslationEditorCard {...defaultProps} isLoading={false} />);

      expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();
    });
  });

  describe('Tabs', () => {
    it('renders the Fields and Raw JSON tabs', () => {
      render(<TranslationEditorCard {...defaultProps} />);

      expect(screen.getByRole('tab', {name: t('editor.textFields')})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: 'JSON'})).toBeInTheDocument();
    });

    it('calls onTabChange when a tab is clicked', async () => {
      const onTabChange = vi.fn();
      const user = userEvent.setup();

      render(<TranslationEditorCard {...defaultProps} onTabChange={onTabChange} />);

      await user.click(screen.getByRole('tab', {name: 'JSON'}));

      expect(onTabChange).toHaveBeenCalledTimes(1);
    });
  });

  describe('Fields view', () => {
    it('renders the fields view when editView is "fields"', () => {
      render(<TranslationEditorCard {...defaultProps} editView="fields" />);

      expect(screen.getByTestId('fields-view')).toBeInTheDocument();
    });

    it('renders the search input in fields view', () => {
      render(<TranslationEditorCard {...defaultProps} editView="fields" />);

      expect(screen.getByPlaceholderText(t('editor.searchPlaceholder'))).toBeInTheDocument();
    });

    it('calls onSearchChange when text is typed in the search input', async () => {
      const onSearchChange = vi.fn();
      const user = userEvent.setup();

      render(<TranslationEditorCard {...defaultProps} editView="fields" onSearchChange={onSearchChange} />);

      // The search input is controlled (value={search} prop stays ''), so
      // each keystroke fires onSearchChange with just that character.
      await user.type(screen.getByPlaceholderText(t('editor.searchPlaceholder')), 's');

      expect(onSearchChange).toHaveBeenCalledWith('s');
    });

    it('does not render the JSON editor when editView is "fields"', () => {
      render(<TranslationEditorCard {...defaultProps} editView="fields" />);

      expect(screen.queryByTestId('json-editor')).not.toBeInTheDocument();
    });
  });

  describe('JSON view', () => {
    it('renders the JSON editor when editView is "json"', () => {
      render(<TranslationEditorCard {...defaultProps} editView="json" />);

      expect(screen.getByTestId('json-editor')).toBeInTheDocument();
    });

    it('does not render the fields view when editView is "json"', () => {
      render(<TranslationEditorCard {...defaultProps} editView="json" />);

      expect(screen.queryByTestId('fields-view')).not.toBeInTheDocument();
    });

    it('does not render the search input in JSON view', () => {
      render(<TranslationEditorCard {...defaultProps} editView="json" />);

      expect(screen.queryByPlaceholderText(t('editor.searchPlaceholder'))).not.toBeInTheDocument();
    });
  });

  describe('No language selected', () => {
    it('does not render editor views when selectedLanguage is null', () => {
      render(<TranslationEditorCard {...defaultProps} selectedLanguage={null} editView="fields" />);

      expect(screen.queryByTestId('fields-view')).not.toBeInTheDocument();
      expect(screen.queryByTestId('json-editor')).not.toBeInTheDocument();
    });
  });
});
