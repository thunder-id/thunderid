// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {Stack, Typography, CircularProgress, TextField} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link} from 'react-router';
import useGetOrganizationUnit from '../../../api/useGetOrganizationUnit';
import useOrganizationUnitRoutes from '../../../hooks/useOrganizationUnitRoutes';
import type {OUNavigationState} from '../../../models/navigation';
import type {OrganizationUnit} from '../../../models/organization-unit';

/**
 * Props for the {@link ParentSettingsSection} component.
 */
interface ParentSettingsSectionProps {
  /**
   * The organization unit being displayed
   */
  organizationUnit: OrganizationUnit;
}

/**
 * Section component displaying the parent organization unit information.
 *
 * Shows:
 * - A link to the parent OU if one exists and is loaded
 * - A loading spinner while fetching parent OU details
 * - A "no parent" message if the OU has no parent
 * - The raw parent ID if the parent OU details cannot be resolved
 *
 * @param props - Component props
 * @returns Parent organization unit info within a SettingsCard
 */
export default function ParentSettingsSection({organizationUnit}: ParentSettingsSectionProps): JSX.Element {
  const {t} = useTranslation();
  const routes = useOrganizationUnitRoutes();

  const {data: parentOU, isLoading: isLoadingParent} = useGetOrganizationUnit(
    organizationUnit.parent ?? undefined,
    Boolean(organizationUnit.parent),
  );

  const renderParentInfo = (): JSX.Element => {
    if (!organizationUnit.parent) {
      return (
        <Typography variant="body2" color="text.secondary">
          {t('organizationUnits:edit.general.ou.noParent.label', 'This is a root organization unit.')}
        </Typography>
      );
    }

    if (isLoadingParent) {
      return <CircularProgress size={16} />;
    }

    if (parentOU) {
      const navigationState: OUNavigationState = {
        fromOU: {
          id: organizationUnit.id,
          name: organizationUnit.name,
        },
      };

      return (
        <Stack direction="row" spacing={1} alignItems="center">
          <Typography
            component={Link}
            to={routes.detail(parentOU.id)}
            state={navigationState}
            data-state={JSON.stringify(navigationState)}
            variant="body2"
            sx={{
              color: 'primary.main',
              textDecoration: 'none',
              '&:hover': {
                textDecoration: 'underline',
              },
            }}
          >
            {parentOU.name}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            ({parentOU.id})
          </Typography>
        </Stack>
      );
    }

    return (
      <TextField
        fullWidth
        id="parent-ou-input"
        value={organizationUnit.parent}
        inputProps={{
          'aria-label': t('organizationUnits:edit.general.ou.parent.label', 'Parent organization unit'),
        }}
        InputProps={{
          readOnly: true,
        }}
        sx={{
          '& input': {
            fontFamily: 'monospace',
            fontSize: '0.875rem',
          },
        }}
      />
    );
  };

  return (
    <SettingsCard
      title={t('organizationUnits:edit.general.sections.parentOUSettings.title', 'Parent organization unit')}
      description={t(
        'organizationUnits:edit.general.sections.parentOUSettings.description',
        'The parent organization unit in the hierarchy.',
      )}
    >
      {renderParentInfo()}
    </SettingsCard>
  );
}
