// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {OrganizationAccessSettings, type OrganizationAccessState} from '@thunderid/configure-resource-sharing';
import {useCapabilitiesContext} from '@thunderid/contexts';
import type {JSX} from 'react';
import type {Role} from '../../../models/role';

/**
 * Props for the {@link EditOrganizationAccessSettings} component.
 */
interface EditOrganizationAccessSettingsProps {
  /**
   * The role being edited
   */
  role: Role;
  /**
   * The role's Organization Access state, held by the edit page so it survives tab switches
   */
  access: OrganizationAccessState;
}

/**
 * Organization Access tab of the role edit page: which organization units, of the owner's tree and of
 * other trees, the role is available in. A policy can also turn assignments off where it reaches; the
 * tab only shows and keeps that rule, since the console doesn't set it.
 *
 * @param props - Component props
 * @returns The Organization Access settings for the role
 */
export default function EditOrganizationAccessSettings({
  role,
  access,
}: EditOrganizationAccessSettingsProps): JSX.Element {
  const {can} = useCapabilitiesContext();

  return (
    <OrganizationAccessSettings
      access={access}
      ownerOuId={role.ouId}
      resourceName={role.name}
      readOnly={role.isReadOnly === true || !can('role', 'update')}
      errorNamespace="roles"
    />
  );
}
