// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import type {ResourcePermissions} from '@thunderid/configure-resource-servers';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi, afterEach} from 'vitest';
import EditPermissionsSettings from '../EditPermissionsSettings';

vi.mock('@thunderid/configure-resource-servers', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/configure-resource-servers')>();
  return {
    ...actual,
    PermissionCatalog: ({
      selected,
      onChange,
      readOnly = false,
      ouId = undefined,
    }: {
      selected: ResourcePermissions[];
      onChange: (s: ResourcePermissions[]) => void;
      readOnly?: boolean;
      ouId?: string;
    }) => (
      <div data-testid="permission-catalog" data-readonly={readOnly} data-ou-id={ouId}>
        <span data-testid="catalog-selected">{JSON.stringify(selected)}</span>
        <button
          type="button"
          data-testid="catalog-change"
          onClick={() => onChange([{resourceServerId: 'rs-2', permissions: ['payments:refund']}])}
        >
          Change
        </button>
      </div>
    ),
    SelectedScopesField: ({selected}: {selected: ResourcePermissions[]}) => (
      <div data-testid="selected-scopes-field">
        <span data-testid="scopes-selected">{JSON.stringify(selected)}</span>
      </div>
    ),
  };
});

const permissions: ResourcePermissions[] = [{resourceServerId: 'rs-1', permissions: ['bookings', 'bookings:create']}];

describe('EditPermissionsSettings', () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it('renders the Permissions SettingsCard title and description', () => {
    render(<EditPermissionsSettings permissions={permissions} onPermissionsChange={vi.fn()} />);
    expect(screen.getByText('Permissions')).toBeInTheDocument();
    expect(screen.getByText('Select the permissions this role grants, grouped by resource server')).toBeInTheDocument();
  });

  it('renders the Selected scopes SettingsCard title and description', () => {
    render(<EditPermissionsSettings permissions={permissions} onPermissionsChange={vi.fn()} />);
    expect(screen.getByText('Selected scopes')).toBeInTheDocument();
    expect(
      screen.getByText('The OAuth scopes granted by these permissions. Copy them for use in your application.'),
    ).toBeInTheDocument();
  });

  it('passes permissions through to PermissionCatalog as selected', () => {
    render(<EditPermissionsSettings permissions={permissions} onPermissionsChange={vi.fn()} />);
    expect(screen.getByTestId('catalog-selected')).toHaveTextContent(JSON.stringify(permissions));
  });

  it('passes permissions through to SelectedScopesField as selected', () => {
    render(<EditPermissionsSettings permissions={permissions} onPermissionsChange={vi.fn()} />);
    expect(screen.getByTestId('scopes-selected')).toHaveTextContent(JSON.stringify(permissions));
  });

  it('calls onPermissionsChange when the catalog fires onChange', async () => {
    const user = userEvent.setup();
    const onPermissionsChange = vi.fn();
    render(<EditPermissionsSettings permissions={permissions} onPermissionsChange={onPermissionsChange} />);
    await user.click(screen.getByTestId('catalog-change'));
    expect(onPermissionsChange).toHaveBeenCalledWith([{resourceServerId: 'rs-2', permissions: ['payments:refund']}]);
  });

  it("offers the resource servers of the role's organization unit", () => {
    render(<EditPermissionsSettings permissions={[]} onPermissionsChange={vi.fn()} ouId="ou-1" />);

    expect(screen.getByTestId('permission-catalog')).toHaveAttribute('data-ou-id', 'ou-1');
  });

  it('passes readOnly to PermissionCatalog when isReadOnly is true', () => {
    render(<EditPermissionsSettings permissions={permissions} onPermissionsChange={vi.fn()} isReadOnly />);
    expect(screen.getByTestId('permission-catalog')).toHaveAttribute('data-readonly', 'true');
  });

  it('passes readOnly as false by default', () => {
    render(<EditPermissionsSettings permissions={permissions} onPermissionsChange={vi.fn()} />);
    expect(screen.getByTestId('permission-catalog')).toHaveAttribute('data-readonly', 'false');
  });
});
