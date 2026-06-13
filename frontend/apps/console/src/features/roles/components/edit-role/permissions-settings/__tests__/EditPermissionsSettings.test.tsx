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
    }: {
      selected: ResourcePermissions[];
      onChange: (s: ResourcePermissions[]) => void;
      readOnly?: boolean;
    }) => (
      <div data-testid="permission-catalog" data-readonly={readOnly}>
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

vi.mock('react-i18next', () => ({
  useTranslation: () => ({t: (key: string) => key}),
}));

const permissions: ResourcePermissions[] = [{resourceServerId: 'rs-1', permissions: ['bookings', 'bookings:create']}];

describe('EditPermissionsSettings', () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it('renders the Permissions SettingsCard title and description', () => {
    render(<EditPermissionsSettings permissions={permissions} onPermissionsChange={vi.fn()} />);
    expect(screen.getByText('roles:edit.permissions.title')).toBeInTheDocument();
    expect(screen.getByText('roles:edit.permissions.description')).toBeInTheDocument();
  });

  it('renders the Selected scopes SettingsCard title and description', () => {
    render(<EditPermissionsSettings permissions={permissions} onPermissionsChange={vi.fn()} />);
    expect(screen.getByText('roles:edit.permissions.scopes.title')).toBeInTheDocument();
    expect(screen.getByText('roles:edit.permissions.scopes.description')).toBeInTheDocument();
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

  it('passes readOnly to PermissionCatalog when isReadOnly is true', () => {
    render(<EditPermissionsSettings permissions={permissions} onPermissionsChange={vi.fn()} isReadOnly />);
    expect(screen.getByTestId('permission-catalog')).toHaveAttribute('data-readonly', 'true');
  });

  it('passes readOnly as false by default', () => {
    render(<EditPermissionsSettings permissions={permissions} onPermissionsChange={vi.fn()} />);
    expect(screen.getByTestId('permission-catalog')).toHaveAttribute('data-readonly', 'false');
  });
});
