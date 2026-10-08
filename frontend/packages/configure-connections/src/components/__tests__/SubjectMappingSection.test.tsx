// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen, waitFor, within} from '@thunderid/test-utils';
import {type JSX, useState} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import type {SubjectMappingValues} from '../../models/connection';
import SubjectMappingSection from '../SubjectMappingSection';

vi.mock('@thunderid/components', () => ({
  SettingsCard: ({children}: {children: React.ReactNode}) => <section>{children}</section>,
}));

let userTypes: {id: string; handle: string; displayName: string}[] = [];
let agentTypes: {id: string; handle: string; displayName: string}[] = [];
let subjectTypesLoading = false;
let subjectTypesError: Error | null = null;
const refetchSubjectTypes = vi.fn();

vi.mock('@thunderid/configure-user-types', () => ({
  useGetUserType: () => ({data: {schema: {}}}),
}));

vi.mock('@thunderid/configure-agent-types', () => ({
  useGetAgentType: () => ({data: {schema: {}}}),
}));

vi.mock('../../api/useSubjectMappingTypes', () => ({
  default: () => ({
    data: {
      userTypes: userTypes.map((type) => ({...type, category: 'user'})),
      agentTypes: agentTypes.map((type) => ({...type, category: 'agent'})),
    },
    isLoading: subjectTypesLoading,
    error: subjectTypesError,
    refetch: refetchSubjectTypes,
  }),
}));

function ControlledSubjectMappingSection({
  initialValues = {},
  onChange: onChangeSpy = () => undefined,
}: {
  initialValues?: SubjectMappingValues;
  onChange?: (
    field: keyof SubjectMappingValues,
    value: NonNullable<SubjectMappingValues[keyof SubjectMappingValues]>,
  ) => void;
}): JSX.Element {
  const [values, setValues] = useState<SubjectMappingValues>(initialValues);

  return (
    <SubjectMappingSection
      values={values}
      onChange={(field, value) => {
        onChangeSpy?.(field, value);
        setValues((previous) => ({...previous, [field]: value}));
      }}
    />
  );
}

describe('SubjectMappingSection', () => {
  beforeEach(() => {
    userTypes = [];
    agentTypes = [];
    subjectTypesLoading = false;
    subjectTypesError = null;
    vi.clearAllMocks();
  });

  it('shows a loader while subject types are loading', () => {
    subjectTypesLoading = true;

    render(<SubjectMappingSection values={{}} onChange={vi.fn()} />);

    expect(screen.getByTestId('subject-mapping-types-loading')).toHaveTextContent('Loading user and agent types...');
    expect(screen.queryByTestId('subject-mapping-category-user')).not.toBeInTheDocument();
    expect(screen.queryByTestId('subject-mapping-category-agent')).not.toBeInTheDocument();
  });

  it('shows an error and retries when loading subject types fails', () => {
    subjectTypesError = new Error('Failed to load subject types');

    render(<SubjectMappingSection values={{}} onChange={vi.fn()} />);

    expect(screen.getByTestId('subject-mapping-types-error')).toHaveTextContent('Failed to load user and agent types.');
    expect(screen.queryByTestId('subject-mapping-category-user')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', {name: 'Retry'}));
    expect(refetchSubjectTypes).toHaveBeenCalledTimes(1);
  });

  it.each([
    ['agent', 'user'],
    ['user', 'agent'],
  ] as const)('hides the %s mapping section when that category has no types', (hiddenCategory, visibleCategory) => {
    if (visibleCategory === 'user') {
      userTypes = [{id: 'employee-id', handle: 'employee', displayName: 'Employee'}];
    } else {
      agentTypes = [{id: 'assistant-id', handle: 'assistant', displayName: 'Assistant'}];
    }

    render(<SubjectMappingSection values={{}} onChange={vi.fn()} />);

    expect(screen.getByTestId(`subject-mapping-category-${visibleCategory}`)).toBeInTheDocument();
    expect(screen.queryByTestId(`subject-mapping-category-${hiddenCategory}`)).not.toBeInTheDocument();
  });

  it('selects the sole user and agent types without displaying entity-type fields', async () => {
    userTypes = [{id: 'employee-id', handle: 'employee', displayName: 'Employee'}];
    agentTypes = [{id: 'assistant-id', handle: 'assistant', displayName: 'Assistant'}];
    const onChange = vi.fn();

    render(<SubjectMappingSection values={{}} onChange={onChange} />);

    await waitFor(() => {
      expect(screen.queryByTestId(/subject-mapping-group-user-type-select-/)).not.toBeInTheDocument();
    });
    expect(screen.queryByDisplayValue('employee')).not.toBeInTheDocument();
    expect(screen.queryByDisplayValue('assistant')).not.toBeInTheDocument();
    expect(onChange).not.toHaveBeenCalled();

    fireEvent.change(screen.getAllByLabelText('ThunderID Attribute')[0], {target: {value: 'email'}});

    expect(onChange).toHaveBeenLastCalledWith('subjectAttributeMappings', [
      {
        entityType: 'employee',
        attributes: [{attribute: 'email'}],
      },
    ]);
  });

  it('hides the built-in default agent type while retaining it in the mapping payload', async () => {
    agentTypes = [{id: 'default-agent-id', handle: 'default', displayName: 'Default'}];
    const onChange = vi.fn();

    render(<SubjectMappingSection values={{}} onChange={onChange} />);

    await waitFor(() => {
      expect(screen.queryByDisplayValue('default')).not.toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText('ThunderID Attribute'), {target: {value: 'groups'}});

    expect(onChange).toHaveBeenLastCalledWith('subjectAttributeMappings', [
      {
        entityType: 'default',
        attributes: [{attribute: 'groups'}],
      },
    ]);
  });

  it('only sends explicitly entered values when an attribute edit is undone', () => {
    userTypes = [{id: 'employee-id', handle: 'employee', displayName: 'Employee'}];
    const onChange = vi.fn();

    render(
      <ControlledSubjectMappingSection
        initialValues={{
          subjectAttributeMappings: [{entityType: 'employee', attributes: [{attribute: 'email'}]}],
        }}
        onChange={onChange}
      />,
    );

    const attribute = screen.getByDisplayValue('email');
    fireEvent.change(attribute, {target: {value: 'department'}});
    fireEvent.change(attribute, {target: {value: 'email'}});

    expect(onChange).toHaveBeenLastCalledWith('subjectAttributeMappings', [
      {entityType: 'employee', attributes: [{attribute: 'email'}]},
    ]);
    expect(document.querySelector('input[id^="subject-mapping-pdp-attribute-"]')).not.toBeInTheDocument();
  });

  it('keeps the type dropdown when a category has multiple types', () => {
    userTypes = [
      {id: 'employee-id', handle: 'employee', displayName: 'Employee'},
      {id: 'customer-id', handle: 'customer', displayName: 'Customer'},
    ];

    render(<ControlledSubjectMappingSection />);

    expect(
      within(screen.getByTestId('subject-mapping-category-user')).getByTestId(
        /subject-mapping-group-user-type-select-/,
      ),
    ).toBeInTheDocument();
  });

  it('shows a legacy mapping when its entity type identifies one subject category', () => {
    userTypes = [{id: 'customer-id', handle: 'customer', displayName: 'Customer'}];

    render(
      <SubjectMappingSection
        values={{
          subjectAttributeMappings: [
            {
              entityType: 'customer',
              attributes: [{attribute: 'customerRef', pdpAttribute: 'customerRef'}],
            },
          ],
        }}
        onChange={vi.fn()}
      />,
    );

    expect(screen.getAllByDisplayValue('customerRef')).toHaveLength(1);
  });

  it('shows and preserves a mapping whose entity type cannot be resolved', () => {
    const onChange = vi.fn();

    render(
      <SubjectMappingSection
        values={{
          subjectAttributeMappings: [
            {
              entityType: 'deleted-type',
              attributes: [{attribute: 'email', pdpAttribute: 'mail'}],
            },
          ],
        }}
        onChange={onChange}
      />,
    );

    expect(screen.getByTestId('subject-mapping-unresolved')).toBeInTheDocument();
    expect(screen.getByText('Entity type: deleted-type')).toBeInTheDocument();
    expect(screen.getByText('Attributes: email → mail')).toBeInTheDocument();
    expect(onChange).not.toHaveBeenCalled();
  });

  it('keeps an unresolved mapping when another mapping is edited', () => {
    userTypes = [{id: 'employee-id', handle: 'employee', displayName: 'Employee'}];
    const onChange = vi.fn();

    render(
      <SubjectMappingSection
        values={{
          subjectAttributeMappings: [
            {entityType: 'employee', attributes: [{attribute: 'email'}]},
            {entityType: 'deleted-type', attributes: [{attribute: 'legacyId'}]},
          ],
        }}
        onChange={onChange}
      />,
    );

    fireEvent.change(screen.getByDisplayValue('email'), {target: {value: 'department'}});

    expect(onChange).toHaveBeenLastCalledWith('subjectAttributeMappings', [
      {entityType: 'employee', attributes: [{attribute: 'department'}]},
      {entityType: 'deleted-type', attributes: [{attribute: 'legacyId'}]},
    ]);
  });

  it('shows an ambiguous entity type as unresolved and lets the user remove it', () => {
    userTypes = [{id: 'shared-user-id', handle: 'shared', displayName: 'Shared'}];
    agentTypes = [{id: 'shared-agent-id', handle: 'shared', displayName: 'Shared'}];
    const onChange = vi.fn();

    render(
      <SubjectMappingSection
        values={{subjectAttributeMappings: [{entityType: 'shared', attributes: [{attribute: 'groups'}]}]}}
        onChange={onChange}
      />,
    );

    expect(screen.getByText('Entity type: shared')).toBeInTheDocument();
    fireEvent.click(screen.getByTestId(/subject-mapping-unresolved-remove-/));

    expect(onChange).toHaveBeenLastCalledWith('subjectAttributeMappings', []);
  });

  it('selects the final remaining user type without displaying an entity-type field', async () => {
    userTypes = [
      {id: 'employee-id', handle: 'employee', displayName: 'Employee'},
      {id: 'customer-id', handle: 'customer', displayName: 'Customer'},
      {id: 'partner-id', handle: 'partner', displayName: 'Partner'},
    ];

    render(<ControlledSubjectMappingSection />);

    const userMappings = screen.getByTestId('subject-mapping-category-user');
    const [firstSelect] = within(userMappings).getAllByTestId(/subject-mapping-group-user-type-select-/);
    fireEvent.mouseDown(firstSelect.querySelector('[role="combobox"]')!);
    fireEvent.click(screen.getByRole('option', {name: 'Employee'}));
    fireEvent.click(screen.getByTestId('subject-mapping-add-user'));
    const [, secondSelect] = within(userMappings).getAllByTestId(/subject-mapping-group-user-type-select-/);
    fireEvent.mouseDown(secondSelect.querySelector('[role="combobox"]')!);
    fireEvent.click(screen.getByRole('option', {name: 'Customer'}));
    fireEvent.click(screen.getByTestId('subject-mapping-add-user'));

    await waitFor(() => {
      expect(within(userMappings).queryAllByTestId(/subject-mapping-group-user-type-select-/)).toHaveLength(0);
      expect(screen.queryByDisplayValue('employee')).not.toBeInTheDocument();
      expect(screen.queryByDisplayValue('customer')).not.toBeInTheDocument();
      expect(screen.queryByDisplayValue('partner')).not.toBeInTheDocument();
    });
  });

  it('adds, updates, and removes attribute rows and subject mappings', async () => {
    userTypes = [
      {id: 'employee-id', handle: 'employee', displayName: 'Employee'},
      {id: 'customer-id', handle: 'customer', displayName: 'Customer'},
    ];

    render(<ControlledSubjectMappingSection />);

    const userMappings = screen.getByTestId('subject-mapping-category-user');
    const typeSelect = within(userMappings).getByTestId(/subject-mapping-group-user-type-select-/);
    fireEvent.mouseDown(typeSelect.querySelector('[role="combobox"]')!);
    fireEvent.click(screen.getByRole('option', {name: 'Employee'}));

    fireEvent.change(within(userMappings).getByLabelText('ThunderID Attribute'), {
      target: {value: 'email'},
    });
    fireEvent.click(within(userMappings).getByRole('button', {name: 'Add Mapping'}));

    const attributes = within(userMappings).getAllByLabelText('ThunderID Attribute');
    fireEvent.change(attributes[1], {target: {value: 'groups'}});
    fireEvent.click(within(userMappings).getAllByRole('button', {name: 'Map to a different PDP attribute'})[1]);
    fireEvent.change(within(userMappings).getByLabelText('PDP Attribute'), {target: {value: 'roles'}});

    fireEvent.change(attributes[1], {target: {value: 'email'}});
    expect(attributes[1]).toHaveValue('email');
    expect(screen.getAllByText('Each ThunderID attribute can be mapped only once for an entity type.')).toHaveLength(2);

    fireEvent.change(attributes[1], {target: {value: 'email_verified'}});
    expect(attributes[1]).toHaveValue('email_verified');
    expect(
      screen.queryByText('Each ThunderID attribute can be mapped only once for an entity type.'),
    ).not.toBeInTheDocument();

    fireEvent.click(within(userMappings).getByTestId(/subject-mapping-remove-\d+-2/));
    await waitFor(() => {
      expect(within(userMappings).getAllByLabelText('ThunderID Attribute')).toHaveLength(1);
    });

    fireEvent.click(screen.getByTestId('subject-mapping-add-user'));
    expect(within(userMappings).getAllByText('Remove')).toHaveLength(2);
    fireEvent.click(within(userMappings).getAllByText('Remove')[1]);

    await waitFor(() => {
      expect(within(userMappings).queryByDisplayValue('customer')).not.toBeInTheDocument();
    });
  });
});
