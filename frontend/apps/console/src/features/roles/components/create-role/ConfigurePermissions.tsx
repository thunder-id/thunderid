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

import {PermissionCatalog, SelectedScopesField, type ResourcePermissions} from '@thunderid/configure-resource-servers';
import {Box, Divider, FormLabel, Stack, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

interface ConfigurePermissionsProps {
  permissions: ResourcePermissions[];
  onPermissionsChange: (permissions: ResourcePermissions[]) => void;
}

export default function ConfigurePermissions({
  permissions,
  onPermissionsChange,
}: ConfigurePermissionsProps): JSX.Element {
  const {t} = useTranslation();

  return (
    <Stack spacing={3} sx={{width: '100%'}}>
      <Box>
        <Typography variant="h4" sx={{mb: 1}}>
          {t('roles:createWizard.permissions.title', 'Assign permissions (optional)')}
        </Typography>
        <Typography variant="body2" color="text.secondary">
          {t(
            'roles:createWizard.permissions.subtitle',
            'Choose what this role grants. You can skip this step and add permissions later.',
          )}
        </Typography>
      </Box>
      <PermissionCatalog selected={permissions} onChange={onPermissionsChange} />
      <Divider />
      <Box>
        <FormLabel sx={{display: 'block', mb: 1, fontWeight: 'medium'}}>
          {t('roles:createWizard.permissions.scopes.label', 'Selected scopes')}
        </FormLabel>
        <SelectedScopesField selected={permissions} />
      </Box>
    </Stack>
  );
}
