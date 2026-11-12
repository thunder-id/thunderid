// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {InputAdornment, TextField} from '@wso2/oxygen-ui';
import {useEffect, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {LoginConsentConfig} from '../../../models/application';

const WHOLE_NUMBER = /^\d+$/;

/**
 * Returns the validity period a typed value stands for: 0 for an empty field, the number for a
 * whole number, or null for anything else.
 */
const parseValidityPeriod = (value: string): number | null => {
  const trimmed = value.trim();
  if (trimmed === '') return 0;
  return WHOLE_NUMBER.test(trimmed) ? Number(trimmed) : null;
};

const toFieldValue = (validityPeriod: number | undefined): string =>
  validityPeriod === undefined ? '' : String(validityPeriod);

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
  /**
   * Called whenever the field's validation state changes. `true` means the typed validity
   * period is not a whole number, so the parent can block Save.
   */
  onValidationChange?: (hasErrors: boolean) => void;
}

/**
 * Field for how long a user's consent to an application or agent lasts.
 *
 * The validity period is a whole number of seconds. An empty field or 0 means consent does not
 * expire, and both are stored as 0. A typed value that is not a whole number is held locally and
 * never committed.
 *
 * @param props - Component props
 * @returns The consent validity period field
 */
export default function LoginConsentField({
  loginConsent = undefined,
  onLoginConsentChange = undefined,
  disabled = false,
  onValidationChange = undefined,
}: LoginConsentFieldProps): JSX.Element {
  const {t} = useTranslation();
  const [fieldValue, setFieldValue] = useState<string>(toFieldValue(loginConsent?.validityPeriod));
  const [reconciledLoginConsent, setReconciledLoginConsent] = useState<LoginConsentConfig | undefined>(loginConsent);

  // Takes the stored value again when the configuration is replaced by anything other than this
  // field's own change (e.g. on discard), so a rejected or stale value does not linger. A cleared
  // field is stored as 0 and stays empty. Adjusted during render, per React's guidance for
  // resetting state when a prop changes.
  if (loginConsent !== reconciledLoginConsent) {
    setReconciledLoginConsent(loginConsent);
    if (parseValidityPeriod(fieldValue) !== (loginConsent?.validityPeriod ?? 0)) {
      setFieldValue(toFieldValue(loginConsent?.validityPeriod));
    }
  }

  const hasError = parseValidityPeriod(fieldValue) === null;

  useEffect(() => {
    onValidationChange?.(hasError);
  }, [hasError, onValidationChange]);

  // A typed value is a draft that goes with the field, so its error is cleared when it unmounts.
  useEffect(() => () => onValidationChange?.(false), [onValidationChange]);

  const isEditable = Boolean(onLoginConsentChange) && !disabled;

  const handleChange = (value: string): void => {
    setFieldValue(value);
    const validityPeriod = parseValidityPeriod(value);
    if (validityPeriod !== null) {
      onLoginConsentChange?.({...loginConsent, validityPeriod});
    }
  };

  return (
    <TextField
      labelPlacement="top"
      label={t('applications:edit.advanced.loginConsent.validityPeriod.label', 'Consent validity period')}
      id="login_consent_validity_period"
      type="number"
      size="small"
      fullWidth
      disabled={!isEditable}
      value={fieldValue}
      error={hasError}
      helperText={
        hasError
          ? t('applications:edit.advanced.loginConsent.validityPeriod.error', 'Enter a whole number of seconds.')
          : t(
              'applications:edit.advanced.loginConsent.validityPeriod.hint',
              'Leave empty or enter 0 so consent does not expire.',
            )
      }
      onChange={(e) => handleChange(e.target.value)}
      inputProps={{min: 0, step: 1}}
      InputProps={{
        endAdornment: (
          <InputAdornment position="end">
            {t('applications:edit.advanced.loginConsent.validityPeriod.unit', 'seconds')}
          </InputAdornment>
        ),
      }}
    />
  );
}
