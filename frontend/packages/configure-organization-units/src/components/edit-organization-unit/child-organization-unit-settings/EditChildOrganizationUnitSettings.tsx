// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Stack} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import ManageChildOUsSection from './ManageChildOrganizationUnitSection';

/**
 * Props for the {@link EditChildOrganizationUnitSettings} component.
 */
interface EditChildOrganizationUnitSettingsProps {
  /**
   * The ID of the parent organization unit
   */
  organizationUnitId: string;
  /**
   * The name of the parent organization unit (for back navigation)
   */
  organizationUnitName: string;
}

/**
 * Child Organization Units tab content for the Organization Unit edit page.
 *
 * Displays sections for:
 * - Managing child organization units (lazy-loaded hierarchy)
 *
 * @param props - Component props
 * @returns Child OUs tab content
 */
export default function EditChildOrganizationUnitSettings({
  organizationUnitId,
  organizationUnitName,
}: EditChildOrganizationUnitSettingsProps): JSX.Element {
  return (
    <Stack spacing={3}>
      <ManageChildOUsSection organizationUnitId={organizationUnitId} organizationUnitName={organizationUnitName} />
    </Stack>
  );
}
