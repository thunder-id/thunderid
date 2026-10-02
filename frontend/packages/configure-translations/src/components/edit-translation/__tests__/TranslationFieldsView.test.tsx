// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, renderHook, screen, fireEvent} from '@thunderid/test-utils';
import {useTranslation} from 'react-i18next';
import {describe, expect, it, vi, beforeAll, beforeEach} from 'vitest';
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
  isKeyCreationAllowedNamespace: false,
  onChange: vi.fn(),
  onResetField: vi.fn(),
};

describe('TranslationFieldsView', () => {
  let t: (key: string) => string;

  beforeAll(() => {
    ({t} = renderHook(() => useTranslation('translations')).result.current);
  });

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

      expect(screen.getByText(t('editor.noKeys'))).toBeInTheDocument();
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

      expect(screen.getByText(t('editor.noResults'))).toBeInTheDocument();
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
    it('shows the Add Key button when isKeyCreationAllowedNamespace is true', () => {
      render(<TranslationFieldsView {...defaultProps} isKeyCreationAllowedNamespace />);

      expect(screen.getByText(t('editor.addKey'))).toBeInTheDocument();
    });

    it('does not show the Add Key button when isKeyCreationAllowedNamespace is false', () => {
      render(<TranslationFieldsView {...defaultProps} isKeyCreationAllowedNamespace={false} />);

      expect(screen.queryByText(t('editor.addKey'))).not.toBeInTheDocument();
    });

    it('shows the add key form when the Add Key button is clicked', async () => {
      const user = userEvent.setup();
      render(<TranslationFieldsView {...defaultProps} isKeyCreationAllowedNamespace />);

      await user.click(screen.getByText(t('editor.addKey')));

      expect(screen.getByLabelText(t('editor.addKey.keyLabel'))).toBeInTheDocument();
      expect(screen.getByLabelText(t('editor.addKey.valueLabel'))).toBeInTheDocument();
    });

    it('calls onChange and closes the form when a new key is submitted', async () => {
      const onChange = vi.fn();
      const user = userEvent.setup();
      render(<TranslationFieldsView {...defaultProps} isKeyCreationAllowedNamespace onChange={onChange} />);

      await user.click(screen.getByText(t('editor.addKey')));

      fireEvent.change(screen.getByPlaceholderText(t('editor.addKey.keyPlaceholder')), {
        target: {value: 'new.key'},
      });
      fireEvent.change(screen.getByPlaceholderText(t('editor.addKey.valuePlaceholder')), {
        target: {value: 'New Value'},
      });

      await user.click(screen.getByText(t('editor.addKey.submit')));

      expect(onChange).toHaveBeenCalledWith('new.key', 'New Value');
      // Form should be closed, Add Key button visible again
      expect(screen.getByText(t('editor.addKey'))).toBeInTheDocument();
    });

    it('closes the form and clears inputs when Cancel is clicked', async () => {
      const user = userEvent.setup();
      render(<TranslationFieldsView {...defaultProps} isKeyCreationAllowedNamespace />);

      await user.click(screen.getByText(t('editor.addKey')));

      fireEvent.change(screen.getByPlaceholderText(t('editor.addKey.keyPlaceholder')), {
        target: {value: 'some.key'},
      });

      await user.click(screen.getByRole('button', {name: t('editor.addKey.cancel')}));

      // Form should be closed, Add Key button visible again
      expect(screen.getByText(t('editor.addKey'))).toBeInTheDocument();
    });

    it('shows a duplicate key error when the entered key already exists', async () => {
      const user = userEvent.setup();
      render(<TranslationFieldsView {...defaultProps} isKeyCreationAllowedNamespace />);

      await user.click(screen.getByText(t('editor.addKey')));

      fireEvent.change(screen.getByPlaceholderText(t('editor.addKey.keyPlaceholder')), {
        target: {value: 'actions.save'},
      });

      expect(screen.getByText(t('editor.addKey.duplicateKey'))).toBeInTheDocument();
    });

    it('disables the submit button when the key is empty', async () => {
      const user = userEvent.setup();
      render(<TranslationFieldsView {...defaultProps} isKeyCreationAllowedNamespace />);

      await user.click(screen.getByText(t('editor.addKey')));

      const submitButton = screen.getByText(t('editor.addKey.submit')).closest('button');
      expect(submitButton).toBeDisabled();
    });

    it('disables the submit button when the key is a duplicate', async () => {
      const user = userEvent.setup();
      render(<TranslationFieldsView {...defaultProps} isKeyCreationAllowedNamespace />);

      await user.click(screen.getByText(t('editor.addKey')));

      fireEvent.change(screen.getByPlaceholderText(t('editor.addKey.keyPlaceholder')), {
        target: {value: 'actions.save'},
      });

      const submitButton = screen.getByText(t('editor.addKey.submit')).closest('button');
      expect(submitButton).toBeDisabled();
    });
  });
});
