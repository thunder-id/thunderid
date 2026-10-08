// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen, within} from '@testing-library/react';
import {useState} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import type {AttributeConfiguration} from '../../models/connection';
import AttributeMappingSection from '../AttributeMappingSection';

// Mirrors ConnectionDetailPage's real usage: an inline (identity-unstable) onChange handler, so a
// regression of the report-up effect depending on onChange's identity would loop here too.
function Harness({initialConfig = undefined}: {initialConfig?: AttributeConfiguration}): JSX.Element {
  const [, setConfig] = useState<AttributeConfiguration | undefined>(undefined);
  return <AttributeMappingSection initialConfig={initialConfig} onChange={(config) => setConfig(config)} />;
}

// The package's test setup renders real translations, so assert on the resolved English strings.
const RESOLUTION_TITLE = 'User type resolution';
const MAPPINGS_TITLE = 'Attribute Mappings';
const EXTERNAL_ATTRIBUTE = 'External Attribute';
const EXTERNAL_VALUE = 'External Value';
const LOCAL_ATTRIBUTE = 'Local Attribute';

// The SettingsCard is mocked to a labelled <section> so each card can be located and scoped by title.
vi.mock('@thunderid/components', () => ({
  SettingsCard: ({title, children}: {title: string; children: React.ReactNode}) => (
    <section aria-label={title}>{children}</section>
  ),
}));

let userTypes: {id: string; handle: string; displayName: string}[] = [
  {id: 'u1', handle: 'person', displayName: 'Person'},
];

vi.mock('@thunderid/configure-user-types', () => ({
  useGetUserTypes: () => ({data: {types: userTypes}}),
  useGetUserType: () => ({data: {schema: {firstName: {type: 'string'}}}}),
}));

describe('AttributeMappingSection', () => {
  const onChange = vi.fn();
  beforeEach(() => {
    vi.clearAllMocks();
    userTypes = [{id: 'u1', handle: 'person', displayName: 'Person'}];
  });

  it('renders both sections when more than one user type exists', () => {
    userTypes = [
      {id: 'u1', handle: 'person', displayName: 'Person'},
      {id: 'u2', handle: 'employee', displayName: 'Employee'},
    ];
    render(<AttributeMappingSection onChange={onChange} />);
    expect(screen.getByTestId('attribute-mapping-section')).toBeInTheDocument();
    expect(screen.getByLabelText(RESOLUTION_TITLE)).toBeInTheDocument();
    expect(screen.getByLabelText(MAPPINGS_TITLE)).toBeInTheDocument();
  });

  it('hides the whole user type resolution section when only one user type exists', () => {
    render(<AttributeMappingSection onChange={onChange} />);
    expect(screen.queryByLabelText(RESOLUTION_TITLE)).not.toBeInTheDocument();
    expect(screen.getByLabelText(MAPPINGS_TITLE)).toBeInTheDocument();
  });

  it('hides the per-group user type dropdown when only one user type exists', () => {
    render(<AttributeMappingSection onChange={onChange} />);
    expect(screen.queryByTestId(/attribute-mapping-group-user-type-select-/)).not.toBeInTheDocument();
  });

  it('emits no config on an empty single-user-type mount, so opening the tab does not dirty the form', () => {
    render(<AttributeMappingSection onChange={onChange} />);
    expect(onChange).toHaveBeenLastCalledWith(undefined, true);
  });

  it('round-trips an existing default-only config on a single-user-type system without dirtying it', () => {
    // Suppression of a default-only config must apply only to fresh connections; a previously saved
    // default-only config has to emit unchanged so opening the tab does not appear dirty.
    const initial: AttributeConfiguration = {userTypeResolution: {default: 'Person'}};
    render(<AttributeMappingSection initialConfig={initial} onChange={onChange} />);
    expect(onChange).toHaveBeenLastCalledWith(initial, true);
  });

  it('emits the existing config and a valid state when seeded (edit prefill)', () => {
    const initial: AttributeConfiguration = {
      userTypeResolution: {default: 'Person'},
      userTypeAttributeMappings: [
        {userType: 'Person', attributes: [{externalAttribute: 'given_name', localAttribute: 'firstName'}]},
      ],
    };
    render(<AttributeMappingSection initialConfig={initial} onChange={onChange} />);
    expect(onChange).toHaveBeenLastCalledWith(initial, true);
  });

  it('does not loop when an inline onChange stores an existing mapping config (regression)', () => {
    const initial: AttributeConfiguration = {
      userTypeResolution: {default: 'Person'},
      userTypeAttributeMappings: [
        {userType: 'Person', attributes: [{externalAttribute: 'given_name', localAttribute: 'firstName'}]},
      ],
    };
    expect(() => render(<Harness initialConfig={initial} />)).not.toThrow();
    expect(screen.getByTestId('attribute-mapping-section')).toBeInTheDocument();
  });

  it('pre-enables the value mapping toggle and shows existing entries on edit prefill', () => {
    userTypes = [
      {id: 'u1', handle: 'person', displayName: 'Person'},
      {id: 'u2', handle: 'employee', displayName: 'Employee'},
    ];
    const initial: AttributeConfiguration = {
      userTypeResolution: {default: 'Person', externalAttribute: 'user_type', valueMapping: {staff: 'Employee'}},
    };
    render(<AttributeMappingSection initialConfig={initial} onChange={onChange} />);
    const resolution = within(screen.getByLabelText(RESOLUTION_TITLE));
    expect(resolution.getByTestId('attribute-mapping-value-remove-1')).toBeInTheDocument();
    expect(onChange).toHaveBeenLastCalledWith(initial, true);
  });

  it('reports invalid when a mapping group has content but no user type', () => {
    userTypes = [
      {id: 'u1', handle: 'person', displayName: 'Person'},
      {id: 'u2', handle: 'employee', displayName: 'Employee'},
    ];
    render(<AttributeMappingSection onChange={onChange} />);
    // A second, unused user type exists, so "Add user type" is offered.
    fireEvent.click(screen.getByTestId('attribute-mapping-add-user-type'));
    // Fill an external attribute in the newly added (user-type-less) group's row.
    const mappings = within(screen.getByLabelText(MAPPINGS_TITLE));
    const externalInputs = mappings.getAllByLabelText(EXTERNAL_ATTRIBUTE);
    fireEvent.change(externalInputs[externalInputs.length - 1], {target: {value: 'given_name'}});
    expect(onChange).toHaveBeenLastCalledWith(undefined, false);
  });

  it('excludes a user type already picked by another mapping group from the options', () => {
    userTypes = [
      {id: 'u1', handle: 'person', displayName: 'Person'},
      {id: 'u2', handle: 'employee', displayName: 'Employee'},
    ];
    render(<AttributeMappingSection onChange={onChange} />);
    fireEvent.click(screen.getByTestId('attribute-mapping-add-user-type'));

    const [firstSelect, secondSelect] = screen.getAllByTestId(/attribute-mapping-group-user-type-select-/);
    fireEvent.mouseDown(firstSelect.querySelector('[role="combobox"]')!);
    fireEvent.click(screen.getByRole('option', {name: 'Person'}));

    fireEvent.mouseDown(secondSelect.querySelector('[role="combobox"]')!);
    expect(screen.queryByRole('option', {name: 'Person'})).not.toBeInTheDocument();
    expect(screen.getByRole('option', {name: 'Employee'})).toBeInTheDocument();
  });

  it('renders an expanded mapping group by default, before any user type is configured', () => {
    userTypes = [];
    render(<AttributeMappingSection onChange={onChange} />);
    const mappings = within(screen.getByLabelText(MAPPINGS_TITLE));
    expect(mappings.getByLabelText(EXTERNAL_ATTRIBUTE)).toBeInTheDocument();
    expect(mappings.getByPlaceholderText('e.g. firstName')).toBeInTheDocument();
  });

  it('does not offer "Add user type" once every user type is already used', () => {
    render(<AttributeMappingSection onChange={onChange} />);
    // Only one user type exists and it's already used by the auto-filled group.
    expect(screen.queryByTestId('attribute-mapping-add-user-type')).not.toBeInTheDocument();
  });

  it('reveals the external attribute field when the toggle is turned on, seeding a value-mapping row only once its own toggle is enabled', () => {
    userTypes = [
      {id: 'u1', handle: 'person', displayName: 'Person'},
      {id: 'u2', handle: 'employee', displayName: 'Employee'},
    ];
    const {container} = render(<AttributeMappingSection onChange={onChange} />);
    const resolution = within(screen.getByLabelText(RESOLUTION_TITLE));
    fireEvent.click(container.querySelector('.MuiSwitch-input')!);
    // Toggling dynamic on with no default and no external attribute is invalid.
    expect(onChange).toHaveBeenLastCalledWith(undefined, false);
    // Value mapping requires its own explicit toggle — typing the external attribute alone doesn't reveal it.
    expect(screen.queryByLabelText(EXTERNAL_VALUE)).not.toBeInTheDocument();

    fireEvent.change(resolution.getByLabelText(EXTERNAL_ATTRIBUTE), {target: {value: 'user_type'}});
    expect(screen.queryByLabelText(EXTERNAL_VALUE)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('switch', {name: 'Enable value mapping'}));
    // Enabling the toggle seeds a starter row immediately, mirroring the mapping-group/linking pattern.
    expect(screen.getByLabelText(EXTERNAL_VALUE)).toBeInTheDocument();
    // "Add value" is offered for further entries, disabled until the starter row is filled in.
    expect(screen.getByTestId('attribute-mapping-value-add')).toBeDisabled();
  });

  it('is valid with an external attribute and default set but no value mappings configured', () => {
    userTypes = [
      {id: 'u1', handle: 'person', displayName: 'Person'},
      {id: 'u2', handle: 'employee', displayName: 'Employee'},
    ];
    const {container} = render(<AttributeMappingSection onChange={onChange} />);
    const resolution = within(screen.getByLabelText(RESOLUTION_TITLE));
    fireEvent.click(container.querySelector('.MuiSwitch-input')!);
    fireEvent.change(resolution.getByLabelText(EXTERNAL_ATTRIBUTE), {target: {value: 'user_type'}});

    const defaultSelect = screen.getByTestId('attribute-mapping-default-user-type-select');
    fireEvent.mouseDown(defaultSelect.querySelector('[role="combobox"]')!);
    fireEvent.click(screen.getByRole('option', {name: 'Person'}));

    // Value Mapping toggle stays off — every identity resolves to Person until mappings are added.
    expect(onChange).toHaveBeenLastCalledWith(
      {userTypeResolution: {default: 'person', externalAttribute: 'user_type'}},
      true,
    );
  });

  it('hides dynamic resolution when only one user type exists', () => {
    const {container} = render(<AttributeMappingSection onChange={onChange} />);
    expect(container.querySelector('.MuiSwitch-input')).toBeNull();
  });

  it('hides delete for the default empty attribute-mapping row until it has content', () => {
    render(<AttributeMappingSection onChange={onChange} />);
    const mappings = within(screen.getByLabelText(MAPPINGS_TITLE));
    expect(mappings.queryByRole('button', {name: /remove attribute mapping/i})).not.toBeInTheDocument();

    fireEvent.change(mappings.getByLabelText(EXTERNAL_ATTRIBUTE), {target: {value: 'given_name'}});
    expect(mappings.getByRole('button', {name: /remove attribute mapping/i})).toBeInTheDocument();
  });

  it('shows delete for every row once an extra blank row is added', () => {
    render(<AttributeMappingSection onChange={onChange} />);
    const mappings = within(screen.getByLabelText(MAPPINGS_TITLE));
    // "Add mapping" is disabled until the current row has both sides filled in.
    fireEvent.change(mappings.getByLabelText(EXTERNAL_ATTRIBUTE), {target: {value: 'given_name'}});
    fireEvent.change(mappings.getByLabelText(LOCAL_ATTRIBUTE), {target: {value: 'firstName'}});
    fireEvent.click(screen.getByTestId(/^attribute-mapping-add-/));
    expect(mappings.getAllByRole('button', {name: /remove attribute mapping/i})).toHaveLength(2);
  });

  it('disables "Add mapping" until both sides of the current row are filled', () => {
    render(<AttributeMappingSection onChange={onChange} />);
    const mappings = within(screen.getByLabelText(MAPPINGS_TITLE));
    const addButton = screen.getByTestId(/^attribute-mapping-add-/);
    expect(addButton).toBeDisabled();

    fireEvent.change(mappings.getByLabelText(EXTERNAL_ATTRIBUTE), {target: {value: 'given_name'}});
    expect(addButton).toBeDisabled();

    fireEvent.change(mappings.getByLabelText(LOCAL_ATTRIBUTE), {target: {value: 'firstName'}});
    expect(addButton).not.toBeDisabled();
  });

  it('hides delete for an empty value-mapping row until it has content', () => {
    userTypes = [
      {id: 'u1', handle: 'person', displayName: 'Person'},
      {id: 'u2', handle: 'employee', displayName: 'Employee'},
    ];
    const {container} = render(<AttributeMappingSection onChange={onChange} />);
    const resolution = within(screen.getByLabelText(RESOLUTION_TITLE));
    fireEvent.click(container.querySelector('.MuiSwitch-input')!);
    fireEvent.change(resolution.getByLabelText(EXTERNAL_ATTRIBUTE), {target: {value: 'user_type'}});
    // Enabling seeds one empty starter row.
    fireEvent.click(screen.getByRole('switch', {name: 'Enable value mapping'}));

    expect(screen.queryByRole('button', {name: /remove value mapping/i})).not.toBeInTheDocument();

    fireEvent.change(screen.getByLabelText(EXTERNAL_VALUE), {target: {value: 'employee'}});
    expect(screen.getByRole('button', {name: /remove value mapping/i})).toBeInTheDocument();
  });
});
