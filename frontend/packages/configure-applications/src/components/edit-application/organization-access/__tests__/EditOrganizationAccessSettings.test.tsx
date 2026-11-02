// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {OrganizationAccessSettings, type OrganizationAccessState} from '@thunderid/configure-resource-sharing';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import type {Application} from '../../../../models/application';
import getApplicationErrorMessage from '../../../../utils/getApplicationErrorMessage';
import EditOrganizationAccessSettings from '../EditOrganizationAccessSettings';

vi.mock('@thunderid/configure-resource-sharing', () => ({
  OrganizationAccessSettings: vi.fn(() => <div data-testid="organization-access-settings" />),
}));

const access = {hasChanges: false} as unknown as OrganizationAccessState;

describe('EditOrganizationAccessSettings', () => {
  it('renders the shared settings for the application and its owning unit', () => {
    const application = {id: 'app-1', name: 'Storefront', ouId: 'acme'} as Application;

    render(<EditOrganizationAccessSettings application={application} access={access} />);

    expect(screen.getByTestId('organization-access-settings')).toBeInTheDocument();
    expect(vi.mocked(OrganizationAccessSettings).mock.calls.at(-1)?.[0]).toEqual({
      access,
      ownerOuId: 'acme',
      resourceName: 'Storefront',
      readOnly: false,
      errorNamespace: 'applications',
      resolveErrorMessage: getApplicationErrorMessage,
    });
  });

  it('passes a declared application on as read-only', () => {
    const application = {id: 'app-1', name: 'Storefront', ouId: 'acme', isReadOnly: true} as Application;

    render(<EditOrganizationAccessSettings application={application} access={access} />);

    expect(vi.mocked(OrganizationAccessSettings).mock.calls.at(-1)?.[0]).toEqual(
      expect.objectContaining({readOnly: true}),
    );
  });

  it('renders nothing for an application without an owning unit', () => {
    const application = {id: 'app-1', name: 'Storefront'} as Application;

    render(<EditOrganizationAccessSettings application={application} access={access} />);

    expect(screen.queryByTestId('organization-access-settings')).not.toBeInTheDocument();
  });
});
