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
      expect(textboxes).toHaveLength(2);
      expect(textboxes[0]).toHaveValue('username');
      expect(textboxes[1]).toHaveValue('password');
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
      expect(checkboxes).toHaveLength(2);
      expect(checkboxes[0]).toBeChecked();
      expect(checkboxes[1]).toBeChecked();
    });

    it('should render unchecked checkbox when required is false', () => {
      const inputs: FlowNodeInput[] = [{type: 'TEXT_INPUT', identifier: 'optional', required: false}];

      render(<ExecutorInputsEditor inputs={inputs} onChange={mockOnChange} />);

      expect(screen.getByRole('checkbox')).not.toBeChecked();
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

      const textbox = screen.getByRole('textbox');
      fireEvent.change(textbox, {target: {value: 'new_id'}});
      fireEvent.blur(textbox);

      expect(mockOnChange).toHaveBeenCalledWith([{type: 'TEXT_INPUT', identifier: 'new_id', required: true}]);
    });

    it('should not call onChange on blur when identifier is unchanged', () => {
      const inputs: FlowNodeInput[] = [{type: 'TEXT_INPUT', identifier: 'same_id', required: true}];

      render(<ExecutorInputsEditor inputs={inputs} onChange={mockOnChange} />);

      const textbox = screen.getByRole('textbox');
      fireEvent.blur(textbox);

      expect(mockOnChange).not.toHaveBeenCalled();
    });
  });

  describe('Update Required', () => {
    it('should call onChange with toggled required when checkbox is clicked', async () => {
      const user = userEvent.setup();
      const inputs: FlowNodeInput[] = [{type: 'TEXT_INPUT', identifier: 'field', required: true}];

      render(<ExecutorInputsEditor inputs={inputs} onChange={mockOnChange} />);

      await user.click(screen.getByRole('checkbox'));

      expect(mockOnChange).toHaveBeenCalledWith([{type: 'TEXT_INPUT', identifier: 'field', required: false}]);
    });

    it('should call onChange with required true when unchecked checkbox is clicked', async () => {
      const user = userEvent.setup();
      const inputs: FlowNodeInput[] = [{type: 'TEXT_INPUT', identifier: 'field', required: false}];

      render(<ExecutorInputsEditor inputs={inputs} onChange={mockOnChange} />);

      await user.click(screen.getByRole('checkbox'));

      expect(mockOnChange).toHaveBeenCalledWith([{type: 'TEXT_INPUT', identifier: 'field', required: true}]);
    });
  });

  describe('Identifier Sync', () => {
    it('should update local identifier when prop changes externally', () => {
      const inputs: FlowNodeInput[] = [{type: 'TEXT_INPUT', identifier: 'initial', required: true}];

      const {rerender} = render(<ExecutorInputsEditor inputs={inputs} onChange={mockOnChange} />);

      expect(screen.getByRole('textbox')).toHaveValue('initial');

      const updatedInputs: FlowNodeInput[] = [{type: 'TEXT_INPUT', identifier: 'updated', required: true}];
      rerender(<ExecutorInputsEditor inputs={updatedInputs} onChange={mockOnChange} />);

      expect(screen.getByRole('textbox')).toHaveValue('updated');
    });
  });
});
