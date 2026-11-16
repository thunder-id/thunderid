// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Resource} from '../../../../../models/resources';
import ProvisioningProperties from '../ProvisioningProperties';

interface MockListQuery {
  data: {id: string; name: string; ouId: string; ouHandle?: string}[] | undefined;
  error: Error | null;
  isLoading: boolean;
  isSuccess: boolean;
  refetch: ReturnType<typeof vi.fn>;
}

const {mockGroupsQuery, mockRolesQuery} = vi.hoisted(() => ({
  mockGroupsQuery: {current: undefined as unknown as MockListQuery},
  mockRolesQuery: {current: undefined as unknown as MockListQuery},
}));

vi.mock('../../../../../api/useGetAllGroups', () => ({
  default: () => mockGroupsQuery.current,
}));

vi.mock('../../../../../api/useGetAllRoles', () => ({
  default: () => mockRolesQuery.current,
}));

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, _default?: unknown, options?: {id?: string}) => {
      const translations: Record<string, string> = {
        'flows:core.executions.provisioning.description': 'Configure just-in-time provisioning.',
        'flows:core.executions.federation.allowCrossOUProvisioning.label': 'Allow cross-OU provisioning',
        'flows:core.executions.federation.allowCrossOUProvisioning.hint': 'Provision users into another OU.',
        'flows:core.executions.provisioning.includeOptional.label': 'Include optional attributes',
        'flows:core.executions.provisioning.includeOptional.hint': 'Include optional attributes hint.',
        'flows:core.executions.provisioning.includeOptionalCredentials.label': 'Include optional credentials',
        'flows:core.executions.provisioning.includeOptionalCredentials.hint': 'Include optional credentials hint.',
        'flows:core.executions.provisioning.maxPerPrompt.label': 'Max attributes per prompt',
        'flows:core.executions.provisioning.maxPerPrompt.placeholder': '3',
        'flows:core.executions.provisioning.maxPerPrompt.hint': 'Maximum attributes shown per prompt.',
        'flows:core.executions.provisioning.unavailable': `Unavailable: ${options?.id}`,
        'flows:core.executions.provisioning.assignGroup.noneExist': 'No groups exist. Create one, then select it here.',
        'flows:core.executions.provisioning.assignRole.noneExist': 'No roles exist. Create one, then select it here.',
        'flows:core.executions.provisioning.assignGroup.label': 'Assign group',
        'flows:core.executions.provisioning.assignGroup.placeholder': 'Select groups to assign',
        'flows:core.executions.provisioning.assignGroup.noOptions': 'No groups found',
        'flows:core.executions.provisioning.assignGroup.loadError': 'Failed to load groups.',
        'flows:core.executions.provisioning.assignRole.label': 'Assign role',
        'flows:core.executions.provisioning.assignRole.placeholder': 'Select roles to assign',
        'flows:core.executions.provisioning.assignRole.loadError': 'Failed to load roles.',
        'common:actions.refresh': 'Refresh',
        'flows:core.executions.provisioning.delegated.label': 'Delegated mode',
        'flows:core.executions.provisioning.delegated.hint':
          'When enabled, agents created by this node act on behalf of a signed-in user. A value collected by the flow takes precedence.',
      };
      return translations[key] ?? key;
    },
  }),
}));

let mockHasFederatedExecutor = false;

vi.mock('@xyflow/react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@xyflow/react')>()),
  useStore: () => mockHasFederatedExecutor,
}));

describe('ProvisioningProperties', () => {
  const mockOnChange = vi.fn();

  const createResource = (properties: Record<string, unknown> = {}): Resource =>
    ({data: {properties}}) as unknown as Resource;

  beforeEach(() => {
    mockOnChange.mockClear();
    mockGroupsQuery.current = {
      data: [
        {id: 'group-1', name: 'Engineering', ouId: 'ou-1'},
        {id: 'group-2', name: 'Sales', ouId: 'ou-1'},
      ],
      error: null,
      isLoading: false,
      isSuccess: true,
      refetch: vi.fn(),
    };
    mockRolesQuery.current = {
      data: [{id: 'role-1', name: 'Administrator', ouId: 'ou-1'}],
      error: null,
      isLoading: false,
      isSuccess: true,
      refetch: vi.fn(),
    };
    mockHasFederatedExecutor = false;
  });

  it('should render the description and checkbox labels', () => {
    render(<ProvisioningProperties resource={createResource()} onChange={mockOnChange} />);

    expect(screen.getByText('Configure just-in-time provisioning.')).toBeInTheDocument();
    expect(screen.getByText('Allow cross-OU provisioning')).toBeInTheDocument();
  });

  it('should default maxPerPrompt to 0 when the node has no value configured', () => {
    render(<ProvisioningProperties resource={createResource()} onChange={mockOnChange} />);

    expect(screen.getByLabelText('Max attributes per prompt')).toHaveValue(0);
  });

  it('should coerce a string maxPerPrompt value to a number', () => {
    render(<ProvisioningProperties resource={createResource({maxPerPrompt: '5'})} onChange={mockOnChange} />);

    expect(screen.getByLabelText('Max attributes per prompt')).toHaveValue(5);
  });

  it('should render the configured numeric maxPerPrompt value as-is', () => {
    render(<ProvisioningProperties resource={createResource({maxPerPrompt: 7})} onChange={mockOnChange} />);

    expect(screen.getByLabelText('Max attributes per prompt')).toHaveValue(7);
  });

  it('should toggle the allowCrossOUProvisioning checkbox', async () => {
    const user = userEvent.setup();
    render(<ProvisioningProperties resource={createResource()} onChange={mockOnChange} />);

    await user.click(screen.getByLabelText('Allow cross-OU provisioning'));

    expect(mockOnChange).toHaveBeenCalledWith('data.properties.allowCrossOUProvisioning', true, expect.anything());
  });

  it('should offer the delegation setting, unchecked by default', () => {
    render(<ProvisioningProperties resource={createResource()} onChange={mockOnChange} />);

    expect(screen.getByLabelText('Delegated mode')).not.toBeChecked();
  });

  it('should keep delegation as a real boolean when it is switched on and back off', async () => {
    const user = userEvent.setup();
    const {rerender} = render(<ProvisioningProperties resource={createResource()} onChange={mockOnChange} />);

    await user.click(screen.getByLabelText('Delegated mode'));

    expect(mockOnChange).toHaveBeenLastCalledWith('data.properties.delegated', true, expect.anything());

    rerender(<ProvisioningProperties resource={createResource({delegated: true})} onChange={mockOnChange} />);
    expect(screen.getByLabelText('Delegated mode')).toBeChecked();

    await user.click(screen.getByLabelText('Delegated mode'));

    // Unchecking stores an explicit false rather than removing the property.
    expect(mockOnChange).toHaveBeenLastCalledWith('data.properties.delegated', false, expect.anything());
  });

  describe.each([
    {
      kind: 'group',
      plural: 'groups',
      property: 'assignGroup',
      label: 'Assign group',
      query: mockGroupsQuery,
      otherPlural: 'roles',
      items: [
        {id: 'group-1', name: 'Engineering', ouId: 'ou-1'},
        {id: 'group-2', name: 'Sales', ouId: 'ou-1'},
      ],
    },
    {
      kind: 'role',
      plural: 'roles',
      property: 'assignRole',
      label: 'Assign role',
      query: mockRolesQuery,
      otherPlural: 'groups',
      items: [
        {id: 'role-1', name: 'Administrator', ouId: 'ou-1'},
        {id: 'role-2', name: 'Auditor', ouId: 'ou-1'},
      ],
    },
  ])('$property', ({kind, plural, property, label, query, otherPlural, items}) => {
    const [first, second] = items;
    const path = `data.properties.${property}`;

    beforeEach(() => {
      query.current = {...query.current, data: items};
    });

    it(`should show the stored ${kind} IDs by name`, () => {
      render(
        <ProvisioningProperties
          resource={createResource({[property]: `${first.id}, ${second.id}`})}
          onChange={mockOnChange}
        />,
      );

      expect(screen.getByText(first.name)).toBeInTheDocument();
      expect(screen.getByText(second.name)).toBeInTheDocument();
    });

    it(`should store the ID of a ${kind} picked by name`, async () => {
      const user = userEvent.setup();
      render(<ProvisioningProperties resource={createResource({[property]: first.id})} onChange={mockOnChange} />);

      await user.click(screen.getByLabelText(label));
      await user.click(screen.getByRole('option', {name: second.name}));

      expect(mockOnChange).toHaveBeenCalledWith(path, `${first.id},${second.id}`, expect.anything());
    });

    it(`should filter the ${plural} by the name typed`, async () => {
      const user = userEvent.setup();
      render(<ProvisioningProperties resource={createResource()} onChange={mockOnChange} />);

      await user.type(screen.getByLabelText(label), first.name.slice(0, 3));

      expect(screen.getByRole('option', {name: first.name})).toBeInTheDocument();
      expect(screen.queryByRole('option', {name: second.name})).not.toBeInTheDocument();
    });

    it(`should store the remaining IDs when a ${kind} is removed`, async () => {
      const user = userEvent.setup();
      render(
        <ProvisioningProperties
          resource={createResource({[property]: `${first.id},${second.id}`})}
          onChange={mockOnChange}
        />,
      );

      await user.click(screen.getByLabelText(label));
      await user.keyboard('{Backspace}');

      expect(mockOnChange).toHaveBeenCalledWith(path, first.id, expect.anything());
    });

    it(`should keep a stored ID that matches no ${kind} when another is picked`, async () => {
      const user = userEvent.setup();
      render(<ProvisioningProperties resource={createResource({[property]: 'deleted'})} onChange={mockOnChange} />);

      expect(screen.getByText(/deleted/)).toBeInTheDocument();

      await user.click(screen.getByLabelText(label));
      await user.click(screen.getByRole('option', {name: first.name}));

      expect(mockOnChange).toHaveBeenCalledWith(path, `deleted,${first.id}`, expect.anything());
    });

    it(`should mark a stored ID that matches no ${kind} as unavailable once the ${plural} have loaded`, () => {
      render(
        <ProvisioningProperties
          resource={createResource({[property]: `deleted,${first.id}`})}
          onChange={mockOnChange}
        />,
      );

      expect(screen.getByText(/Unavailable: deleted/)).toBeInTheDocument();
      expect(screen.queryByText(new RegExp(`Unavailable: ${first.name}`))).not.toBeInTheDocument();
    });

    it(`should not mark a stored ID as unavailable while the ${plural} are loading`, () => {
      query.current = {...query.current, data: undefined, isLoading: true, isSuccess: false};
      render(<ProvisioningProperties resource={createResource({[property]: first.id})} onChange={mockOnChange} />);

      expect(screen.queryByText(/Unavailable/)).not.toBeInTheDocument();
    });

    it(`should tell ${plural} with the same name apart by their organization unit`, async () => {
      const user = userEvent.setup();
      query.current = {
        ...query.current,
        data: [
          {id: first.id, name: first.name, ouId: 'ou-1', ouHandle: 'emea'},
          {id: 'duplicate', name: first.name, ouId: 'ou-2', ouHandle: 'apac'},
          {id: second.id, name: second.name, ouId: 'ou-1', ouHandle: 'emea'},
        ],
      };
      render(<ProvisioningProperties resource={createResource({[property]: 'duplicate'})} onChange={mockOnChange} />);

      expect(screen.getByText(`${first.name} (apac)`)).toBeInTheDocument();

      await user.click(screen.getByLabelText(label));

      expect(screen.getByRole('option', {name: `${first.name} (emea)`})).toBeInTheDocument();
      expect(screen.getByRole('option', {name: second.name})).toBeInTheDocument();
    });

    it(`should leave a repeated ${kind} name without an organization unit handle as it is`, async () => {
      const user = userEvent.setup();
      query.current = {
        ...query.current,
        data: [
          {id: first.id, name: first.name, ouId: 'ou-1', ouHandle: 'emea'},
          {id: 'duplicate', name: first.name, ouId: 'ou-2'},
        ],
      };
      render(<ProvisioningProperties resource={createResource()} onChange={mockOnChange} />);

      await user.click(screen.getByLabelText(label));

      expect(screen.getByRole('option', {name: `${first.name} (emea)`})).toBeInTheDocument();
      expect(screen.getByRole('option', {name: first.name})).toBeInTheDocument();
    });

    it(`should hint to create a ${kind} first when none exist`, () => {
      query.current = {...query.current, data: []};
      render(<ProvisioningProperties resource={createResource()} onChange={mockOnChange} />);

      expect(screen.getByText(`No ${plural} exist. Create one, then select it here.`)).toBeInTheDocument();
      expect(screen.queryByText(`No ${otherPlural} exist. Create one, then select it here.`)).not.toBeInTheDocument();
    });

    it(`should not hint to create a ${kind} while the ${plural} are loading`, () => {
      query.current = {...query.current, data: undefined, isLoading: true, isSuccess: false};
      render(<ProvisioningProperties resource={createResource()} onChange={mockOnChange} />);

      expect(screen.queryByText(new RegExp(`No ${plural} exist`))).not.toBeInTheDocument();
    });

    it(`should offer a retry when the ${plural} fail to load`, async () => {
      const user = userEvent.setup();
      query.current = {...query.current, data: undefined, error: new Error('Network error'), isSuccess: false};
      render(<ProvisioningProperties resource={createResource()} onChange={mockOnChange} />);

      expect(screen.getByText(`Failed to load ${plural}.`)).toBeInTheDocument();

      await user.click(screen.getByRole('button', {name: 'Refresh'}));

      expect(query.current.refetch).toHaveBeenCalled();
    });
  });

  describe('provisioningOU', () => {
    it('should create in the resolved unit by default, in any flow', () => {
      render(<ProvisioningProperties resource={createResource()} onChange={mockOnChange} />);

      expect(screen.getByRole('combobox', {name: /provisioningOU\.label/})).toHaveTextContent(
        'flows:core.executions.provisioning.provisioningOU.resolved',
      );
    });

    it('should set the property when the user type unit is chosen', async () => {
      const user = userEvent.setup();
      render(<ProvisioningProperties resource={createResource()} onChange={mockOnChange} />);

      await user.click(screen.getByRole('combobox', {name: /provisioningOU\.label/}));
      await user.click(screen.getByText('flows:core.executions.provisioning.provisioningOU.userType'));

      expect(mockOnChange).toHaveBeenCalledWith(
        'data.properties.provisioningOU',
        'userType',
        expect.objectContaining({data: {properties: {}}}),
      );
    });

    it('should show the configured value', () => {
      render(
        <ProvisioningProperties resource={createResource({provisioningOU: 'userType'})} onChange={mockOnChange} />,
      );

      expect(screen.getByRole('combobox', {name: /provisioningOU\.label/})).toHaveTextContent(
        'flows:core.executions.provisioning.provisioningOU.userType',
      );
    });
  });

  describe('federated override warning', () => {
    const warning = 'flows:core.executions.provisioning.federatedOverrideWarning';

    it('should warn when a group is assigned in a flow with a federated sign-in', () => {
      mockHasFederatedExecutor = true;
      render(
        <ProvisioningProperties resource={createResource({assignGroup: 'engineering'})} onChange={mockOnChange} />,
      );

      expect(screen.getByText(warning)).toBeInTheDocument();
    });

    it('should not warn when nothing is assigned or the flow has no federated sign-in', () => {
      mockHasFederatedExecutor = true;
      const {unmount} = render(<ProvisioningProperties resource={createResource()} onChange={mockOnChange} />);
      expect(screen.queryByText(warning)).not.toBeInTheDocument();
      unmount();

      mockHasFederatedExecutor = false;
      render(<ProvisioningProperties resource={createResource({assignRole: 'admin'})} onChange={mockOnChange} />);
      expect(screen.queryByText(warning)).not.toBeInTheDocument();
    });
  });
});
