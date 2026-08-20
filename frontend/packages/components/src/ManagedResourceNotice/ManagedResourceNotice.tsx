// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Alert, AlertTitle, Typography} from '@wso2/oxygen-ui';
import {type JSX} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * Explains why a resource cannot be edited here.
 *
 * Without this the read-only controls look like a permissions problem or a bug. The resource was
 * applied from the control plane, and a change made here would be replaced the next time the control
 * plane applied its configuration, so it has to be made there instead.
 */
export default function ManagedResourceNotice({sx = undefined}: {sx?: Record<string, unknown>}): JSX.Element {
  const {t} = useTranslation();

  return (
    <Alert severity="info" sx={{mb: 2, ...sx}}>
      <AlertTitle>{t('common:managedResource.title', 'Managed by the control plane')}</AlertTitle>
      <Typography variant="body2">
        {t(
          'common:managedResource.body',
          'This resource was applied from the control plane and is read only here. Change it there and apply again, otherwise the next apply would replace whatever was changed on this deployment.',
        )}
      </Typography>
    </Alert>
  );
}
