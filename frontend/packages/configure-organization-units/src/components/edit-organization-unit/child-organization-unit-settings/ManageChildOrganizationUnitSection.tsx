// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {useLogger} from '@thunderid/logger/react';
import {Box} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useOrganizationUnitRoutes from '../../../hooks/useOrganizationUnitRoutes';
import type {OUNavigationState} from '../../../models/navigation';
import OrganizationUnitTreePicker from '../../OrganizationUnitTreePicker';

/**
 * Props for the {@link ManageChildOrganizationUnitSection} component.
 */
interface ManageChildOrganizationUnitSectionProps {
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
 * Section component for managing child organization units.
 *
 * Displays a lazy-loaded hierarchy rooted below the current organization unit.
 * Clicking a row navigates to that organization unit's detail page.
 *
 * @param props - Component props
 * @returns Manage child OUs section within a SettingsCard
 */
export default function ManageChildOrganizationUnitSection({
  organizationUnitId,
  organizationUnitName,
}: ManageChildOrganizationUnitSectionProps): JSX.Element {
  const navigate = useNavigate();
  const routes = useOrganizationUnitRoutes();
  const {t} = useTranslation();
  const logger = useLogger('ManageChildOrganizationUnitSection');

  return (
    <SettingsCard
      title={t('organizationUnits:edit.childOUs.sections.manage.title', 'Child Organization Units')}
      description={t(
        'organizationUnits:edit.childOUs.sections.manage.description',
        'Organization units nested under this one.',
      )}
      slotProps={{content: {sx: {p: 0}}}}
    >
      <Box sx={{width: '100%', p: 2}}>
        <OrganizationUnitTreePicker
          rootOuId={organizationUnitId}
          hideRoot
          value=""
          onChange={() => undefined}
          maxHeight={400}
          onItemActivate={(organizationUnitIdToOpen) => {
            const navigationState: OUNavigationState = {
              fromOU: {
                id: organizationUnitId,
                name: organizationUnitName,
              },
            };
            (async (): Promise<void> => {
              await navigate(routes.detail(organizationUnitIdToOpen), {state: navigationState});
            })().catch((_error: unknown) => {
              logger.error('Failed to navigate to child organization unit', {
                error: _error,
                ouId: organizationUnitIdToOpen,
              });
            });
          }}
        />
      </Box>
    </SettingsCard>
  );
}
