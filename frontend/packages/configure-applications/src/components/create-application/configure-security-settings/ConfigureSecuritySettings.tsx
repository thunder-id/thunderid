// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Stack, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useEffect, useState} from 'react';
import {useTranslation} from 'react-i18next';
import useApplicationCreateContext from '../../../hooks/useApplicationCreateContext';
import {OrganizationUnitDefaultItem} from '../../../models/application-create-flow';
import ConfigureSignInOptions from '../configure-signin-options/ConfigureSignInOptions';

export interface ConfigureSecuritySettingsProps {
  /**
   * Callback function to broadcast whether this step is ready to proceed.
   */
  onReadyChange?: (isReady: boolean) => void;
}

/**
 * Second step of the application creation wizard: Sign In flow selection. Skipped entirely (see
 * `ApplicationCreatePage`'s `visibleSteps`) when the Details step chose to snapshot the
 * organization unit's sign-in default instead. Sign Up and Recovery are no longer configured
 * here either; they're inherited from the organization unit's defaults or left unset and
 * configured later from the application's edit page. Which user types can sign up through the
 * application is configured on the Access step alongside the sign-in experience.
 */
export default function ConfigureSecuritySettings({
  onReadyChange = undefined,
}: ConfigureSecuritySettingsProps): JSX.Element {
  const {t} = useTranslation();
  const {integrations, toggleIntegration, ouDefaults} = useApplicationCreateContext();

  const [signInReady, setSignInReady] = useState(false);

  const effectiveSignInReady = ouDefaults[OrganizationUnitDefaultItem.SIGN_IN] || signInReady;

  useEffect((): void => {
    onReadyChange?.(effectiveSignInReady);
  }, [effectiveSignInReady, onReadyChange]);

  return (
    <Stack direction="column" spacing={3} data-testid="application-configure-security-settings">
      <Stack direction="column" spacing={1}>
        <Typography variant="h1" gutterBottom>
          {t('applications:onboarding.configure.security.title', 'Sign-in Experience')}
        </Typography>
      </Stack>

      {!ouDefaults[OrganizationUnitDefaultItem.SIGN_IN] && (
        <ConfigureSignInOptions
          integrations={integrations}
          onIntegrationToggle={toggleIntegration}
          onReadyChange={setSignInReady}
          showTitle={false}
        />
      )}
    </Stack>
  );
}
