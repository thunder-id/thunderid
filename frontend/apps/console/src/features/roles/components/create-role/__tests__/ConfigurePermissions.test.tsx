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
    }: {
      selected: {resourceServerId: string; permissions: string[]}[];
      onChange: (s: {resourceServerId: string; permissions: string[]}[]) => void;
    }) => (
      <div data-testid="permission-catalog" data-selected={JSON.stringify(selected)}>
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

vi.mock('react-i18next', () => ({
  useTranslation: () => ({t: (key: string, fallback?: string) => fallback ?? key}),
}));

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
