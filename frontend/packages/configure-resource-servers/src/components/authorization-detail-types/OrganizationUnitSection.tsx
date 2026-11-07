// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CopyableField, SettingsCard} from '@thunderid/components';
import {useGetOrganizationUnit} from '@thunderid/configure-organization-units';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

export interface OrganizationUnitSectionProps {
  ouId: string;
}

/**
 * The organization unit an authorization details type belongs to, with its handle and id each copyable. The owner
 * cannot change once the type is created, so it is only shown. The handle is left out until it is known.
 */
export default function OrganizationUnitSection({ouId}: OrganizationUnitSectionProps): JSX.Element {
  const {t} = useTranslation();
  const {data: organizationUnit} = useGetOrganizationUnit(ouId);

  return (
    <SettingsCard
      title={t('authorizationDetailTypes:organizationUnit.title', 'Organization unit')}
      description={t(
        'authorizationDetailTypes:organizationUnit.description',
        'The organization unit this intent belongs to.',
      )}
    >
      {organizationUnit?.handle && (
        <CopyableField
          label={t('authorizationDetailTypes:organizationUnit.handleLabel', 'Handle')}
          value={organizationUnit.handle}
          copyLabel={t('authorizationDetailTypes:organizationUnit.copyHandle', 'Copy handle')}
        />
      )}
      <CopyableField
        label={t('authorizationDetailTypes:organizationUnit.idLabel', 'ID')}
        value={ouId}
        copyLabel={t('authorizationDetailTypes:organizationUnit.copyId', 'Copy organization unit ID')}
      />
    </SettingsCard>
  );
}
