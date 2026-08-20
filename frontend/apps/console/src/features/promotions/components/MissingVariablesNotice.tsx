// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Alert, AlertTitle, Box, Button, Chip, Stack, Typography} from '@wso2/oxygen-ui';
import {type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';

/**
 * Warns that placeholders would resolve to nothing on the next apply.
 *
 * This is worth calling out prominently because the import reports success either way: an unresolved
 * placeholder renders as empty, so the resource applies with the field stripped, which is how an
 * application ends up on a Data Plane with no redirect URIs and a broken login.
 */
export default function MissingVariablesNotice({
  envId = undefined,
  missing,
  missingSecrets = [],
}: {
  /** The environment the secrets belong to, so the notice can lead to where they are set. */
  envId?: string;
  missing: string[];
  missingSecrets?: string[];
}): JSX.Element | null {
  const {t} = useTranslation();
  const navigate = useNavigate();

  if (missing.length === 0 && missingSecrets.length === 0) {
    return null;
  }

  return (
    <>
      {missingSecrets.length > 0 && (
        <Alert
          severity="error"
          sx={{mb: 2}}
          action={
            envId ? (
              <Button
                size="small"
                onClick={() => {
                  void navigate(`/promotions/${envId}/secrets`);
                }}
              >
                {t('promotions:variables.manageSecrets', 'Manage secrets')}
              </Button>
            ) : undefined
          }
        >
          <AlertTitle>
            {t('promotions:variables.missingSecretsTitle', '{{count}} secret is not configured', {
              count: missingSecrets.length,
            })}
          </AlertTitle>
          <Typography variant="body2" sx={{mb: 1}}>
            {t(
              'promotions:variables.missingSecretsBody',
              "These credentials are not held by the Data Plane's secret service. Applying now creates resources whose credentials reject every attempt. Add them to the secret service first.",
            )}
          </Typography>
          <Box>
            <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
              {missingSecrets.map((name: string) => (
                <Chip key={name} size="small" color="error" label={name} />
              ))}
            </Stack>
          </Box>
        </Alert>
      )}
      {missing.length > 0 && (
        <MissingConfiguredVariables
          missing={missing}
          onManage={() => {
            void navigate('/environment-variables');
          }}
        />
      )}
    </>
  );
}

/** The non-secret half: values that are managed on the Control Plane. */
function MissingConfiguredVariables({missing, onManage}: {missing: string[]; onManage: () => void}): JSX.Element {
  const {t} = useTranslation();

  return (
    <Alert
      severity="warning"
      sx={{mb: 2}}
      action={
        <Button size="small" onClick={onManage}>
          {t('promotions:variables.manage', 'Manage variables')}
        </Button>
      }
    >
      <AlertTitle>
        {t('promotions:variables.missingTitle', '{{count}} variable has no value', {count: missing.length})}
      </AlertTitle>
      <Typography variant="body2" sx={{mb: 1}}>
        {t(
          'promotions:variables.missingBody',
          'Applying now leaves these fields empty on the Data Plane, which can break logins. Set them under Environment Variables first.',
        )}
      </Typography>
      <Box>
        <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
          {missing.map((name: string) => (
            <Chip key={name} size="small" label={name} />
          ))}
        </Stack>
      </Box>
    </Alert>
  );
}
