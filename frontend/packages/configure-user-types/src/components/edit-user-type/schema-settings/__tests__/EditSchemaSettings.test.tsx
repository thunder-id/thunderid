// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor, userEvent} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {LibraryAttribute, SchemaPropertyInput} from '../../../../types/user-types';
import EditSchemaSettings from '../EditSchemaSettings';

const {mockAttributes} = vi.hoisted(() => ({
  mockAttributes: [
    {
      name: 'phone',
      displayName: 'Phone Number',
      type: 'string',
      required: false,
      unique: false,
      credential: false,
      enum: [],
      regex: '',
    },
    {
      name: 'username',
      displayName: 'Username',
      type: 'string',
      required: true,
      unique: true,
      credential: false,
      enum: [],
      regex: '',
    },
  ] as LibraryAttribute[],
}));

vi.mock('../../../../constants/attributes', () => ({default: mockAttributes}));

describe('EditSchemaSettings', () => {
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

  it.each(['email', 'mobile'] as const)('requires a regex for %s attributes', async (type) => {
    const user = userEvent.setup();
    const {rerender} = render(
      <EditSchemaSettings
        properties={[{...baseProperties[0], type, regex: '^custom$'}]}
        onPropertiesChange={mockOnPropertiesChange}
        userTypeHandle="test"
      />,
    );
    await user.click(screen.getByText('email'));
    const input = screen.getByDisplayValue('^custom$');
    expect(input).toBeRequired();
    await user.clear(input);
    rerender(
      <EditSchemaSettings
        properties={[{...baseProperties[0], type, regex: ''}]}
        onPropertiesChange={mockOnPropertiesChange}
        userTypeHandle="test"
      />,
    );
    expect(
      screen.getByText(
        type === 'email'
          ? 'Enter a regular expression for email attributes.'
          : 'Enter a regular expression for mobile attributes.',
      ),
    ).toBeInTheDocument();
  });

  it.each([
    ['email', false, 'other', 'email', false],
    ['email', true, 'other', 'email', true],
    ['email', true, '', 'email', false],
    ['email', true, ' ', 'email', false],
    ['email', true, 'other', 'mobile', false],
    ['mobile', false, 'other', 'mobile', false],
    ['mobile', true, 'other', 'mobile', true],
    ['mobile', true, '', 'mobile', false],
    ['mobile', true, ' ', 'mobile', false],
    ['mobile', true, 'other', 'email', false],
  ] as const)(
    'shows replacement instructions only when another named %s is available (primary: %s, alternative: %j %s)',
    async (type, primary, name, alternativeType, canReplace) => {
      const user = userEvent.setup();
      render(
        <EditSchemaSettings
          properties={[
            {...baseProperties[0], type, primary},
            {...baseProperties[0], id: '2', name, type: alternativeType, primary: false},
          ]}
          onPropertiesChange={mockOnPropertiesChange}
          userTypeHandle="test"
        />,
      );
      await user.click(screen.getByRole('button', {name: 'email'}));
      await user.hover(screen.getByLabelText('Primary').closest('label')!);

      const tooltip = await screen.findByRole('tooltip');
      expect(tooltip).toHaveTextContent('Used as the primary contact for users');
      if (canReplace) {
        expect(tooltip).toHaveTextContent('To change it, select Primary on another attribute of this type');
      } else {
        expect(tooltip).not.toHaveTextContent('To change it');
      }
    },
  );

  it.each(['email', 'mobile'] as const)('omits replacement instructions for a sole primary %s', async (type) => {
    const user = userEvent.setup();
    render(
      <EditSchemaSettings
        properties={[{...baseProperties[0], type, primary: true}]}
        onPropertiesChange={mockOnPropertiesChange}
        userTypeHandle="test"
      />,
    );
    await user.click(screen.getByRole('button', {name: 'email'}));
    await user.hover(screen.getByLabelText('Primary').closest('label')!);

    const tooltip = await screen.findByRole('tooltip');
    expect(tooltip).toHaveTextContent('Used as the primary contact for users');
    expect(tooltip).not.toHaveTextContent('To change it');
  });

  it('renders existing properties as collapsed rows', () => {
    const props = {
      properties: baseProperties,
      onPropertiesChange: mockOnPropertiesChange,
      userTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    // Row headers show the property name when there is no display name.
    expect(screen.getByText('email')).toBeInTheDocument();
    expect(screen.getByText('age')).toBeInTheDocument();

    // Bodies are collapsed by default, so the field inputs are not mounted.
    expect(screen.queryByDisplayValue('email')).not.toBeInTheDocument();
  });

  it('expands a row to reveal editable name and type fields', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [baseProperties[0]],
      onPropertiesChange: mockOnPropertiesChange,
      userTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    await user.click(screen.getByText('email'));

    const nameInput = await screen.findByDisplayValue('email');
    expect(nameInput).not.toBeDisabled();

    // Type select is editable too.
    expect(screen.getByRole('combobox')).not.toHaveAttribute('aria-disabled', 'true');
  });

  it('renders the required, unique and credential fields as editable', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [{...baseProperties[0], unique: true}],
      onPropertiesChange: mockOnPropertiesChange,
      userTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    await user.click(screen.getByText('email'));

    await screen.findByLabelText(/required/i);
    const requiredCheckbox = screen.getByLabelText(/required/i);
    const uniqueCheckbox = screen.getByLabelText(/unique/i);
    const credentialCheckbox = screen.getByLabelText(/credential/i);

    expect(requiredCheckbox).not.toBeDisabled();
    expect(uniqueCheckbox).toBeChecked();
    expect(uniqueCheckbox).not.toBeDisabled();
    expect(credentialCheckbox).not.toBeDisabled();
  });

  it('checking the credential checkbox applies immediately without confirmation', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [baseProperties[0]],
      onPropertiesChange: mockOnPropertiesChange,
      userTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    await user.click(screen.getByText('email'));

    const credentialCheckbox = await screen.findByLabelText(/credential/i);
    await user.click(credentialCheckbox);

    expect(screen.queryByText(/remove credential flag/i)).not.toBeInTheDocument();
    expect(mockOnPropertiesChange).toHaveBeenCalledWith(
      expect.arrayContaining([expect.objectContaining({credential: true})]),
    );
  });

  it('asks for confirmation before unchecking the credential checkbox', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [{...baseProperties[0], credential: true}],
      onPropertiesChange: mockOnPropertiesChange,
      userTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    await user.click(screen.getByText('email'));

    const credentialCheckbox = await screen.findByLabelText(/credential/i);
    expect(credentialCheckbox).toBeChecked();
    await user.click(credentialCheckbox);

    expect(await screen.findByText(/remove credential flag/i)).toBeInTheDocument();
    // The flag is not cleared until the dialog is confirmed.
    expect(mockOnPropertiesChange).not.toHaveBeenCalled();
  });

  it('keeps the credential flag when the removal dialog is cancelled', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [{...baseProperties[0], credential: true}],
      onPropertiesChange: mockOnPropertiesChange,
      userTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    await user.click(screen.getByText('email'));

    const credentialCheckbox = await screen.findByLabelText(/credential/i);
    await user.click(credentialCheckbox);

    await screen.findByText(/remove credential flag/i);
    await user.click(screen.getByRole('button', {name: /^cancel$/i}));

    await waitFor(() => {
      expect(screen.queryByText(/remove credential flag/i)).not.toBeInTheDocument();
    });
    expect(mockOnPropertiesChange).not.toHaveBeenCalled();
    expect(screen.getByLabelText(/credential/i)).toBeChecked();
  });

  it('clears the credential flag when the removal dialog is confirmed', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [{...baseProperties[0], credential: true}],
      onPropertiesChange: mockOnPropertiesChange,
      userTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    await user.click(screen.getByText('email'));

    const credentialCheckbox = await screen.findByLabelText(/credential/i);
    await user.click(credentialCheckbox);

    await screen.findByText(/remove credential flag/i);
    await user.click(screen.getByRole('button', {name: /remove credential/i}));

    await waitFor(() => {
      expect(screen.queryByText(/remove credential flag/i)).not.toBeInTheDocument();
    });
    expect(mockOnPropertiesChange).toHaveBeenCalledWith(
      expect.arrayContaining([expect.objectContaining({credential: false})]),
    );
  });

  it('toggles the required checkbox', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [baseProperties[0]],
      onPropertiesChange: mockOnPropertiesChange,
      userTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    await user.click(screen.getByText('email'));

    const requiredCheckbox = await screen.findByLabelText(/required/i);
    expect(requiredCheckbox).toBeChecked();

    await user.click(requiredCheckbox);

    expect(mockOnPropertiesChange).toHaveBeenCalledWith(
      expect.arrayContaining([expect.objectContaining({required: false})]),
    );
  });

  it('allows editing the regex pattern for a string property', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [baseProperties[0]],
      onPropertiesChange: mockOnPropertiesChange,
      userTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    await user.click(screen.getByText('email'));

    const regexInput = await screen.findByPlaceholderText(/e.g., \^/i);
    await user.click(regexInput);
    await user.paste('^[a-z]+$');

    expect(mockOnPropertiesChange).toHaveBeenCalledWith(
      expect.arrayContaining([expect.objectContaining({regex: '^[a-z]+$'})]),
    );
  });

  it('preserves enum values for an enum-typed property', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [{...baseProperties[0], type: 'enum' as const, enum: ['ACTIVE', 'INACTIVE']}],
      onPropertiesChange: mockOnPropertiesChange,
      userTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    await user.click(screen.getByText('email'));

    expect(await screen.findByText('ACTIVE')).toBeInTheDocument();
    expect(screen.getByText('INACTIVE')).toBeInTheDocument();
  });

  it('adds an enum value for an enum-typed property', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [{...baseProperties[0], type: 'enum' as const, enum: ['ACTIVE']}],
      onPropertiesChange: mockOnPropertiesChange,
      userTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    await user.click(screen.getByText('email'));

    const enumInput = await screen.findByPlaceholderText(/add value and press enter/i);
    await user.type(enumInput, 'PENDING');
    const addButton = screen.getByRole('button', {name: /^add$/i});
    await user.click(addButton);

    expect(mockOnPropertiesChange).toHaveBeenCalledWith(
      expect.arrayContaining([expect.objectContaining({enum: ['ACTIVE', 'PENDING']})]),
    );
  });

  it('does not add a duplicate enum value', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [{...baseProperties[0], type: 'enum' as const, enum: ['ACTIVE']}],
      onPropertiesChange: mockOnPropertiesChange,
      userTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    await user.click(screen.getByText('email'));

    const enumInput = await screen.findByPlaceholderText(/add value and press enter/i);
    await user.type(enumInput, 'ACTIVE');
    const addButton = screen.getByRole('button', {name: /^add$/i});
    await user.click(addButton);

    // onPropertiesChange should NOT have been called for a duplicate.
    expect(mockOnPropertiesChange).not.toHaveBeenCalled();
  });

  it('removes an enum value for an enum-typed property', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [{...baseProperties[0], type: 'enum' as const, enum: ['ACTIVE', 'INACTIVE']}],
      onPropertiesChange: mockOnPropertiesChange,
      userTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    await user.click(screen.getByText('email'));

    const activeChipLabel = await screen.findByText('ACTIVE');
    await user.click(activeChipLabel);
    await user.keyboard('[Backspace]');

    expect(mockOnPropertiesChange).toHaveBeenCalledWith(
      expect.arrayContaining([expect.objectContaining({enum: ['INACTIVE']})]),
    );
  });

  it('adds a selected basic attribute as a property with incremented id', async () => {
    const user = userEvent.setup();
    const props = {
      properties: baseProperties,
      onPropertiesChange: mockOnPropertiesChange,
      userTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    await user.click(screen.getByRole('button', {name: /phone number/i}));

    expect(mockOnPropertiesChange).toHaveBeenCalledWith(
      expect.arrayContaining([
        ...baseProperties,
        expect.objectContaining({
          id: '2',
          name: 'phone',
          type: 'string',
        }),
      ]),
    );
  });

  it('seeds required and unique flags from the picked attribute definition', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [],
      onPropertiesChange: mockOnPropertiesChange,
      userTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    await user.click(screen.getByRole('button', {name: /username/i}));

    expect(mockOnPropertiesChange).toHaveBeenCalledWith(
      expect.arrayContaining([
        expect.objectContaining({
          name: 'username',
          required: true,
          unique: true,
        }),
      ]),
    );
  });

  it('adds a blank property when Add Property is clicked', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [],
      onPropertiesChange: mockOnPropertiesChange,
      userTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    await user.click(screen.getByRole('button', {name: /^add property$/i}));

    expect(mockOnPropertiesChange).toHaveBeenCalledWith([expect.objectContaining({name: '', type: 'string'})]);
  });

  it('hides a picked attribute from the library once it is added', () => {
    const props = {
      properties: [{...baseProperties[0], name: 'phone'}],
      onPropertiesChange: mockOnPropertiesChange,
      userTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    expect(screen.queryByRole('button', {name: /phone number/i})).not.toBeInTheDocument();
    expect(screen.getByRole('button', {name: /username/i})).toBeInTheDocument();
  });

  it('propagates property name edits', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [baseProperties[0]],
      onPropertiesChange: mockOnPropertiesChange,
      userTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    await user.click(screen.getByText('email'));

    const nameInput = await screen.findByDisplayValue('email');
    expect(nameInput).not.toBeDisabled();

    await user.type(nameInput, 'x');

    expect(mockOnPropertiesChange).toHaveBeenCalledWith([expect.objectContaining({name: 'emailx'})]);
  });

  it('removes a property when the delete button is clicked', async () => {
    const user = userEvent.setup();
    const props = {
      properties: baseProperties,
      onPropertiesChange: mockOnPropertiesChange,
      userTypeHandle: 'test',
    };

    render(<EditSchemaSettings {...props} />);

    const removeButtons = screen.getAllByRole('button', {name: /remove property/i});
    await user.click(removeButtons[0]);

    await waitFor(() => {
      expect(mockOnPropertiesChange).toHaveBeenCalledWith([baseProperties[1]]);
    });
  });
  it.each([
    [false, true, ['Select one email attribute as the primary email.']],
    [true, false, ['Select one mobile attribute as the primary mobile.']],
    [
      false,
      false,
      ['Select one email attribute as the primary email.', 'Select one mobile attribute as the primary mobile.'],
    ],
    [true, true, []],
  ] as const)(
    'shows separate missing-primary messages (email selected: %s, mobile selected: %s)',
    (emailPrimary, mobilePrimary, messages) => {
      render(
        <EditSchemaSettings
          properties={[
            {...baseProperties[0], name: 'work', type: 'email', primary: emailPrimary},
            {...baseProperties[0], id: '2', name: 'recovery', type: 'email', primary: false},
            {...baseProperties[0], id: '3', name: 'phone', type: 'mobile', primary: mobilePrimary},
            {...baseProperties[0], id: '4', name: 'backupPhone', type: 'mobile', primary: false},
          ]}
          onPropertiesChange={mockOnPropertiesChange}
          userTypeHandle="test"
        />,
      );
      expect(screen.queryAllByRole('alert').map((alert) => alert.textContent)).toEqual(messages);
    },
  );

  it('shows Primary checked for a primary contact and prevents clearing it', async () => {
    const user = userEvent.setup();
    render(
      <EditSchemaSettings
        properties={[{...baseProperties[0], type: 'email', primary: true}]}
        onPropertiesChange={mockOnPropertiesChange}
        userTypeHandle="test"
      />,
    );
    await user.click(screen.getByText('email'));
    expect(screen.getByLabelText('Primary')).toBeChecked();
    expect(screen.getByLabelText('Primary')).toBeDisabled();
    expect(screen.queryByRole('checkbox', {name: /credential/i})).not.toBeInTheDocument();
  });

  it.each([
    ['email', 'Email', '^[^\\s@]+@[^\\s@]+\\.[^\\s@]+$'],
    ['mobile', 'Mobile', '^\\+[1-9]\\d{1,14}$'],
  ] as const)('applies the default regex when the type changes to %s', async (type, label, regex) => {
    const user = userEvent.setup();
    render(
      <EditSchemaSettings
        properties={[{...baseProperties[0], regex: '^[a-z]+$'}]}
        onPropertiesChange={mockOnPropertiesChange}
        userTypeHandle="test"
      />,
    );
    await user.click(screen.getByText('email'));
    await user.click(screen.getByRole('combobox'));
    await user.click(await screen.findByRole('option', {name: label}));
    expect(mockOnPropertiesChange).toHaveBeenCalledWith([expect.objectContaining({type, regex, enum: []})]);
  });

  it.each(['email', 'mobile'] as const)('hides allowed values for %s attributes', async (type) => {
    const user = userEvent.setup();
    render(
      <EditSchemaSettings
        properties={[{...baseProperties[0], type}]}
        onPropertiesChange={mockOnPropertiesChange}
        userTypeHandle="test"
      />,
    );
    await user.click(screen.getByText('email'));
    expect(screen.getByPlaceholderText(/e.g., \^/i)).toBeInTheDocument();
    expect(screen.queryByText(/Allowed Values/i)).not.toBeInTheDocument();
  });

  it('transfers Primary between email attributes without changing mobile selection', async () => {
    const user = userEvent.setup();
    const properties: SchemaPropertyInput[] = [
      {...baseProperties[0], type: 'email', primary: true},
      {...baseProperties[0], id: '2', name: 'recovery', type: 'email', primary: false},
      {...baseProperties[0], id: '3', name: 'phone', type: 'mobile', primary: true},
    ];
    render(
      <EditSchemaSettings properties={properties} onPropertiesChange={mockOnPropertiesChange} userTypeHandle="test" />,
    );
    await user.click(screen.getByText('recovery'));
    await user.click(screen.getByLabelText('Primary'));
    const next = mockOnPropertiesChange.mock.calls.at(-1)?.[0] as SchemaPropertyInput[];
    expect(next.find((property) => property.name === 'email')?.primary).toBe(false);
    expect(next.find((property) => property.name === 'recovery')?.primary).toBe(true);
    expect(next.find((property) => property.name === 'phone')?.primary).toBe(true);
  });
});
