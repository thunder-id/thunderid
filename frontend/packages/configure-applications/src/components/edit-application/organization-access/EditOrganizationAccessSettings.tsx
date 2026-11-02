// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {OrganizationAccessSettings, type OrganizationAccessState} from '@thunderid/configure-resource-sharing';
import type {JSX} from 'react';
import type {Application} from '../../../models/application';
import getApplicationErrorMessage from '../../../utils/getApplicationErrorMessage';

/**
 * Props for the {@link EditOrganizationAccessSettings} component.
 */
interface EditOrganizationAccessSettingsProps {
  /**
   * The application being edited
   */
  application: Application;
  /**
   * The application's Organization Access state, held by the edit page so it survives tab switches
   */
  access: OrganizationAccessState;
}

/**
 * Container component for the Organization Access tab of the application edit page: which
 * organization units, of the owner's tree and of other trees, can use the application. A granted unit
 * uses it to request tokens for itself, since signing a granted unit's users in isn't shared yet.
 *
 * @param props - Component props
 * @returns The Organization Access settings, or nothing for an application without an owning unit
 */
export default function EditOrganizationAccessSettings({
  application,
  access,
}: EditOrganizationAccessSettingsProps): JSX.Element | null {
  if (!application.ouId) {
    return null;
  }

  return (
    <OrganizationAccessSettings
      access={access}
      ownerOuId={application.ouId}
      resourceName={application.name}
      readOnly={application.isReadOnly === true}
      errorNamespace="applications"
      resolveErrorMessage={getApplicationErrorMessage}
    />
  );
}
