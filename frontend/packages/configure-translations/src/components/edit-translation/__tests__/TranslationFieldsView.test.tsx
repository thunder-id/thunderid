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
import {render, screen, fireEvent} from '@thunderid/test-utils';
import {describe, expect, it, vi, beforeEach} from 'vitest';
import TranslationFieldsView from '@/components/edit-translation/TranslationFieldsView';

const sampleValues = {
  'actions.save': 'Save',
  'actions.cancel': 'Cancel',
  'page.title': 'My Page',
};

const defaultProps = {
  localValues: sampleValues,
  serverValues: sampleValues,
  search: '',
  isCustomNamespace: false,
  onChange: vi.fn(),
  onResetField: vi.fn(),
};

describe('TranslationFieldsView', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('renders a text field for each translation key', () => {
      render(<TranslationFieldsView {...defaultProps} />);

      expect(screen.getByDisplayValue('Save')).toBeInTheDocument();
      expect(screen.getByDisplayValue('Cancel')).toBeInTheDocument();
      expect(screen.getByDisplayValue('My Page')).toBeInTheDocument();
    });

    it('renders the translation key as a label above each field', () => {
      render(<TranslationFieldsView {...defaultProps} />);

      expect(screen.getByText('actions.save')).toBeInTheDocument();
      expect(screen.getByText('actions.cancel')).toBeInTheDocument();
    });

    it('shows no-keys message when localValues is empty', () => {
      render(<TranslationFieldsView {...defaultProps} localValues={{}} serverValues={{}} />);

      expect(screen.getByText('No translatable keys in this namespace.')).toBeInTheDocument();
    });
  });

  describe('Search filtering', () => {
    it('shows only keys matching the search query', () => {
      render(<TranslationFieldsView {...defaultProps} search="save" />);

      expect(screen.getByDisplayValue('Save')).toBeInTheDocument();
      expect(screen.queryByDisplayValue('Cancel')).not.toBeInTheDocument();
    });

    it('matches search against key names (case-insensitive)', () => {
      render(<TranslationFieldsView {...defaultProps} search="PAGE" />);

      expect(screen.getByDisplayValue('My Page')).toBeInTheDocument();
      expect(screen.queryByDisplayValue('Save')).not.toBeInTheDocument();
    });

    it('matches search against field values', () => {
      render(<TranslationFieldsView {...defaultProps} search="Cancel" />);

      expect(screen.getByDisplayValue('Cancel')).toBeInTheDocument();
      expect(screen.queryByDisplayValue('Save')).not.toBeInTheDocument();
    });

    it('shows no-results message when search matches nothing', () => {
      render(<TranslationFieldsView {...defaultProps} search="nonexistent" />);

      expect(screen.getByText('No matching translations.')).toBeInTheDocument();
    });
  });

  describe('Dirty field state', () => {
    it('does not show reset button for a clean field', () => {
      render(<TranslationFieldsView {...defaultProps} />);

      expect(screen.queryByRole('button')).not.toBeInTheDocument();
    });

    it('shows a reset button when a field has a local change', () => {
      render(
        <TranslationFieldsView
          {...defaultProps}
          localValues={{'actions.save': 'Enregistrer', 'actions.cancel': 'Cancel', 'page.title': 'My Page'}}
        />,
      );

      expect(screen.getByRole('button')).toBeInTheDocument();
    });

    it('shows reset buttons only for dirty fields', () => {
      render(
        <TranslationFieldsView
          {...defaultProps}
          localValues={{
            'actions.save': 'Enregistrer',
            'actions.cancel': 'Annuler',
            'page.title': 'My Page',
          }}
        />,
      );

      // Two fields are dirty (save, cancel), page.title is clean
      expect(screen.getAllByRole('button')).toHaveLength(2);
    });
  });

  describe('Interaction', () => {
    it('calls onChange with the key and new value when a field is edited', () => {
      const onChange = vi.fn();

      render(<TranslationFieldsView {...defaultProps} onChange={onChange} />);

      // The field is a controlled input (value driven by localValues prop), so
      // userEvent.type accumulates against the re-rendered prop value on each
      // keystroke. Use fireEvent.change to set an exact target value instead.
      fireEvent.change(screen.getByDisplayValue('Save'), {target: {value: 'Enregistrer'}});

      expect(onChange).toHaveBeenCalledWith('actions.save', 'Enregistrer');
    });

    it('calls onResetField with the key when the reset button is clicked', async () => {
      const onResetField = vi.fn();
      const user = userEvent.setup();

      render(
        <TranslationFieldsView
          {...defaultProps}
          localValues={{'actions.save': 'Enregistrer', 'actions.cancel': 'Cancel', 'page.title': 'My Page'}}
          onResetField={onResetField}
        />,
      );

      await user.click(screen.getByRole('button'));

      expect(onResetField).toHaveBeenCalledWith('actions.save');
    });
  });

  describe('Add Key (custom namespace)', () => {
    it('shows the Add Key button when isCustomNamespace is true', () => {
      render(<TranslationFieldsView {...defaultProps} isCustomNamespace />);

      expect(screen.getByText('Add Key')).toBeInTheDocument();
    });

    it('does not show the Add Key button when isCustomNamespace is false', () => {
      render(<TranslationFieldsView {...defaultProps} isCustomNamespace={false} />);

      expect(screen.queryByText('Add Key')).not.toBeInTheDocument();
    });

    it('shows the add key form when the Add Key button is clicked', async () => {
      const user = userEvent.setup();
      render(<TranslationFieldsView {...defaultProps} isCustomNamespace />);

      await user.click(screen.getByText('Add Key'));

      expect(screen.getByLabelText('Key')).toBeInTheDocument();
      expect(screen.getByLabelText('Value')).toBeInTheDocument();
    });

    it('calls onChange and closes the form when a new key is submitted', async () => {
      const onChange = vi.fn();
      const user = userEvent.setup();
      render(<TranslationFieldsView {...defaultProps} isCustomNamespace onChange={onChange} />);

      await user.click(screen.getByText('Add Key'));

      fireEvent.change(screen.getByPlaceholderText('e.g. my.translation.key'), {
        target: {value: 'new.key'},
      });
      fireEvent.change(screen.getByPlaceholderText('Translation value'), {
        target: {value: 'New Value'},
      });

      await user.click(screen.getByText('Add'));

      expect(onChange).toHaveBeenCalledWith('new.key', 'New Value');
      // Form should be closed, Add Key button visible again
      expect(screen.getByText('Add Key')).toBeInTheDocument();
    });

    it('closes the form and clears inputs when Cancel is clicked', async () => {
      const user = userEvent.setup();
      render(<TranslationFieldsView {...defaultProps} isCustomNamespace />);

      await user.click(screen.getByText('Add Key'));

      fireEvent.change(screen.getByPlaceholderText('e.g. my.translation.key'), {
        target: {value: 'some.key'},
      });

      await user.click(screen.getByRole('button', {name: 'Cancel'}));

      // Form should be closed, Add Key button visible again
      expect(screen.getByText('Add Key')).toBeInTheDocument();
    });

    it('shows a duplicate key error when the entered key already exists', async () => {
      const user = userEvent.setup();
      render(<TranslationFieldsView {...defaultProps} isCustomNamespace />);

      await user.click(screen.getByText('Add Key'));

      fireEvent.change(screen.getByPlaceholderText('e.g. my.translation.key'), {
        target: {value: 'actions.save'},
      });

      expect(screen.getByText('This key already exists.')).toBeInTheDocument();
    });

    it('disables the submit button when the key is empty', async () => {
      const user = userEvent.setup();
      render(<TranslationFieldsView {...defaultProps} isCustomNamespace />);

      await user.click(screen.getByText('Add Key'));

      const submitButton = screen.getByText('Add').closest('button');
      expect(submitButton).toBeDisabled();
    });

    it('disables the submit button when the key is a duplicate', async () => {
      const user = userEvent.setup();
      render(<TranslationFieldsView {...defaultProps} isCustomNamespace />);

      await user.click(screen.getByText('Add Key'));

      fireEvent.change(screen.getByPlaceholderText('e.g. my.translation.key'), {
        target: {value: 'actions.save'},
      });

      const submitButton = screen.getByText('Add').closest('button');
      expect(submitButton).toBeDisabled();
    });
  });
});
