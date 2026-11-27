// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {PermissionCatalog, SelectedScopesField, type ResourcePermissions} from '@thunderid/configure-resource-servers';
import {Stack} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

interface EditPermissionsSettingsProps {
  /** Permissions currently staged for this role, grouped by resource server */
  permissions: ResourcePermissions[];
  /** Called whenever the user changes the permission selection */
  onPermissionsChange: (permissions: ResourcePermissions[]) => void;
  /** When true, all checkboxes are disabled */
  isReadOnly?: boolean;
  /** The role's organization unit: only the resource servers it owns or is shared are offered. */
  ouId?: string;
}

export default function EditPermissionsSettings({
  permissions,
  onPermissionsChange,
  isReadOnly = false,
  ouId = undefined,
}: EditPermissionsSettingsProps): JSX.Element {
  const {t} = useTranslation();

  return (
    <Stack spacing={3}>
      <SettingsCard title={t('roles:edit.permissions.title')} description={t('roles:edit.permissions.description')}>
        <PermissionCatalog selected={permissions} onChange={onPermissionsChange} readOnly={isReadOnly} ouId={ouId} />
      </SettingsCard>
      <SettingsCard
        title={t('roles:edit.permissions.scopes.title')}
        description={t('roles:edit.permissions.scopes.description')}
      >
        <SelectedScopesField selected={permissions} />
      </SettingsCard>
    </Stack>
  );
}
