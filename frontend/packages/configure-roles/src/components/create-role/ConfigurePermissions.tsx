// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {PermissionCatalog, SelectedScopesField, type ResourcePermissions} from '@thunderid/configure-resource-servers';
import {Box, Divider, FormLabel, Stack, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

interface ConfigurePermissionsProps {
  permissions: ResourcePermissions[];
  onPermissionsChange: (permissions: ResourcePermissions[]) => void;
  /** The role's organization unit: only the resource servers it owns or is shared are offered. */
  ouId?: string;
}

export default function ConfigurePermissions({
  permissions,
  onPermissionsChange,
  ouId = undefined,
}: ConfigurePermissionsProps): JSX.Element {
  const {t} = useTranslation();

  return (
    <Stack spacing={3} sx={{width: '100%'}}>
      <Box>
        <Typography variant="h4" sx={{mb: 1}}>
          {t('roles:createWizard.permissions.title')}
        </Typography>
        <Typography variant="body2" color="text.secondary">
          {t('roles:createWizard.permissions.subtitle')}
        </Typography>
      </Box>
      <PermissionCatalog selected={permissions} onChange={onPermissionsChange} ouId={ouId} />
      <Divider />
      <Box>
        <FormLabel sx={{display: 'block', mb: 1, fontWeight: 'medium'}}>
          {t('roles:createWizard.permissions.scopes.label')}
        </FormLabel>
        <SelectedScopesField selected={permissions} />
      </Box>
    </Stack>
  );
}
