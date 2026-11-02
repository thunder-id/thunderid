// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CopyableField, SettingsCard} from '@thunderid/components';
import {useGetOrganizationUnit} from '@thunderid/configure-organization-units';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

interface OrganizationUnitSectionProps {
  ouId: string;
}

/**
 * The organization unit a connection belongs to, with its handle and id each copyable. The handle is left out until it
 * is known.
 */
export default function OrganizationUnitSection({ouId}: OrganizationUnitSectionProps): JSX.Element {
  const {t} = useTranslation('connections');
  const {data: organizationUnit} = useGetOrganizationUnit(ouId);

  return (
    <SettingsCard
      title={t('detail.organizationUnit.title', 'Organization unit')}
      description={t('detail.organizationUnit.description', 'The organization unit this connection belongs to.')}
    >
      {organizationUnit?.handle && (
        <CopyableField
          label={t('detail.organizationUnit.handleLabel', 'Handle')}
          value={organizationUnit.handle}
          copyLabel={t('detail.organizationUnit.copyHandle', 'Copy handle')}
        />
      )}
      <CopyableField
        label={t('detail.organizationUnit.idLabel', 'ID')}
        value={ouId}
        copyLabel={t('detail.organizationUnit.copyId', 'Copy organization unit ID')}
      />
    </SettingsCard>
  );
}
