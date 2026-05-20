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
import TranslationEditorCard from '@/components/edit-translation/TranslationEditorCard';

vi.mock('react-i18next', async () => {
  const actual = await vi.importActual<typeof import('react-i18next')>('react-i18next');
  return {
    ...actual,
    useTranslation: () => ({t: (key: string) => key}),
  };
});

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
  isCustomNamespace: false,
  colorMode: 'light' as const,
  onTabChange: vi.fn(),
  onSearchChange: vi.fn(),
  onFieldChange: vi.fn(),
  onResetField: vi.fn(),
  onJsonChange: vi.fn(),
};

describe('TranslationEditorCard', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Loading state', () => {
    it('shows a loading spinner and message while data is loading', () => {
      render(<TranslationEditorCard {...defaultProps} isLoading />);

      expect(screen.getByRole('progressbar')).toBeInTheDocument();
      expect(screen.getByText('editor.loading')).toBeInTheDocument();
    });

    it('hides the spinner once loading is complete', () => {
      render(<TranslationEditorCard {...defaultProps} isLoading={false} />);

      expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();
    });
  });

  describe('Tabs', () => {
    it('renders the Fields and Raw JSON tabs', () => {
      render(<TranslationEditorCard {...defaultProps} />);

      expect(screen.getByRole('tab', {name: 'editor.textFields'})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: 'editor.rawJson'})).toBeInTheDocument();
    });

    it('calls onTabChange when a tab is clicked', async () => {
      const onTabChange = vi.fn();
      const user = userEvent.setup();

      render(<TranslationEditorCard {...defaultProps} onTabChange={onTabChange} />);

      await user.click(screen.getByRole('tab', {name: 'editor.rawJson'}));

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

      expect(screen.getByPlaceholderText('editor.searchPlaceholder')).toBeInTheDocument();
    });

    it('calls onSearchChange when text is typed in the search input', async () => {
      const onSearchChange = vi.fn();
      const user = userEvent.setup();

      render(<TranslationEditorCard {...defaultProps} editView="fields" onSearchChange={onSearchChange} />);

      // The search input is controlled (value={search} prop stays ''), so
      // each keystroke fires onSearchChange with just that character.
      await user.type(screen.getByPlaceholderText('editor.searchPlaceholder'), 's');

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

      expect(screen.queryByPlaceholderText('editor.searchPlaceholder')).not.toBeInTheDocument();
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
