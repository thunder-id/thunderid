// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {FlowNodeInput} from '../../../../../models/responses';
import ExecutorInputsEditor from '../ExecutorInputsEditor';

// Mock react-i18next
vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

// Mock generateResourceId to return deterministic values in tests
vi.mock('../../../../../utils/generateResourceId', () => ({
  default: (prefix: string) => `${prefix}-test-id`,
}));

describe('ExecutorInputsEditor', () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    mockOnChange.mockClear();
  });

  describe('Empty State', () => {
    it('should render the title and info alert when inputs are empty', () => {
      render(<ExecutorInputsEditor inputs={[]} onChange={mockOnChange} />);

      expect(screen.getByText('flows:core.executions.inputs.title')).toBeInTheDocument();
      expect(screen.getByText('flows:core.executions.inputs.empty')).toBeInTheDocument();
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });

    it('should render the add button when inputs are empty', () => {
      render(<ExecutorInputsEditor inputs={[]} onChange={mockOnChange} />);

      expect(screen.getByText('flows:core.executions.inputs.add')).toBeInTheDocument();
    });

    it('should handle undefined inputs gracefully', () => {
      render(<ExecutorInputsEditor inputs={undefined as unknown as FlowNodeInput[]} onChange={mockOnChange} />);

      expect(screen.getByText('flows:core.executions.inputs.empty')).toBeInTheDocument();
    });

    it('should handle non-array inputs gracefully', () => {
      render(<ExecutorInputsEditor inputs={'invalid' as unknown as FlowNodeInput[]} onChange={mockOnChange} />);

      expect(screen.getByText('flows:core.executions.inputs.empty')).toBeInTheDocument();
    });
  });

  describe('Rendering Inputs', () => {
    const twoInputs: FlowNodeInput[] = [
      {type: 'TEXT_INPUT', identifier: 'username', required: true},
      {type: 'PASSWORD_INPUT', identifier: 'password', required: true},
    ];

    it('should render input rows for each input', () => {
      render(<ExecutorInputsEditor inputs={twoInputs} onChange={mockOnChange} />);

      const textboxes = screen.getAllByRole('textbox');
      expect(textboxes).toHaveLength(3);
      expect(textboxes[0]).toHaveValue('username');
      expect(textboxes[2]).toHaveValue('password');
    });

    it('should not render the info alert when inputs exist', () => {
      render(<ExecutorInputsEditor inputs={twoInputs} onChange={mockOnChange} />);

      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });

    it('should render remove buttons for each input', () => {
      render(<ExecutorInputsEditor inputs={twoInputs} onChange={mockOnChange} />);

      expect(screen.getAllByLabelText('flows:core.executions.inputs.remove')).toHaveLength(2);
    });

    it('should render checkboxes for each input', () => {
      render(<ExecutorInputsEditor inputs={twoInputs} onChange={mockOnChange} />);

      const checkboxes = screen.getAllByRole('checkbox');
      expect(checkboxes).toHaveLength(3);
      expect(checkboxes[0]).toBeChecked();
      expect(checkboxes[1]).not.toBeChecked();
      expect(checkboxes[2]).toBeChecked();
    });

    it('should render unchecked checkbox when required is false', () => {
      const inputs: FlowNodeInput[] = [{type: 'TEXT_INPUT', identifier: 'optional', required: false}];

      render(<ExecutorInputsEditor inputs={inputs} onChange={mockOnChange} />);

      expect(screen.getAllByRole('checkbox')[0]).not.toBeChecked();
    });
  });

  describe('Add Input', () => {
    it('should call onChange with a new TEXT_INPUT when add button is clicked', async () => {
      const user = userEvent.setup();

      render(<ExecutorInputsEditor inputs={[]} onChange={mockOnChange} />);

      await user.click(screen.getByText('flows:core.executions.inputs.add'));

      expect(mockOnChange).toHaveBeenCalledWith([
        {ref: 'input-test-id', type: 'TEXT_INPUT', identifier: '', required: true},
      ]);
    });

    it('should append to existing inputs when add button is clicked', async () => {
      const user = userEvent.setup();
      const existingInputs: FlowNodeInput[] = [{type: 'EMAIL_INPUT', identifier: 'email', required: true}];

      render(<ExecutorInputsEditor inputs={existingInputs} onChange={mockOnChange} />);

      await user.click(screen.getByText('flows:core.executions.inputs.add'));

      expect(mockOnChange).toHaveBeenCalledWith([
        {type: 'EMAIL_INPUT', identifier: 'email', required: true},
        {ref: 'input-test-id', type: 'TEXT_INPUT', identifier: '', required: true},
      ]);
    });
  });

  describe('Remove Input', () => {
    it('should call onChange without the removed input', async () => {
      const user = userEvent.setup();
      const inputs: FlowNodeInput[] = [
        {type: 'TEXT_INPUT', identifier: 'username', required: true},
        {type: 'PASSWORD_INPUT', identifier: 'password', required: true},
      ];

      render(<ExecutorInputsEditor inputs={inputs} onChange={mockOnChange} />);

      const removeButtons = screen.getAllByLabelText('flows:core.executions.inputs.remove');
      await user.click(removeButtons[0]);

      expect(mockOnChange).toHaveBeenCalledWith([{type: 'PASSWORD_INPUT', identifier: 'password', required: true}]);
    });

    it('should call onChange with empty array when last input is removed', async () => {
      const user = userEvent.setup();
      const inputs: FlowNodeInput[] = [{type: 'TEXT_INPUT', identifier: 'username', required: true}];

      render(<ExecutorInputsEditor inputs={inputs} onChange={mockOnChange} />);

      await user.click(screen.getByLabelText('flows:core.executions.inputs.remove'));

      expect(mockOnChange).toHaveBeenCalledWith([]);
    });
  });

  describe('Update Type', () => {
    it('should call onChange with updated type when type is changed', async () => {
      const user = userEvent.setup();
      const inputs: FlowNodeInput[] = [{type: 'TEXT_INPUT', identifier: 'field', required: true}];

      render(<ExecutorInputsEditor inputs={inputs} onChange={mockOnChange} />);

      // Open the select dropdown and pick a new type
      const selectButton = screen.getByRole('combobox');
      await user.click(selectButton);

      const passwordOption = screen.getByText('flows:core.executions.inputs.types.password');
      await user.click(passwordOption);

      expect(mockOnChange).toHaveBeenCalledWith([{type: 'PASSWORD_INPUT', identifier: 'field', required: true}]);
    });
  });

  describe('Update Identifier', () => {
    it('should call onChange with updated identifier on blur', () => {
      const inputs: FlowNodeInput[] = [{type: 'TEXT_INPUT', identifier: 'old_id', required: true}];

      render(<ExecutorInputsEditor inputs={inputs} onChange={mockOnChange} />);

      const textbox = screen.getAllByRole('textbox')[0];
      fireEvent.change(textbox, {target: {value: 'new_id'}});
      fireEvent.blur(textbox);

      expect(mockOnChange).toHaveBeenCalledWith([{type: 'TEXT_INPUT', identifier: 'new_id', required: true}]);
    });

    it('should not call onChange on blur when identifier is unchanged', () => {
      const inputs: FlowNodeInput[] = [{type: 'TEXT_INPUT', identifier: 'same_id', required: true}];

      render(<ExecutorInputsEditor inputs={inputs} onChange={mockOnChange} />);

      const textbox = screen.getAllByRole('textbox')[0];
      fireEvent.blur(textbox);

      expect(mockOnChange).not.toHaveBeenCalled();
    });
  });

  describe('Update Required', () => {
    it('should call onChange with toggled required when checkbox is clicked', async () => {
      const user = userEvent.setup();
      const inputs: FlowNodeInput[] = [{type: 'TEXT_INPUT', identifier: 'field', required: true}];

      render(<ExecutorInputsEditor inputs={inputs} onChange={mockOnChange} />);

      await user.click(screen.getAllByRole('checkbox')[0]);

      expect(mockOnChange).toHaveBeenCalledWith([{type: 'TEXT_INPUT', identifier: 'field', required: false}]);
    });

    it('should call onChange with required true when unchecked checkbox is clicked', async () => {
      const user = userEvent.setup();
      const inputs: FlowNodeInput[] = [{type: 'TEXT_INPUT', identifier: 'field', required: false}];

      render(<ExecutorInputsEditor inputs={inputs} onChange={mockOnChange} />);

      await user.click(screen.getAllByRole('checkbox')[0]);

      expect(mockOnChange).toHaveBeenCalledWith([{type: 'TEXT_INPUT', identifier: 'field', required: true}]);
    });
  });

  describe('Default value and constant', () => {
    const localeInput: FlowNodeInput = {type: 'TEXT_INPUT', identifier: 'locale', required: false};

    it('should show the default value and constant controls without any expanding', () => {
      render(<ExecutorInputsEditor inputs={[localeInput]} onChange={mockOnChange} />);

      expect(screen.getAllByRole('textbox')).toHaveLength(2);
      expect(screen.getAllByRole('checkbox')).toHaveLength(2);
      expect(screen.getByText('flows:core.executions.inputs.defaultLabel')).toBeInTheDocument();
      expect(screen.getByText('flows:core.executions.inputs.constant')).toBeInTheDocument();
    });

    it.each<[string, {defaultValue: string; constant?: boolean}]>([
      ['a default value', {defaultValue: 'en-US'}],
      ['the constant flag', {defaultValue: 'en-US', constant: true}],
    ])('should show the stored settings when the input has %s', (_label, extra) => {
      render(<ExecutorInputsEditor inputs={[{...localeInput, ...extra}]} onChange={mockOnChange} />);

      expect(screen.getAllByRole('textbox')[1]).toHaveValue('en-US');
      expect(screen.getAllByRole('checkbox')[1]).toHaveProperty('checked', extra.constant === true);
    });

    it.each([
      ['default value', 'flows:core.executions.inputs.defaultHint'],
      ['constant', 'flows:core.executions.inputs.constantHint'],
    ])('should explain the %s in a caption that is always shown', (_name, hint) => {
      render(<ExecutorInputsEditor inputs={[localeInput]} onChange={mockOnChange} />);

      expect(screen.getByText(hint)).toBeVisible();
      expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
    });

    it('should commit a typed default value on blur', () => {
      render(<ExecutorInputsEditor inputs={[localeInput]} onChange={mockOnChange} />);

      const defaultField = screen.getAllByRole('textbox')[1];
      fireEvent.change(defaultField, {target: {value: 'en-US'}});
      fireEvent.blur(defaultField);

      expect(mockOnChange).toHaveBeenCalledWith([{...localeInput, defaultValue: 'en-US'}]);
    });

    it('should remove the default property when the field is cleared', () => {
      render(<ExecutorInputsEditor inputs={[{...localeInput, defaultValue: 'en-US'}]} onChange={mockOnChange} />);

      const defaultField = screen.getAllByRole('textbox')[1];
      fireEvent.change(defaultField, {target: {value: ''}});
      fireEvent.blur(defaultField);

      expect(mockOnChange).toHaveBeenCalledWith([localeInput]);
    });

    it('should set the constant flag when the checkbox is checked', async () => {
      const user = userEvent.setup();

      render(<ExecutorInputsEditor inputs={[{...localeInput, defaultValue: 'en-US'}]} onChange={mockOnChange} />);

      await user.click(screen.getAllByRole('checkbox')[1]);

      expect(mockOnChange).toHaveBeenCalledWith([{...localeInput, defaultValue: 'en-US', constant: true}]);
    });

    it('should remove the constant property when the checkbox is unchecked', async () => {
      const user = userEvent.setup();

      render(
        <ExecutorInputsEditor
          inputs={[{...localeInput, defaultValue: 'en-US', constant: true}]}
          onChange={mockOnChange}
        />,
      );

      await user.click(screen.getAllByRole('checkbox')[1]);

      expect(mockOnChange).toHaveBeenCalledWith([{...localeInput, defaultValue: 'en-US'}]);
    });

    it('should mark the default field invalid for a constant without a default value', () => {
      render(<ExecutorInputsEditor inputs={[{...localeInput, constant: true}]} onChange={mockOnChange} />);

      expect(screen.getAllByRole('textbox')[1]).toHaveAttribute('aria-invalid', 'true');
    });

    it('should not mark the default field invalid for a constant with a default value', () => {
      render(
        <ExecutorInputsEditor
          inputs={[{...localeInput, defaultValue: 'en-US', constant: true}]}
          onChange={mockOnChange}
        />,
      );

      expect(screen.getAllByRole('textbox')[1]).toHaveAttribute('aria-invalid', 'false');
    });

    it('should show the validation message under the title', () => {
      render(
        <ExecutorInputsEditor
          inputs={[localeInput]}
          onChange={mockOnChange}
          errorMessage="One or more inputs are not properly configured."
        />,
      );

      expect(screen.getByText('One or more inputs are not properly configured.')).toBeInTheDocument();
    });

    it.each(['PASSWORD_INPUT', 'OTP_INPUT'])('should hide the default value and constant for %s', (type) => {
      render(<ExecutorInputsEditor inputs={[{type, identifier: 'secret', required: true}]} onChange={mockOnChange} />);

      expect(screen.queryByText('flows:core.executions.inputs.defaultLabel')).not.toBeInTheDocument();
      expect(screen.queryByText('flows:core.executions.inputs.constant')).not.toBeInTheDocument();
      expect(screen.queryByText('flows:core.executions.inputs.defaultHint')).not.toBeInTheDocument();
      expect(screen.queryByText('flows:core.executions.inputs.constantHint')).not.toBeInTheDocument();
      expect(screen.getAllByRole('textbox')).toHaveLength(1);
      expect(screen.getAllByRole('checkbox')).toHaveLength(1);
    });

    it('should drop the default and constant when the type changes to a sensitive type', async () => {
      const user = userEvent.setup();

      render(
        <ExecutorInputsEditor
          inputs={[{...localeInput, defaultValue: 'en-US', constant: true}]}
          onChange={mockOnChange}
        />,
      );

      await user.click(screen.getByRole('combobox'));
      await user.click(screen.getByText('flows:core.executions.inputs.types.password'));

      expect(mockOnChange).toHaveBeenCalledWith([{type: 'PASSWORD_INPUT', identifier: 'locale', required: false}]);
    });
  });

  describe('Boolean inputs', () => {
    const delegatedInput: FlowNodeInput = {type: 'BOOLEAN_INPUT', identifier: 'delegated', required: false};

    it('should offer the boolean type and show it for a boolean input', () => {
      render(<ExecutorInputsEditor inputs={[delegatedInput]} onChange={mockOnChange} />);

      expect(screen.getAllByRole('combobox')[0]).toHaveTextContent('flows:core.executions.inputs.types.boolean');
    });

    it('should pick the default from a list instead of typing it', () => {
      render(<ExecutorInputsEditor inputs={[delegatedInput]} onChange={mockOnChange} />);

      expect(screen.getAllByRole('textbox')).toHaveLength(1);
      expect(screen.getAllByRole('combobox')).toHaveLength(2);
      expect(screen.getAllByRole('combobox')[1]).toHaveTextContent('flows:core.executions.inputs.defaultNotSet');
    });

    it.each([
      ['True', 'true'],
      ['False', 'false'],
    ])('should commit %s as the default', async (label, expected) => {
      const user = userEvent.setup();

      render(<ExecutorInputsEditor inputs={[delegatedInput]} onChange={mockOnChange} />);
      await user.click(screen.getAllByRole('combobox')[1]);
      await user.click(screen.getByRole('option', {name: `flows:core.executions.inputs.default${label}`}));

      expect(mockOnChange).toHaveBeenCalledWith([{...delegatedInput, defaultValue: expected}]);
    });

    it('should remove the default when it is set back to not set', async () => {
      const user = userEvent.setup();

      render(<ExecutorInputsEditor inputs={[{...delegatedInput, defaultValue: 'true'}]} onChange={mockOnChange} />);
      await user.click(screen.getAllByRole('combobox')[1]);
      await user.click(screen.getByRole('option', {name: 'flows:core.executions.inputs.defaultNotSet'}));

      expect(mockOnChange).toHaveBeenCalledWith([delegatedInput]);
    });

    it('should show a stored default that uses another accepted spelling', () => {
      render(<ExecutorInputsEditor inputs={[{...delegatedInput, defaultValue: 'FALSE'}]} onChange={mockOnChange} />);

      expect(screen.getAllByRole('combobox')[1]).toHaveTextContent('flows:core.executions.inputs.defaultFalse');
    });

    it('should accept false as a constant default', () => {
      render(
        <ExecutorInputsEditor
          inputs={[{...delegatedInput, defaultValue: 'false', constant: true}]}
          onChange={mockOnChange}
        />,
      );

      expect(screen.getAllByRole('combobox')[1]).not.toHaveAttribute('aria-invalid', 'true');
    });

    it('should drop a text default that is not a boolean when the type changes to boolean', async () => {
      const user = userEvent.setup();

      render(
        <ExecutorInputsEditor
          inputs={[{type: 'TEXT_INPUT', identifier: 'locale', required: false, defaultValue: 'en-US'}]}
          onChange={mockOnChange}
        />,
      );
      await user.click(screen.getAllByRole('combobox')[0]);
      await user.click(screen.getByRole('option', {name: 'flows:core.executions.inputs.types.boolean'}));

      expect(mockOnChange).toHaveBeenCalledWith([{type: 'BOOLEAN_INPUT', identifier: 'locale', required: false}]);
    });

    it('should keep a boolean default when the type changes to boolean', async () => {
      const user = userEvent.setup();

      render(
        <ExecutorInputsEditor
          inputs={[{type: 'TEXT_INPUT', identifier: 'delegated', required: false, defaultValue: 'true'}]}
          onChange={mockOnChange}
        />,
      );
      await user.click(screen.getAllByRole('combobox')[0]);
      await user.click(screen.getByRole('option', {name: 'flows:core.executions.inputs.types.boolean'}));

      expect(mockOnChange).toHaveBeenCalledWith([
        {type: 'BOOLEAN_INPUT', identifier: 'delegated', required: false, defaultValue: 'true'},
      ]);
    });
  });

  describe('Identifier Sync', () => {
    it('should update local identifier when prop changes externally', () => {
      const inputs: FlowNodeInput[] = [{type: 'TEXT_INPUT', identifier: 'initial', required: true}];

      const {rerender} = render(<ExecutorInputsEditor inputs={inputs} onChange={mockOnChange} />);

      expect(screen.getAllByRole('textbox')[0]).toHaveValue('initial');

      const updatedInputs: FlowNodeInput[] = [{type: 'TEXT_INPUT', identifier: 'updated', required: true}];
      rerender(<ExecutorInputsEditor inputs={updatedInputs} onChange={mockOnChange} />);

      expect(screen.getAllByRole('textbox')[0]).toHaveValue('updated');
    });
  });
});
