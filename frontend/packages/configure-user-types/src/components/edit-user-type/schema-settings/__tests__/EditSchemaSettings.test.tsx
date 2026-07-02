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

import {render, screen, waitFor, userEvent} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Attribute, SchemaPropertyInput} from '../../../../types/user-types';
import EditSchemaSettings from '../EditSchemaSettings';

const {mockAttributes} = vi.hoisted(() => ({
  mockAttributes: [
    {id: 'phone', displayName: 'Phone Number', dataType: 'string', required: false, unique: false, credential: false},
    {id: 'username', displayName: 'Username', dataType: 'string', required: true, unique: true, credential: false},
  ] as Attribute[],
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

  it('renders existing properties as collapsed rows', () => {
    const props = {
      properties: baseProperties,
      onPropertiesChange: mockOnPropertiesChange,
      userTypeName: 'Test',
    };

    render(<EditSchemaSettings {...props} />);

    // Row headers show the property name when there is no display name.
    expect(screen.getByText('email')).toBeInTheDocument();
    expect(screen.getByText('age')).toBeInTheDocument();

    // Bodies are collapsed by default, so the field inputs are not mounted.
    expect(screen.queryByDisplayValue('email')).not.toBeInTheDocument();
  });

  it('expands a row to reveal locked name and type fields', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [baseProperties[0]],
      onPropertiesChange: mockOnPropertiesChange,
      userTypeName: 'Test',
    };

    render(<EditSchemaSettings {...props} />);

    await user.click(screen.getByText('email'));

    // The name is locked for a non-custom property.
    const nameInput = await screen.findByDisplayValue('email');
    expect(nameInput).toBeDisabled();

    // Type select stays locked to the definition.
    expect(screen.getByRole('combobox')).toHaveAttribute('aria-disabled', 'true');
  });

  it('shows the required, unique and credential fields as read-only except required', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [{...baseProperties[0], unique: true}],
      onPropertiesChange: mockOnPropertiesChange,
      userTypeName: 'Test',
    };

    render(<EditSchemaSettings {...props} />);

    await user.click(screen.getByText('email'));

    await screen.findByLabelText(/required/i);
    const requiredCheckbox = screen.getByLabelText(/required/i);
    const uniqueCheckbox = screen.getByLabelText(/unique/i);
    const credentialCheckbox = screen.getByLabelText(/credential/i);

    expect(requiredCheckbox).not.toBeDisabled();
    expect(uniqueCheckbox).toBeChecked();
    expect(uniqueCheckbox).toBeDisabled();
    expect(credentialCheckbox).toBeDisabled();
  });

  it('toggles the required checkbox', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [baseProperties[0]],
      onPropertiesChange: mockOnPropertiesChange,
      userTypeName: 'Test',
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
      userTypeName: 'Test',
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
      userTypeName: 'Test',
    };

    render(<EditSchemaSettings {...props} />);

    await user.click(screen.getByText('email'));

    expect(await screen.findByText('ACTIVE')).toBeInTheDocument();
    expect(screen.getByText('INACTIVE')).toBeInTheDocument();

    // Type select remains locked even for enum properties.
    expect(screen.getByRole('combobox')).toHaveAttribute('aria-disabled', 'true');
  });

  it('adds an enum value for an enum-typed property', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [{...baseProperties[0], type: 'enum' as const, enum: ['ACTIVE']}],
      onPropertiesChange: mockOnPropertiesChange,
      userTypeName: 'Test',
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
      userTypeName: 'Test',
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
      userTypeName: 'Test',
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
      userTypeName: 'Test',
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
      userTypeName: 'Test',
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

  it('adds a blank, fully-editable custom property when Add Custom Attribute is clicked', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [],
      onPropertiesChange: mockOnPropertiesChange,
      userTypeName: 'Test',
    };

    render(<EditSchemaSettings {...props} />);

    await user.click(screen.getByRole('button', {name: /add custom attribute/i}));

    expect(mockOnPropertiesChange).toHaveBeenCalledWith([
      expect.objectContaining({name: '', type: 'string', custom: true}),
    ]);
  });

  it('allows editing the type of a custom property', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [{...baseProperties[0], custom: true}],
      onPropertiesChange: mockOnPropertiesChange,
      userTypeName: 'Test',
    };

    render(<EditSchemaSettings {...props} />);

    await user.click(screen.getByText('email'));

    // The type select is editable for a custom property.
    expect(screen.getByRole('combobox')).not.toHaveAttribute('aria-disabled', 'true');
  });

  it('hides a picked attribute from the library once it is added', () => {
    const props = {
      properties: [{...baseProperties[0], name: 'phone'}],
      onPropertiesChange: mockOnPropertiesChange,
      userTypeName: 'Test',
    };

    render(<EditSchemaSettings {...props} />);

    expect(screen.queryByRole('button', {name: /phone number/i})).not.toBeInTheDocument();
    expect(screen.getByRole('button', {name: /username/i})).toBeInTheDocument();
  });

  it('allows editing the property name of a custom property', async () => {
    const user = userEvent.setup();
    const props = {
      properties: [{...baseProperties[0], custom: true}],
      onPropertiesChange: mockOnPropertiesChange,
      userTypeName: 'Test',
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
      userTypeName: 'Test',
    };

    render(<EditSchemaSettings {...props} />);

    const removeButtons = screen.getAllByRole('button', {name: /remove property/i});
    await user.click(removeButtons[0]);

    await waitFor(() => {
      expect(mockOnPropertiesChange).toHaveBeenCalledWith([baseProperties[1]]);
    });
  });
});
