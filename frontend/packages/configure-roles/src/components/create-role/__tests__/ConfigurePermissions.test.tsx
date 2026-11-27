// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi, afterEach} from 'vitest';
import ConfigurePermissions from '../ConfigurePermissions';

vi.mock('@thunderid/configure-resource-servers', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/configure-resource-servers')>();
  return {
    ...actual,
    PermissionCatalog: ({
      selected,
      onChange,
      ouId = undefined,
    }: {
      selected: {resourceServerId: string; permissions: string[]}[];
      onChange: (s: {resourceServerId: string; permissions: string[]}[]) => void;
      ouId?: string;
    }) => (
      <div data-testid="permission-catalog" data-selected={JSON.stringify(selected)} data-ou-id={ouId}>
        <button
          type="button"
          data-testid="fire-change"
          onClick={() => onChange([{resourceServerId: 'rs-1', permissions: ['bookings']}])}
        >
          Fire Change
        </button>
      </div>
    ),
    SelectedScopesField: ({selected}: {selected: {resourceServerId: string; permissions: string[]}[]}) => (
      <div data-testid="selected-scopes-field" data-selected={JSON.stringify(selected)} />
    ),
  };
});

afterEach(() => {
  vi.clearAllMocks();
});

describe('ConfigurePermissions', () => {
  it('should render the heading and subtitle', () => {
    render(<ConfigurePermissions permissions={[]} onPermissionsChange={vi.fn()} />);

    expect(screen.getByText('Assign permissions (optional)')).toBeInTheDocument();
    expect(
      screen.getByText('Choose what this role grants. You can skip this step and add permissions later.'),
    ).toBeInTheDocument();
  });

  it("should offer the resource servers of the role's organization unit", () => {
    render(<ConfigurePermissions permissions={[]} onPermissionsChange={vi.fn()} ouId="ou-1" />);

    expect(screen.getByTestId('permission-catalog')).toHaveAttribute('data-ou-id', 'ou-1');
  });

  it('should render the PermissionCatalog stub', () => {
    render(<ConfigurePermissions permissions={[]} onPermissionsChange={vi.fn()} />);

    expect(screen.getByTestId('permission-catalog')).toBeInTheDocument();
  });

  it('should render the SelectedScopesField as a separate section', () => {
    render(<ConfigurePermissions permissions={[]} onPermissionsChange={vi.fn()} />);

    expect(screen.getByTestId('selected-scopes-field')).toBeInTheDocument();
  });

  it('should render the scopes section label', () => {
    render(<ConfigurePermissions permissions={[]} onPermissionsChange={vi.fn()} />);

    expect(screen.getByText('Selected scopes')).toBeInTheDocument();
  });

  it('should pass the permissions prop as selected to PermissionCatalog', () => {
    const permissions = [{resourceServerId: 'rs-1', permissions: ['bookings', 'bookings:create']}];
    render(<ConfigurePermissions permissions={permissions} onPermissionsChange={vi.fn()} />);

    const catalog = screen.getByTestId('permission-catalog');
    expect(JSON.parse(catalog.getAttribute('data-selected')!)).toEqual(permissions);
  });

  it('should pass the permissions prop as selected to SelectedScopesField', () => {
    const permissions = [{resourceServerId: 'rs-1', permissions: ['bookings', 'bookings:create']}];
    render(<ConfigurePermissions permissions={permissions} onPermissionsChange={vi.fn()} />);

    const scopesField = screen.getByTestId('selected-scopes-field');
    expect(JSON.parse(scopesField.getAttribute('data-selected')!)).toEqual(permissions);
  });

  it('should propagate PermissionCatalog onChange to onPermissionsChange', async () => {
    const onPermissionsChange = vi.fn();
    render(<ConfigurePermissions permissions={[]} onPermissionsChange={onPermissionsChange} />);

    await userEvent.click(screen.getByTestId('fire-change'));

    expect(onPermissionsChange).toHaveBeenCalledWith([{resourceServerId: 'rs-1', permissions: ['bookings']}]);
  });

  it('should not render chip delete buttons even with non-empty permissions', () => {
    render(
      <ConfigurePermissions
        permissions={[{resourceServerId: 'rs-1', permissions: ['bookings']}]}
        onPermissionsChange={vi.fn()}
      />,
    );

    expect(document.querySelectorAll('.MuiChip-deleteIcon')).toHaveLength(0);
  });
});
