// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, within} from '@thunderid/test-utils';
import {describe, expect, it, vi} from 'vitest';
import type {SchemaPropertyInput} from '../../../../types/user-types';
import EditScimMappingSettings from '../EditScimMappingSettings';

const properties: SchemaPropertyInput[] = [
  {
    id: '0',
    name: 'loginId',
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
    name: 'email',
    displayName: '',
    type: 'string',
    required: false,
    unique: false,
    credential: false,
    enum: [],
    regex: '',
  },
  {
    id: '2',
    name: 'address',
    displayName: '',
    type: 'object',
    required: false,
    unique: false,
    credential: false,
    enum: [],
    regex: '',
    properties: {},
  },
];

const twoEmailProperties: SchemaPropertyInput[] = [
  {
    id: '0',
    name: 'workEmail',
    displayName: '',
    type: 'string',
    required: false,
    unique: false,
    credential: false,
    enum: [],
    regex: '',
  },
  {
    id: '1',
    name: 'personalEmail',
    displayName: '',
    type: 'string',
    required: false,
    unique: false,
    credential: false,
    enum: [],
    regex: '',
  },
];

describe('EditScimMappingSettings', () => {
  it('renders one mapping row per scalar property, skipping object/array properties', () => {
    render(
      <EditScimMappingSettings
        properties={properties}
        mapping={{}}
        meta={{}}
        onMappingChange={vi.fn()}
        onMetaChange={vi.fn()}
        userTypeName="Employee"
      />,
    );

    expect(screen.getByText('loginId')).toBeInTheDocument();
    expect(screen.getByText('email')).toBeInTheDocument();
    expect(screen.queryByText('address')).not.toBeInTheDocument();
  });

  it('hides a single-valued target from other rows once it is chosen, but keeps it on its own row', () => {
    render(
      <EditScimMappingSettings
        properties={properties}
        mapping={{loginId: 'userName'}}
        meta={{}}
        onMappingChange={vi.fn()}
        onMetaChange={vi.fn()}
        userTypeName="Employee"
      />,
    );

    const loginRow = screen.getByTestId('scim-mapping-row-loginId');
    expect(within(loginRow).getByText('userName')).toBeInTheDocument();

    const emailRow = screen.getByTestId('scim-mapping-row-email');
    expect(within(emailRow).queryByText('userName')).not.toBeInTheDocument();
  });

  it('shows unmapped properties under the user type custom extension URN in the preview', () => {
    render(
      <EditScimMappingSettings
        properties={properties}
        mapping={{loginId: 'userName'}}
        meta={{}}
        onMappingChange={vi.fn()}
        onMetaChange={vi.fn()}
        userTypeName="Employee"
      />,
    );

    expect(screen.getByText(/urn:thunderid:params:scim:schemas:employee:2.0:User/)).toBeInTheDocument();
    expect(screen.getByText(/"email": "<email>"/)).toBeInTheDocument();
    expect(screen.getByText(/"address": "<address>"/)).toBeInTheDocument();
  });

  it('shows a warning when no property is mapped to userName', () => {
    render(
      <EditScimMappingSettings
        properties={properties}
        mapping={{}}
        meta={{}}
        onMappingChange={vi.fn()}
        onMetaChange={vi.fn()}
        userTypeName="Employee"
      />,
    );

    expect(screen.getByText(/No property is mapped to the SCIM userName attribute/i)).toBeInTheDocument();
  });

  it('does not show the userName warning once a property is mapped to it', () => {
    render(
      <EditScimMappingSettings
        properties={properties}
        mapping={{loginId: 'userName'}}
        meta={{}}
        onMappingChange={vi.fn()}
        onMetaChange={vi.fn()}
        userTypeName="Employee"
      />,
    );

    expect(screen.queryByText(/No property is mapped to the SCIM userName attribute/i)).not.toBeInTheDocument();
  });

  it('does not hide a multi-valued target from other rows, and allows both to be selected', () => {
    render(
      <EditScimMappingSettings
        properties={twoEmailProperties}
        mapping={{workEmail: 'emails', personalEmail: 'emails'}}
        meta={{}}
        onMappingChange={vi.fn()}
        onMetaChange={vi.fn()}
        userTypeName="Employee"
      />,
    );

    const workRow = screen.getByTestId('scim-mapping-row-workEmail');
    const personalRow = screen.getByTestId('scim-mapping-row-personalEmail');
    expect(within(workRow).getByText('emails')).toBeInTheDocument();
    expect(within(personalRow).getByText('emails')).toBeInTheDocument();
  });

  it('shows Type and Primary controls only for rows mapped to a multi-valued target', () => {
    render(
      <EditScimMappingSettings
        properties={twoEmailProperties}
        mapping={{workEmail: 'emails'}}
        meta={{}}
        onMappingChange={vi.fn()}
        onMetaChange={vi.fn()}
        userTypeName="Employee"
      />,
    );

    const workRow = screen.getByTestId('scim-mapping-row-workEmail');
    expect(within(workRow).getByText('Type (optional)')).toBeInTheDocument();
    expect(within(workRow).getByText('Primary')).toBeInTheDocument();

    const personalRow = screen.getByTestId('scim-mapping-row-personalEmail');
    expect(within(personalRow).queryByText('Primary')).not.toBeInTheDocument();
  });

  it('calls onMetaChange when the Primary checkbox is toggled', async () => {
    const {userEvent} = await import('@thunderid/test-utils');
    const onMetaChange = vi.fn();
    render(
      <EditScimMappingSettings
        properties={twoEmailProperties}
        mapping={{workEmail: 'emails', personalEmail: 'emails'}}
        meta={{}}
        onMappingChange={vi.fn()}
        onMetaChange={onMetaChange}
        userTypeName="Employee"
      />,
    );

    const workRow = screen.getByTestId('scim-mapping-row-workEmail');
    await userEvent.click(within(workRow).getByRole('checkbox'));

    expect(onMetaChange).toHaveBeenCalledWith({workEmail: {type: '', primary: true}});
  });

  it('moves Primary to the newly checked property and clears it from the other one', async () => {
    const {userEvent} = await import('@thunderid/test-utils');
    const onMetaChange = vi.fn();
    render(
      <EditScimMappingSettings
        properties={twoEmailProperties}
        mapping={{workEmail: 'emails', personalEmail: 'emails'}}
        meta={{workEmail: {type: 'work', primary: true}, personalEmail: {type: 'home', primary: false}}}
        onMappingChange={vi.fn()}
        onMetaChange={onMetaChange}
        userTypeName="Employee"
      />,
    );

    const personalRow = screen.getByTestId('scim-mapping-row-personalEmail');
    await userEvent.click(within(personalRow).getByRole('checkbox'));

    expect(onMetaChange).toHaveBeenCalledWith({
      workEmail: {type: 'work', primary: false},
      personalEmail: {type: 'home', primary: true},
    });
  });

  it('leaves the primary flag of another target alone when marking a property primary', async () => {
    const {userEvent} = await import('@thunderid/test-utils');
    const onMetaChange = vi.fn();
    const threeProperties: SchemaPropertyInput[] = [
      ...twoEmailProperties,
      {...twoEmailProperties[1], id: '2', name: 'mobile'},
    ];
    render(
      <EditScimMappingSettings
        properties={threeProperties}
        mapping={{workEmail: 'emails', personalEmail: 'phoneNumbers', mobile: 'phoneNumbers'}}
        meta={{workEmail: {type: '', primary: true}}}
        onMappingChange={vi.fn()}
        onMetaChange={onMetaChange}
        userTypeName="Employee"
      />,
    );

    const mobileRow = screen.getByTestId('scim-mapping-row-mobile');
    await userEvent.click(within(mobileRow).getByRole('checkbox'));

    expect(onMetaChange).toHaveBeenCalledWith({
      workEmail: {type: '', primary: true},
      mobile: {type: '', primary: true},
    });
  });

  it('clears the type and primary of a property when it moves to another target', async () => {
    const {userEvent} = await import('@thunderid/test-utils');
    const onMappingChange = vi.fn();
    const onMetaChange = vi.fn();
    render(
      <EditScimMappingSettings
        properties={twoEmailProperties}
        mapping={{workEmail: 'emails', personalEmail: 'emails'}}
        meta={{workEmail: {type: 'work', primary: true}, personalEmail: {type: 'home', primary: false}}}
        onMappingChange={onMappingChange}
        onMetaChange={onMetaChange}
        userTypeName="Employee"
      />,
    );

    const workRow = screen.getByTestId('scim-mapping-row-workEmail');
    await userEvent.click(within(workRow).getAllByRole('combobox')[0]);
    await userEvent.click(await screen.findByRole('option', {name: 'phoneNumbers'}));

    expect(onMappingChange).toHaveBeenCalledWith({workEmail: 'phoneNumbers', personalEmail: 'emails'});
    expect(onMetaChange).toHaveBeenCalledWith({personalEmail: {type: 'home', primary: false}});
  });

  it('locks the Primary checkbox on when only one property is mapped to the target', () => {
    render(
      <EditScimMappingSettings
        properties={twoEmailProperties}
        mapping={{workEmail: 'emails'}}
        meta={{}}
        onMappingChange={vi.fn()}
        onMetaChange={vi.fn()}
        userTypeName="Employee"
      />,
    );

    const checkbox = within(screen.getByTestId('scim-mapping-row-workEmail')).getByRole('checkbox');
    expect(checkbox).toBeChecked();
    expect(checkbox).toBeDisabled();
  });
});
