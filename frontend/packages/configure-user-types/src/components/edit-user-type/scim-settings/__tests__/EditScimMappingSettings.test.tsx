// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent, within} from '@thunderid/test-utils';
import type {ComponentProps} from 'react';
import {describe, expect, it, vi} from 'vitest';
import type {SchemaPropertyInput} from '../../../../types/user-types';
import EditScimMappingSettings from '../EditScimMappingSettings';

const property = (name: string, overrides: Partial<SchemaPropertyInput> = {}): SchemaPropertyInput => ({
  id: name,
  name,
  displayName: '',
  type: 'string',
  required: false,
  unique: false,
  credential: false,
  enum: [],
  regex: '',
  ...overrides,
});

const properties: SchemaPropertyInput[] = [
  property('loginId', {required: true, unique: true}),
  property('email'),
  property('address', {type: 'object', properties: {}}),
];

const twoEmailProperties: SchemaPropertyInput[] = [property('workEmail'), property('personalEmail')];

type Props = ComponentProps<typeof EditScimMappingSettings>;

const buildProps = (overrides: Partial<Props> = {}): Props => ({
  properties,
  mapping: {},
  meta: {},
  onMappingChange: vi.fn(),
  onMetaChange: vi.fn(),
  userTypeHandle: 'employee',
  scimCoreUserType: true,
  onRequestScimCoreChange: vi.fn(),
  ...overrides,
});

const renderSettings = (overrides: Partial<Props> = {}): Props => {
  const props = buildProps(overrides);
  render(<EditScimMappingSettings {...props} />);
  return props;
};

const comboboxes = (container: HTMLElement): HTMLElement[] => within(container).getAllByRole('combobox');

const openChipMenu = async (name: string): Promise<void> => {
  await userEvent.click(within(screen.getByTestId('scim-custom-attributes')).getByText(name));
};

describe('EditScimMappingSettings', () => {
  it('shows each mapped SCIM attribute once as a label, with its user type attribute dropdown', () => {
    renderSettings({mapping: {loginId: 'userName'}});

    const row = screen.getByTestId('scim-mapping-row-userName');
    expect(within(row).getByText('userName')).toBeInTheDocument();
    expect(comboboxes(row)).toHaveLength(1);
    expect(within(row).getByText('loginId')).toBeInTheDocument();
    expect(screen.queryByTestId('scim-mapping-row-emails')).not.toBeInTheDocument();
  });

  it('starts in SCIM order, then adds a new mapping at the end without re-sorting the rest', () => {
    const withPhone = [...properties, property('phone')];
    const rowIds = (): string[] =>
      screen.getAllByTestId(/^scim-mapping-row-/).map((row) => row.getAttribute('data-testid') ?? '');

    const {rerender} = render(
      <EditScimMappingSettings
        {...buildProps({properties: withPhone, mapping: {email: 'emails', loginId: 'userName'}})}
      />,
    );
    expect(rowIds()).toEqual(['scim-mapping-row-userName', 'scim-mapping-row-emails']);

    rerender(
      <EditScimMappingSettings
        {...buildProps({properties: withPhone, mapping: {email: 'emails', loginId: 'userName', phone: 'displayName'}})}
      />,
    );
    expect(rowIds()).toEqual(['scim-mapping-row-userName', 'scim-mapping-row-emails', 'scim-mapping-row-displayName']);

    rerender(
      <EditScimMappingSettings
        {...buildProps({properties: withPhone, mapping: {phone: 'displayName', loginId: 'userName', email: 'emails'}})}
      />,
    );
    expect(rowIds()).toEqual(['scim-mapping-row-userName', 'scim-mapping-row-emails', 'scim-mapping-row-displayName']);
  });

  it('always shows userName as a fixed row without a remove button and lets the user map a property to it', async () => {
    const {onMappingChange} = renderSettings();

    const row = screen.getByTestId('scim-mapping-row-userName');
    expect(within(row).getByText('userName')).toBeInTheDocument();
    expect(within(row).queryByRole('button', {name: 'Remove mapping'})).not.toBeInTheDocument();

    await userEvent.click(comboboxes(row)[0]);
    await userEvent.click(await screen.findByRole('option', {name: 'loginId'}));

    expect(onMappingChange).toHaveBeenCalledWith({loginId: 'userName'});
  });

  it('has no Add mapping button', () => {
    renderSettings({properties: twoEmailProperties, mapping: {workEmail: 'emails'}});

    expect(screen.queryByRole('button', {name: 'Add mapping'})).not.toBeInTheDocument();
  });

  it('adds another property to a multi-valued SCIM attribute from its Add another button', async () => {
    const {onMappingChange} = renderSettings({properties: twoEmailProperties, mapping: {workEmail: 'emails'}});

    const row = screen.getByTestId('scim-mapping-row-emails');
    await userEvent.click(within(row).getByRole('button', {name: 'Add another'}));
    expect(screen.queryByRole('menuitem', {name: 'workEmail'})).not.toBeInTheDocument();
    await userEvent.click(await screen.findByRole('menuitem', {name: 'personalEmail'}));

    expect(onMappingChange).toHaveBeenCalledWith({workEmail: 'emails', personalEmail: 'emails'});
    expect(screen.queryByRole('menu')).not.toBeInTheDocument();
  });

  it('offers Add another on every mapped multi-valued SCIM attribute but not on single-valued ones', () => {
    renderSettings({
      properties: [...twoEmailProperties, property('loginId'), property('mobile'), property('avatar')],
      mapping: {loginId: 'userName', workEmail: 'emails', mobile: 'phoneNumbers', avatar: 'photos'},
    });

    ['emails', 'phoneNumbers', 'photos'].forEach((target) => {
      expect(
        within(screen.getByTestId(`scim-mapping-row-${target}`)).getByRole('button', {name: 'Add another'}),
      ).toBeInTheDocument();
    });
    expect(
      within(screen.getByTestId('scim-mapping-row-userName')).queryByRole('button', {name: 'Add another'}),
    ).not.toBeInTheDocument();
  });

  it('gives a property mapped to a multi-valued SCIM attribute the first type by default', async () => {
    const {onMappingChange, onMetaChange} = renderSettings({mapping: {loginId: 'userName'}});

    await openChipMenu('email');
    await userEvent.click(await screen.findByRole('menuitem', {name: 'emails'}));

    expect(onMappingChange).toHaveBeenCalledWith({loginId: 'userName', email: 'emails'});
    expect(onMetaChange).toHaveBeenCalledWith({email: {type: 'work', primary: false}});
  });

  it('gives a property added to a multi-valued SCIM attribute the first type the others do not use', async () => {
    const {onMetaChange} = renderSettings({
      properties: twoEmailProperties,
      mapping: {workEmail: 'emails'},
      meta: {workEmail: {type: 'work', primary: true}},
    });

    await userEvent.click(
      within(screen.getByTestId('scim-mapping-row-emails')).getByRole('button', {name: 'Add another'}),
    );
    await userEvent.click(await screen.findByRole('menuitem', {name: 'personalEmail'}));

    expect(onMetaChange).toHaveBeenCalledWith({
      workEmail: {type: 'work', primary: true},
      personalEmail: {type: 'home', primary: false},
    });
  });

  it('does not offer a type another property of the same SCIM attribute already uses', async () => {
    renderSettings({
      properties: twoEmailProperties,
      mapping: {workEmail: 'emails', personalEmail: 'emails'},
      meta: {workEmail: {type: 'work', primary: true}, personalEmail: {type: 'home', primary: false}},
    });

    await userEvent.click(comboboxes(screen.getByTestId('scim-mapping-entry-personalEmail'))[1]);

    expect(await screen.findByRole('option', {name: 'other'})).toBeInTheDocument();
    expect(screen.getByRole('option', {name: 'home'})).toBeInTheDocument();
    expect(screen.queryByRole('option', {name: 'work'})).not.toBeInTheDocument();
  });

  it('moves the type to the replacement when the property of a multi-valued entry is swapped', async () => {
    const {onMappingChange, onMetaChange} = renderSettings({
      properties: [...twoEmailProperties, property('otherEmail')],
      mapping: {workEmail: 'emails'},
      meta: {workEmail: {type: 'home', primary: true}},
    });

    await userEvent.click(comboboxes(screen.getByTestId('scim-mapping-entry-workEmail'))[0]);
    await userEvent.click(await screen.findByRole('option', {name: 'otherEmail'}));

    expect(onMappingChange).toHaveBeenCalledWith({otherEmail: 'emails'});
    expect(onMetaChange).toHaveBeenCalledWith({otherEmail: {type: 'work', primary: false}});
  });

  it('hides Add another when no unmapped attribute is left', () => {
    renderSettings({properties: [property('workEmail')], mapping: {workEmail: 'emails'}});

    expect(screen.queryByRole('button', {name: 'Add another'})).not.toBeInTheDocument();
  });

  it('disables Add another when the settings are disabled', () => {
    renderSettings({properties: twoEmailProperties, mapping: {workEmail: 'emails'}, disabled: true});

    expect(screen.getByRole('button', {name: 'Add another'})).toBeDisabled();
  });

  it('removes a mapping and its type and primary metadata', async () => {
    const {onMappingChange, onMetaChange} = renderSettings({
      mapping: {loginId: 'userName', email: 'emails'},
      meta: {email: {type: 'work', primary: true}},
    });

    await userEvent.click(
      within(screen.getByTestId('scim-mapping-entry-email')).getByRole('button', {name: 'Remove mapping'}),
    );

    expect(onMappingChange).toHaveBeenCalledWith({loginId: 'userName'});
    expect(onMetaChange).toHaveBeenCalledWith({});
  });

  it('swaps the user type attribute of an existing row in place', async () => {
    const {onMappingChange} = renderSettings({
      properties: [...properties, property('phone')],
      mapping: {loginId: 'userName', phone: 'displayName'},
    });

    await userEvent.click(comboboxes(screen.getByTestId('scim-mapping-row-userName'))[0]);
    expect(screen.queryByRole('option', {name: 'phone'})).not.toBeInTheDocument();
    expect(screen.queryByRole('option', {name: 'address'})).not.toBeInTheDocument();
    await userEvent.click(await screen.findByRole('option', {name: 'email'}));

    expect(onMappingChange).toHaveBeenCalledWith({email: 'userName', phone: 'displayName'});
  });

  it('shows unmapped properties under the user type custom extension URN in the preview', () => {
    renderSettings({mapping: {loginId: 'userName'}});

    expect(screen.getAllByText(/urn:thunderid:params:scim:schemas:employee:2.0:User/).length).toBeGreaterThan(0);
    const preview = screen.getByTestId('scim-payload-preview');
    expect(preview).toHaveTextContent('"email": "<email>"');
    expect(preview).toHaveTextContent('"address": "<address>"');
  });

  it('lists unmapped attributes as plain name chips and drops them once mapped', () => {
    const {rerender} = render(<EditScimMappingSettings {...buildProps()} />);

    const custom = screen.getByTestId('scim-custom-attributes');
    expect(within(custom).getByText('loginId')).toBeInTheDocument();
    expect(within(custom).getByText('email')).toBeInTheDocument();
    expect(within(custom).getByText('address')).toBeInTheDocument();
    expect(within(custom).queryByText(/urn:thunderid/)).not.toBeInTheDocument();

    rerender(<EditScimMappingSettings {...buildProps({mapping: {loginId: 'userName'}})} />);

    const updated = screen.getByTestId('scim-custom-attributes');
    expect(within(updated).queryByText('loginId')).not.toBeInTheDocument();
    expect(within(updated).getByText('email')).toBeInTheDocument();
  });

  it('maps a custom attribute straight away when a SCIM attribute is picked from its chip menu', async () => {
    const {onMappingChange} = renderSettings({mapping: {loginId: 'userName'}});

    await openChipMenu('email');
    await userEvent.click(await screen.findByRole('menuitem', {name: 'displayName'}));

    expect(onMappingChange).toHaveBeenCalledWith({loginId: 'userName', email: 'displayName'});
    expect(screen.queryByRole('menu')).not.toBeInTheDocument();
  });

  it('offers userName in the chip menu only while it is unmapped', async () => {
    const {rerender} = render(<EditScimMappingSettings {...buildProps()} />);

    await openChipMenu('email');
    expect(await screen.findByRole('menuitem', {name: 'userName'})).toBeInTheDocument();
    await userEvent.keyboard('{Escape}');

    rerender(<EditScimMappingSettings {...buildProps({mapping: {loginId: 'userName'}})} />);

    await openChipMenu('email');
    expect(await screen.findByRole('menuitem', {name: 'displayName'})).toBeInTheDocument();
    expect(screen.queryByRole('menuitem', {name: 'userName'})).not.toBeInTheDocument();
  });

  it('does not categorize the SCIM attribute menu', async () => {
    renderSettings();

    await openChipMenu('email');

    expect(await screen.findByRole('menuitem', {name: 'displayName'})).toBeInTheDocument();
    expect(screen.queryByText('User Info')).not.toBeInTheDocument();
    expect(screen.queryByText('Enterprise')).not.toBeInTheDocument();
  });

  it('does not make attributes of an unmappable type selectable', async () => {
    renderSettings();

    await userEvent.click(within(screen.getByTestId('scim-custom-attributes')).getByText('address'));

    expect(screen.queryByRole('menu')).not.toBeInTheDocument();
  });

  it('does not open the chip menu when disabled', async () => {
    renderSettings({disabled: true});

    await userEvent.click(within(screen.getByTestId('scim-custom-attributes')).getByText('email'));

    expect(screen.queryByRole('menu')).not.toBeInTheDocument();
  });

  it('hides the custom attributes section once every attribute is mapped', () => {
    renderSettings({properties: [property('loginId')], mapping: {loginId: 'userName'}});

    expect(screen.queryByTestId('scim-custom-attributes')).not.toBeInTheDocument();
  });

  it('ignores a mapping of a property that is no longer mappable', () => {
    renderSettings({mapping: {address: 'userName'}});

    expect(screen.getByText(/No property is mapped to the SCIM userName attribute/i)).toBeInTheDocument();
    expect(screen.getByTestId('scim-payload-preview')).not.toHaveTextContent('"userName"');
    expect(within(screen.getByTestId('scim-custom-attributes')).getByText('address')).toBeInTheDocument();
  });

  it('shows a warning when no property is mapped to userName', () => {
    renderSettings();

    expect(screen.getByText(/No property is mapped to the SCIM userName attribute/i)).toBeInTheDocument();
  });

  it('does not show the userName warning once a property is mapped to it', () => {
    renderSettings({mapping: {loginId: 'userName'}});

    expect(screen.queryByText(/No property is mapped to the SCIM userName attribute/i)).not.toBeInTheDocument();
  });

  it('groups several properties mapped to a multi-valued SCIM attribute under one label', () => {
    renderSettings({
      properties: twoEmailProperties,
      mapping: {workEmail: 'emails', personalEmail: 'emails'},
    });

    expect(screen.getAllByText('emails')).toHaveLength(1);
    const row = screen.getByTestId('scim-mapping-row-emails');
    expect(within(row).getByTestId('scim-mapping-entry-workEmail')).toBeInTheDocument();
    expect(within(row).getByTestId('scim-mapping-entry-personalEmail')).toBeInTheDocument();
  });

  it('adds another property under the same multi-valued SCIM attribute from the chip menu', async () => {
    const {onMappingChange} = renderSettings({properties: twoEmailProperties, mapping: {workEmail: 'emails'}});

    await openChipMenu('personalEmail');
    await userEvent.click(await screen.findByRole('menuitem', {name: 'emails'}));

    expect(onMappingChange).toHaveBeenCalledWith({workEmail: 'emails', personalEmail: 'emails'});
  });

  it('offers a multi-valued SCIM attribute again once mapped, but not a mapped single-valued one', async () => {
    renderSettings({
      properties: [...twoEmailProperties, property('loginId')],
      mapping: {loginId: 'userName', workEmail: 'emails'},
    });

    await openChipMenu('personalEmail');

    expect(await screen.findByRole('menuitem', {name: 'emails'})).toBeInTheDocument();
    expect(screen.queryByRole('menuitem', {name: 'userName'})).not.toBeInTheDocument();
  });

  it('shows Type and Primary controls only for entries under a multi-valued SCIM attribute', () => {
    renderSettings({
      properties: [...twoEmailProperties, property('loginId')],
      mapping: {workEmail: 'emails', loginId: 'userName'},
    });

    const workEntry = screen.getByTestId('scim-mapping-entry-workEmail');
    expect(within(workEntry).getByText('Type')).toBeInTheDocument();
    expect(within(workEntry).getByText('Primary')).toBeInTheDocument();
    expect(within(screen.getByTestId('scim-mapping-entry-loginId')).queryByText('Primary')).not.toBeInTheDocument();
  });

  it('calls onMetaChange when the Primary checkbox is toggled', async () => {
    const {onMetaChange} = renderSettings({
      properties: twoEmailProperties,
      mapping: {workEmail: 'emails', personalEmail: 'emails'},
    });

    await userEvent.click(within(screen.getByTestId('scim-mapping-entry-workEmail')).getByRole('checkbox'));

    expect(onMetaChange).toHaveBeenCalledWith({workEmail: {type: '', primary: true}});
  });

  it('moves Primary to the newly checked property and clears it from the other one', async () => {
    const {onMetaChange} = renderSettings({
      properties: twoEmailProperties,
      mapping: {workEmail: 'emails', personalEmail: 'emails'},
      meta: {workEmail: {type: 'work', primary: true}, personalEmail: {type: 'home', primary: false}},
    });

    await userEvent.click(within(screen.getByTestId('scim-mapping-entry-personalEmail')).getByRole('checkbox'));

    expect(onMetaChange).toHaveBeenCalledWith({
      workEmail: {type: 'work', primary: false},
      personalEmail: {type: 'home', primary: true},
    });
  });

  it('leaves the primary flag of another target alone when marking a property primary', async () => {
    const {onMetaChange} = renderSettings({
      properties: [...twoEmailProperties, property('mobile')],
      mapping: {workEmail: 'emails', personalEmail: 'phoneNumbers', mobile: 'phoneNumbers'},
      meta: {workEmail: {type: '', primary: true}},
    });

    await userEvent.click(within(screen.getByTestId('scim-mapping-entry-mobile')).getByRole('checkbox'));

    expect(onMetaChange).toHaveBeenCalledWith({
      workEmail: {type: '', primary: true},
      mobile: {type: '', primary: true},
    });
  });

  it('clears the type and primary of a property when it is removed from the group', async () => {
    const {onMappingChange, onMetaChange} = renderSettings({
      properties: twoEmailProperties,
      mapping: {workEmail: 'emails', personalEmail: 'emails'},
      meta: {workEmail: {type: 'work', primary: true}, personalEmail: {type: 'home', primary: false}},
    });

    await userEvent.click(
      within(screen.getByTestId('scim-mapping-entry-workEmail')).getByRole('button', {name: 'Remove mapping'}),
    );

    expect(onMappingChange).toHaveBeenCalledWith({personalEmail: 'emails'});
    expect(onMetaChange).toHaveBeenCalledWith({personalEmail: {type: 'home', primary: false}});
  });

  it('locks the Primary checkbox on when only one property is mapped to the target', () => {
    renderSettings({
      properties: twoEmailProperties,
      mapping: {workEmail: 'emails'},
    });

    const checkbox = within(screen.getByTestId('scim-mapping-entry-workEmail')).getByRole('checkbox');
    expect(checkbox).toBeChecked();
    expect(checkbox).toBeDisabled();
  });

  it('hides the mapping table and shows the info box when not the SCIM core type', () => {
    renderSettings({scimCoreUserType: false});

    expect(screen.getByTestId('scim-core-info')).toBeInTheDocument();
    expect(screen.queryByText('No attributes are mapped yet.')).not.toBeInTheDocument();
    expect(screen.queryByText(/No property is mapped to the SCIM userName attribute/i)).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Add mapping'})).not.toBeInTheDocument();
  });

  it('names the current core user type in the info box', () => {
    renderSettings({scimCoreUserType: false, currentCoreUserTypeName: 'Contractor'});

    expect(screen.getByTestId('scim-core-info')).toHaveTextContent('Contractor is currently the SCIM core user type');
  });

  it('falls back to a plain prompt when no other core user type is known', () => {
    renderSettings({scimCoreUserType: false});

    expect(screen.getByTestId('scim-core-info')).toHaveTextContent('Set this user type as the SCIM core user type');
  });

  it('shows the payload preview but not the custom attributes when not the SCIM core type', () => {
    renderSettings({scimCoreUserType: false});

    expect(screen.getByText(/SCIM payload preview/i)).toBeInTheDocument();
    expect(screen.queryByTestId('scim-custom-attributes')).not.toBeInTheDocument();
  });

  it('allows defining a custom type from the Type dropdown', async () => {
    const {onMetaChange} = renderSettings({
      properties: twoEmailProperties,
      mapping: {workEmail: 'emails'},
    });

    const entry = screen.getByTestId('scim-mapping-entry-workEmail');
    // Second combobox in the entry is the Type dropdown (below the property dropdown)
    await userEvent.click(comboboxes(entry)[1]);
    await userEvent.click(await screen.findByRole('option', {name: /Custom type/i}));

    const input = within(entry).getByRole('textbox', {name: /Custom type/i});
    await userEvent.type(input, 'billing{Enter}');

    expect(onMetaChange).toHaveBeenCalledWith({
      workEmail: {type: 'billing', primary: false},
    });
  });

  it('cancels custom type editing on Escape without updating meta', async () => {
    const {onMetaChange} = renderSettings({
      properties: twoEmailProperties,
      mapping: {workEmail: 'emails'},
    });

    const entry = screen.getByTestId('scim-mapping-entry-workEmail');
    await userEvent.click(comboboxes(entry)[1]);
    await userEvent.click(await screen.findByRole('option', {name: /Custom type/i}));

    const input = within(entry).getByRole('textbox', {name: /Custom type/i});
    await userEvent.type(input, 'billing{Escape}');

    expect(onMetaChange).not.toHaveBeenCalled();
    expect(within(entry).queryByRole('textbox')).not.toBeInTheDocument();
  });

  it('lists the custom attributes when it is the SCIM core type', () => {
    renderSettings({mapping: {loginId: 'userName'}});

    expect(screen.getByTestId('scim-custom-attributes')).toBeInTheDocument();
  });

  it('requests setting from the info box button when not the SCIM core type', async () => {
    const {onRequestScimCoreChange} = renderSettings({scimCoreUserType: false});

    await userEvent.click(screen.getByRole('button', {name: 'Set as SCIM Core Type'}));

    expect(onRequestScimCoreChange).toHaveBeenCalledTimes(1);
  });

  it('has no set or remove button and shows the mapping when it is the SCIM core type', () => {
    renderSettings();

    expect(screen.queryByTestId('scim-core-info')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: /SCIM Core Type/i})).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Add mapping'})).not.toBeInTheDocument();
    expect(screen.getByTestId('scim-mapping-row-userName')).toBeInTheDocument();
  });

  it('explains the default for the only user type', () => {
    renderSettings({scimCoreLocked: true});

    expect(screen.queryByRole('button', {name: /SCIM Core Type/i})).not.toBeInTheDocument();
    expect(screen.getByText(/This is the only user type/i)).toBeInTheDocument();
  });
});
