// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DurationField} from '@thunderid/components';
import {Box, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {LoginConsentConfig} from '../../../models/application';

/**
 * Props for the {@link LoginConsentField} component.
 */
interface LoginConsentFieldProps {
  /**
   * The current login consent configuration. A validity period of 0 or unset means consent does
   * not expire.
   */
  loginConsent?: LoginConsentConfig;
  /**
   * Callback invoked with the new configuration. Omit to render read-only.
   */
  onLoginConsentChange?: (loginConsent: LoginConsentConfig) => void;
  /**
   * Whether inputs should be disabled (e.g. read-only resource).
   */
  disabled?: boolean;
}

/**
 * Field for how long a user's consent to an application or agent lasts.
 *
 * The period is entered as a number and a unit and stored as whole seconds. "No expiry" is stored
 * as 0.
 *
 * @param props - Component props
 * @returns The consent validity period field
 */
export default function LoginConsentField({
  loginConsent = undefined,
  onLoginConsentChange = undefined,
  disabled = false,
}: LoginConsentFieldProps): JSX.Element {
  const {t} = useTranslation();
  const isEditable = Boolean(onLoginConsentChange) && !disabled;

  return (
    <Box>
      <DurationField
        label={t('applications:edit.advanced.loginConsent.validityPeriod.label', 'Consent validity period')}
        seconds={loginConsent?.validityPeriod ?? 0}
        zeroLabel={t('applications:edit.advanced.loginConsent.validityPeriod.noExpiry', 'No expiry')}
        testId="login-consent-validity-period"
        disabled={!isEditable}
        onChange={(validityPeriod) => onLoginConsentChange?.({...loginConsent, validityPeriod})}
      />
      <Typography variant="caption" color="text.secondary" sx={{display: 'block'}}>
        {t(
          'applications:edit.advanced.loginConsent.validityPeriod.hint',
          'How long consent is remembered before the user is asked again. A client that asks for consent every time ignores it. Choose No expiry for consent that does not expire.',
        )}
      </Typography>
    </Box>
  );
}
