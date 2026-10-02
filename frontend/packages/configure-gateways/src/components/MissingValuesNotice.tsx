// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Alert, AlertTitle, Box, Button, Chip, Stack, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {MissingValues} from '../models/gateway';

export interface MissingValuesNoticeProps {
  missing: MissingValues;
  /** Opens the gateway's variables. The action is left out when absent. */
  onManageVariables?: () => void;
  /** Opens the gateway's secrets. The action is left out when absent. */
  onManageSecrets?: () => void;
}

function NameChips({names, color}: {names: string[]; color: 'default' | 'error'}): JSX.Element {
  return (
    <Box>
      <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
        {names.map((name: string) => (
          <Chip key={name} size="small" color={color} label={name} sx={{fontFamily: 'monospace'}} />
        ))}
      </Stack>
    </Box>
  );
}

/**
 * Names the variables and secrets a version refers to that the gateway does not hold. A missing
 * secret is called out more strongly than a missing variable: the resources that use it would be
 * created with credentials that fail, where a missing variable leaves a field empty.
 */
export default function MissingValuesNotice({
  missing,
  onManageVariables = undefined,
  onManageSecrets = undefined,
}: MissingValuesNoticeProps): JSX.Element | null {
  const {t} = useTranslation();
  const variables = missing.variables ?? [];
  const secrets = missing.secrets ?? [];

  if (variables.length === 0 && secrets.length === 0) {
    return null;
  }

  return (
    <Stack spacing={2}>
      {secrets.length > 0 && (
        <Alert
          severity="error"
          action={
            onManageSecrets ? (
              <Button color="inherit" size="small" onClick={onManageSecrets}>
                {t('gateways:missing.secrets.manage', 'Manage secrets')}
              </Button>
            ) : undefined
          }
        >
          <AlertTitle>
            {t('gateways:missing.secrets.title', '{{count}} secret is not configured', {count: secrets.length})}
          </AlertTitle>
          <Typography variant="body2" sx={{mb: 1}}>
            {t(
              'gateways:missing.secrets.body',
              'The gateway does not hold these credentials. Applying now would create resources whose credentials fail. Set them on the gateway first.',
            )}
          </Typography>
          <NameChips names={secrets} color="error" />
        </Alert>
      )}
      {variables.length > 0 && (
        <Alert
          severity="warning"
          action={
            onManageVariables ? (
              <Button color="inherit" size="small" onClick={onManageVariables}>
                {t('gateways:missing.variables.manage', 'Manage variables')}
              </Button>
            ) : undefined
          }
        >
          <AlertTitle>
            {t('gateways:missing.variables.title', '{{count}} variable has no value', {count: variables.length})}
          </AlertTitle>
          <Typography variant="body2" sx={{mb: 1}}>
            {t(
              'gateways:missing.variables.body',
              'Applying now would leave these fields empty on the gateway. Set them on the gateway first.',
            )}
          </Typography>
          <NameChips names={variables} color="default" />
        </Alert>
      )}
    </Stack>
  );
}
