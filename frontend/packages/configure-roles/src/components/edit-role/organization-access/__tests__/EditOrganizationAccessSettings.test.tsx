// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {OrganizationAccessSettings, type OrganizationAccessState} from '@thunderid/configure-resource-sharing';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import type {Role} from '../../../../models/role';
import EditOrganizationAccessSettings from '../EditOrganizationAccessSettings';

vi.mock('@thunderid/configure-resource-sharing', () => ({
  OrganizationAccessSettings: vi.fn(() => <div data-testid="organization-access-settings" />),
}));

const access = {hasChanges: false} as unknown as OrganizationAccessState;

describe('EditOrganizationAccessSettings', () => {
  it('renders the shared settings for the role and its owning unit', () => {
    const role: Role = {id: 'role-1', name: 'Auditor', ouId: 'acme'};

    render(<EditOrganizationAccessSettings role={role} access={access} />);

    expect(screen.getByTestId('organization-access-settings')).toBeInTheDocument();
    expect(vi.mocked(OrganizationAccessSettings).mock.calls.at(-1)?.[0]).toEqual({
      access,
      ownerOuId: 'acme',
      resourceName: 'Auditor',
      readOnly: false,
      errorNamespace: 'roles',
    });
  });

  it('passes a declared role on as read-only', () => {
    const role: Role = {id: 'role-1', name: 'Auditor', ouId: 'acme', isReadOnly: true};

    render(<EditOrganizationAccessSettings role={role} access={access} />);

    expect(vi.mocked(OrganizationAccessSettings).mock.calls.at(-1)?.[0]).toEqual(
      expect.objectContaining({readOnly: true}),
    );
  });
});
