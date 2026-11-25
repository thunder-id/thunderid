// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DurationField} from '@thunderid/components';
import {FormControl, FormLabel, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {LoginConsentConfig} from '../../models/application';

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
 * Field for how long a user's consent to an application or agent lasts. "No expiry" is stored as 0.
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

  const label = t('applications:edit.advanced.loginConsent.validityPeriod.label', 'Consent validity period');
  const fieldId = 'login-consent-validity-period';

  return (
    <FormControl fullWidth size="small">
      <FormLabel htmlFor={fieldId}>{label}</FormLabel>
      <DurationField
        inline
        id={fieldId}
        label={label}
        seconds={loginConsent?.validityPeriod ?? 0}
        zeroLabel={t('applications:edit.advanced.loginConsent.validityPeriod.noExpiry', 'No expiry')}
        testId={fieldId}
        disabled={!isEditable}
        onChange={(validityPeriod) => onLoginConsentChange?.({...loginConsent, validityPeriod})}
      />
      <Typography variant="caption" color="text.secondary" sx={{mt: 0.5}}>
        {t(
          'applications:edit.advanced.loginConsent.validityPeriod.hint',
          'How long consent is remembered before the user is asked again. Choose No expiry for consent that does not expire.',
        )}
      </Typography>
    </FormControl>
  );
}
