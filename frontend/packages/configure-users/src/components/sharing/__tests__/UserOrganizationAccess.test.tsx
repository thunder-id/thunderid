// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {OrganizationAccessSettings, type OrganizationAccessState} from '@thunderid/configure-resource-sharing';
import {render, screen} from '@thunderid/test-utils';
import type {User} from '@thunderid/types';
import {describe, it, expect, vi} from 'vitest';
import getUserErrorMessage from '../../../utils/getUserErrorMessage';
import ResharesList from '../ResharesList';
import UserOrganizationAccess from '../UserOrganizationAccess';

vi.mock('@thunderid/configure-resource-sharing', () => ({
  OrganizationAccessSettings: vi.fn(() => <div data-testid="organization-access-settings" />),
}));

vi.mock('../ResharesList', () => ({
  default: vi.fn(() => <div data-testid="reshares-list" />),
}));

const access = {hasChanges: false} as unknown as OrganizationAccessState;

describe('UserOrganizationAccess', () => {
  it('renders the shared settings for the user and its own unit, and its re-shares', () => {
    const user = {id: 'u-1', display: 'Alice', ouId: 'acme', type: 'person'} as User;

    render(<UserOrganizationAccess user={user} access={access} />);

    expect(screen.getByTestId('organization-access-settings')).toBeInTheDocument();
    expect(vi.mocked(OrganizationAccessSettings).mock.calls.at(-1)?.[0]).toEqual({
      access,
      ownerOuId: 'acme',
      resourceName: 'Alice',
      readOnly: false,
      errorNamespace: 'users',
      resolveErrorMessage: getUserErrorMessage,
      description: 'Alice can sign in to the applications of a granted organization unit.',
    });
    expect(vi.mocked(ResharesList).mock.calls.at(-1)?.[0]).toEqual({userName: 'Alice', resourcePath: 'users/u-1'});
  });

  it('names a user without a display name by its id', () => {
    const user = {id: 'u-1', ouId: 'acme', type: 'person'} as User;

    render(<UserOrganizationAccess user={user} access={access} />);

    expect(vi.mocked(OrganizationAccessSettings).mock.calls.at(-1)?.[0]).toEqual(
      expect.objectContaining({resourceName: 'u-1'}),
    );
  });

  it('passes a declared user on as read-only', () => {
    const user = {id: 'u-1', display: 'Alice', ouId: 'acme', type: 'person', isReadOnly: true} as User;

    render(<UserOrganizationAccess user={user} access={access} />);

    expect(vi.mocked(OrganizationAccessSettings).mock.calls.at(-1)?.[0]).toEqual(
      expect.objectContaining({readOnly: true}),
    );
  });
});
