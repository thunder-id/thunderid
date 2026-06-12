/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
}

export default function EditPermissionsSettings({
  permissions,
  onPermissionsChange,
  isReadOnly = false,
}: EditPermissionsSettingsProps): JSX.Element {
  const {t} = useTranslation();

  return (
    <Stack spacing={3}>
      <SettingsCard title={t('roles:edit.permissions.title')} description={t('roles:edit.permissions.description')}>
        <PermissionCatalog selected={permissions} onChange={onPermissionsChange} readOnly={isReadOnly} />
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
