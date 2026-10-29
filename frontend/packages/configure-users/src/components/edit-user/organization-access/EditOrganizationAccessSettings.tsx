// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {OrganizationAccessSettings, type OrganizationAccessState} from '@thunderid/configure-resource-sharing';
import type {User} from '@thunderid/types';
import type {JSX} from 'react';
import getUserErrorMessage from '../../../utils/getUserErrorMessage';

/**
 * Props for the {@link EditOrganizationAccessSettings} component.
 */
interface EditOrganizationAccessSettingsProps {
  /**
   * The user being edited
   */
  user: User;
  /**
   * The user's Organization Access state, held by the edit page so it survives tab switches. Its
   * wording namespace gives the screen the user's own sentences (`users:organizationAccess.*`).
   */
  access: OrganizationAccessState;
}

/**
 * Container component for the Affiliations tab of the user edit page: which organization units, of
 * the owner's tree and of other trees, the user is affiliated with, so it can sign in and act there.
 *
 * @param props - Component props
 * @returns The Organization Access settings for the user
 */
export default function EditOrganizationAccessSettings({
  user,
  access,
}: EditOrganizationAccessSettingsProps): JSX.Element {
  return (
    <OrganizationAccessSettings
      access={access}
      ownerOuId={user.ouId}
      resourceName={user.display ?? user.id}
      readOnly={user.isReadOnly === true}
      errorNamespace="users"
      resolveErrorMessage={getUserErrorMessage}
    />
  );
}
