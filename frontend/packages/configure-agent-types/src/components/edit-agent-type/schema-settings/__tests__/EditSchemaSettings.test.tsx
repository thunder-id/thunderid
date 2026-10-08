// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor, within, userEvent} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {SchemaPropertyInput} from '../../../../models/property-definition';
import EditSchemaSettings from '../EditSchemaSettings';

// I18nTextInput renders a complex i18n-aware widget; replace it with a simple text input.
vi.mock('@thunderid/components', () => ({
  I18nTextInput: ({value, onChange}: {value: string; onChange: (val: string) => void}) => (
    <input data-testid="i18n-text-input" value={value} onChange={(e) => onChange(e.target.value)} />
  ),
}));

describe('EditSchemaSettings (agent-type)', () => {
  const mockOnPropertiesChange = vi.fn();

  const baseProperties: SchemaPropertyInput[] = [
    {
      id: '0',
      name: 'email',
      displayName: '',
      type: 'string',
      required: true,
      unique: true,
      credential: false,
      enum: [],
      regex: '',
    },
    {
      id: '1',
      name: 'age',
      displayName: '',
      type: 'number',
      required: false,
      unique: false,
      credential: false,
      enum: [],
      regex: '',
    },
  ];

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('resets unique and credential when changing type to boolean', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [{...baseProperties[0], unique: true, credential: true}],
      onPropertiesChange: mockOnPropertiesChange,
      agentTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    const typeSelect = screen.getByRole('combobox');
    await user.click(typeSelect);
    const booleanOption = await screen.findByRole('option', {name: 'Boolean'});
    await user.click(booleanOption);

    expect(mockOnPropertiesChange).toHaveBeenCalledWith(
      expect.arrayContaining([
        expect.objectContaining({
          type: 'boolean',
          unique: false,
          credential: false,
        }),
      ]),
    );
  });

  it('preserves enum values when changing type to enum', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [{...baseProperties[0], enum: ['ACTIVE', 'INACTIVE']}],
      onPropertiesChange: mockOnPropertiesChange,
      agentTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    const typeSelect = screen.getByRole('combobox');
    await user.click(typeSelect);
    const enumOption = await screen.findByRole('option', {name: 'Enum'});
    await user.click(enumOption);

    expect(mockOnPropertiesChange).toHaveBeenCalledWith(
      expect.arrayContaining([
        expect.objectContaining({
          type: 'enum',
          enum: ['ACTIVE', 'INACTIVE'],
        }),
      ]),
    );
  });

  it('clears enum values when changing from enum to number', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [{...baseProperties[0], type: 'enum' as const, enum: ['A', 'B']}],
      onPropertiesChange: mockOnPropertiesChange,
      agentTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    const typeSelect = screen.getByRole('combobox');
    await user.click(typeSelect);
    const numberOption = await screen.findByRole('option', {name: 'Number'});
    await user.click(numberOption);

    expect(mockOnPropertiesChange).toHaveBeenCalledWith(
      expect.arrayContaining([
        expect.objectContaining({
          type: 'number',
          enum: [],
          regex: '',
        }),
      ]),
    );
  });

  it('does not add a duplicate enum value', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [{...baseProperties[0], type: 'enum' as const, enum: ['ACTIVE']}],
      onPropertiesChange: mockOnPropertiesChange,
      agentTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    const enumInput = screen.getByPlaceholderText(/add value and press enter/i);
    await user.type(enumInput, 'ACTIVE');
    const addButton = screen.getByRole('button', {name: /^add$/i});
    await user.click(addButton);

    expect(mockOnPropertiesChange).not.toHaveBeenCalled();
  });

  it('adds an enum value via Enter key', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [{...baseProperties[0], type: 'enum' as const, enum: []}],
      onPropertiesChange: mockOnPropertiesChange,
      agentTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    const enumInput = screen.getByPlaceholderText(/add value and press enter/i);
    await user.type(enumInput, 'PENDING{Enter}');

    expect(mockOnPropertiesChange).toHaveBeenCalledWith(
      expect.arrayContaining([expect.objectContaining({enum: ['PENDING']})]),
    );
  });

  it('removes an enum value when its chip is deleted', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [{...baseProperties[0], type: 'enum' as const, enum: ['ACTIVE', 'INACTIVE']}],
      onPropertiesChange: mockOnPropertiesChange,
      agentTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    const activeChip = screen.getByText('ACTIVE').closest('.MuiChip-root');
    const deleteIcon = within(activeChip as HTMLElement).getByTestId('CancelIcon');
    await user.click(deleteIcon);

    expect(mockOnPropertiesChange).toHaveBeenCalledWith(
      expect.arrayContaining([expect.objectContaining({enum: ['INACTIVE']})]),
    );
  });

  it('shows credential removal confirmation dialog when unchecking credential', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [{...baseProperties[0], credential: true}],
      onPropertiesChange: mockOnPropertiesChange,
      agentTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    const credentialCheckbox = screen.getByRole('checkbox', {name: /values will be hashed/i});
    await user.click(credentialCheckbox);

    await waitFor(() => {
      expect(screen.getByText(/removing the credential flag/i)).toBeInTheDocument();
    });
  });

  it('confirms credential removal via the dialog', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [{...baseProperties[0], credential: true}],
      onPropertiesChange: mockOnPropertiesChange,
      agentTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    await user.click(screen.getByRole('checkbox', {name: /values will be hashed/i}));

    const dialog = screen.getByRole('dialog');
    await user.click(within(dialog).getByRole('button', {name: /remove credential/i}));

    await waitFor(() => {
      expect(mockOnPropertiesChange).toHaveBeenCalledWith(
        expect.arrayContaining([expect.objectContaining({credential: false})]),
      );
    });
  });

  it('cancels credential removal via the dialog', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [{...baseProperties[0], credential: true}],
      onPropertiesChange: mockOnPropertiesChange,
      agentTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    await user.click(screen.getByRole('checkbox', {name: /values will be hashed/i}));

    const dialog = screen.getByRole('dialog');
    await user.click(within(dialog).getByRole('button', {name: /cancel/i}));

    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
    expect(mockOnPropertiesChange).not.toHaveBeenCalled();
  });

  it('enables credential and clears unique when checking credential', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [{...baseProperties[0], credential: false, unique: true}],
      onPropertiesChange: mockOnPropertiesChange,
      agentTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    const credentialCheckbox = screen.getByRole('checkbox', {name: /values will be hashed/i});
    await user.click(credentialCheckbox);

    expect(mockOnPropertiesChange).toHaveBeenCalledWith(
      expect.arrayContaining([expect.objectContaining({credential: true, unique: false})]),
    );
  });

  it('adds a new property with an incremented id', async () => {
    const user = userEvent.setup();
    const props = {
      properties: baseProperties,
      onPropertiesChange: mockOnPropertiesChange,
      agentTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    const addButton = screen.getByRole('button', {name: /add property/i});
    await user.click(addButton);

    expect(mockOnPropertiesChange).toHaveBeenCalledWith(
      expect.arrayContaining([...baseProperties, expect.objectContaining({id: '2', name: '', type: 'string'})]),
    );
  });

  it('does not show a remove button when there is only one property', () => {
    const props = {
      properties: [baseProperties[0]],
      onPropertiesChange: mockOnPropertiesChange,
      agentTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    expect(screen.queryByRole('button', {name: /remove property/i})).not.toBeInTheDocument();
  });

  it('removes a property when its remove button is clicked', async () => {
    const user = userEvent.setup();
    const props = {
      properties: baseProperties,
      onPropertiesChange: mockOnPropertiesChange,
      agentTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    const removeButtons = screen.getAllByRole('button', {name: /remove property/i});
    await user.click(removeButtons[0]);

    expect(mockOnPropertiesChange).toHaveBeenCalledWith([baseProperties[1]]);
  });

  it('changes the property name', async () => {
    const user = userEvent.setup({delay: null});
    const props = {
      properties: [baseProperties[0]],
      onPropertiesChange: mockOnPropertiesChange,
      agentTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    const nameInput = screen.getByPlaceholderText(/e\.g\., model, environment, team/i);
    await user.type(nameInput, 'X');

    expect(mockOnPropertiesChange).toHaveBeenCalledWith(
      expect.arrayContaining([expect.objectContaining({name: 'emailX'})]),
    );
  });

  it('toggles the required checkbox', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [{...baseProperties[0], required: false}],
      onPropertiesChange: mockOnPropertiesChange,
      agentTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    const requiredCheckbox = screen.getByRole('checkbox', {name: /agents must provide a value for this field/i});
    await user.click(requiredCheckbox);

    expect(mockOnPropertiesChange).toHaveBeenCalledWith(
      expect.arrayContaining([expect.objectContaining({required: true})]),
    );
  });

  it('toggles the unique checkbox for string type', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [{...baseProperties[0], unique: false}],
      onPropertiesChange: mockOnPropertiesChange,
      agentTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    const uniqueCheckbox = screen.getByRole('checkbox', {name: /each agent must have a distinct value/i});
    await user.click(uniqueCheckbox);

    expect(mockOnPropertiesChange).toHaveBeenCalledWith(
      expect.arrayContaining([expect.objectContaining({unique: true})]),
    );
  });

  it('updates the regex value for string type', async () => {
    const user = userEvent.setup({delay: null});
    const props = {
      properties: [baseProperties[0]],
      onPropertiesChange: mockOnPropertiesChange,
      agentTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    const regexInput = screen.getByPlaceholderText(/\^/i);
    await user.type(regexInput, '^');

    expect(mockOnPropertiesChange).toHaveBeenCalledWith(
      expect.arrayContaining([expect.objectContaining({regex: '^'})]),
    );
  });

  it('does not add an empty enum value', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [{...baseProperties[0], type: 'enum' as const, enum: []}],
      onPropertiesChange: mockOnPropertiesChange,
      agentTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    const addButton = screen.getByRole('button', {name: /^add$/i});
    await user.click(addButton);

    expect(mockOnPropertiesChange).not.toHaveBeenCalled();
  });
});
