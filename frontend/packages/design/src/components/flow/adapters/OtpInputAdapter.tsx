// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {cn} from '@thunderid/utils';
import {Box, FormControl, FormLabel, TextField, Typography} from '@wso2/oxygen-ui';
import {Fragment, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {FlowFieldProps} from '../../../models/flow';

const DEFAULT_OTP_LENGTH = 6;
const DEVICE_USER_CODE_LENGTH = 8;
const NUMERIC_OTP_CHARS = /^[0-9]*$/;
const ALPHANUMERIC_OTP_CHARS = /^[0-9A-Z]*$/;
const NON_NUMERIC_OTP_CHARS = /[^0-9]/g;
const NON_ALPHANUMERIC_OTP_CHARS = /[^0-9A-Z]/g;

export default function OtpInputAdapter({
  component,
  values,
  touched,
  fieldErrors,
  isLoading,
  resolve,
  onInputChange,
  onBlur,
  additionalData,
}: FlowFieldProps): JSX.Element | null {
  const {t} = useTranslation();
  const {ref} = component;

  if (!ref || typeof ref !== 'string') return null;

  const isDeviceUserCode = ref === 'user_code' || component.id === 'device-code-input';
  // Device Grant codes contain eight characters today. Use UI metadata when the server reports it.
  const reportedLength = Number(additionalData?.['otpLength']);
  const otpLength = isDeviceUserCode
    ? DEVICE_USER_CODE_LENGTH
    : Number.isInteger(reportedLength) && reportedLength > 0
      ? reportedLength
      : DEFAULT_OTP_LENGTH;

  // Device Grant user codes use uppercase letters; ordinary OTPs remain numeric unless configured otherwise.
  const numericOnly = !isDeviceUserCode && additionalData?.['otpNumericOnly'] !== 'false';
  const allowedPattern = numericOnly ? NUMERIC_OTP_CHARS : ALPHANUMERIC_OTP_CHARS;
  const disallowedPattern = numericOnly ? NON_NUMERIC_OTP_CHARS : NON_ALPHANUMERIC_OTP_CHARS;
  // Alphanumeric codes are minted from an uppercase charset and verified case-sensitively.
  const normalize = (input: string): string => (numericOnly ? input : input.toUpperCase());

  const hasError = !!(touched?.[ref] && fieldErrors?.[ref]);
  const inputValue = values[ref] ?? '';
  const otpValue = isDeviceUserCode ? inputValue.replace(/[-\s]/g, '').toUpperCase() : inputValue;
  const otpDigits = otpValue.padEnd(otpLength, ' ').split('').slice(0, otpLength);

  const focusDigit = (idx: number) => {
    const input = document.querySelector<HTMLInputElement>(`input[aria-label="OTP digit ${idx + 1}"]`);
    input?.focus();
  };

  return (
    <FormControl required={component.required} className={cn('Flow--otpInput', 'FormControl--root')}>
      <FormLabel htmlFor={ref} className={cn('Label--root')}>
        {t(resolve(component.label)!)}
      </FormLabel>
      <Box
        sx={{display: 'flex', gap: 1, justifyContent: 'center', mt: 1}}
        onBlur={(e) => {
          if (!e.currentTarget.contains(e.relatedTarget as Node)) {
            onBlur?.(ref);
          }
        }}
      >
        {otpDigits.map((digit, idx) => (
          // eslint-disable-next-line react/no-array-index-key
          <Fragment key={`${ref}-otp-${idx}`}>
            <TextField
              className={cn('TextField--root')}
              slotProps={{
                htmlInput: {
                  maxLength: 1,
                  inputMode: numericOnly ? 'numeric' : 'text',
                  style: {textAlign: 'center', fontSize: '1.5rem'},
                  'aria-label': `OTP digit ${idx + 1}`,
                },
              }}
              value={digit.trim()}
              onChange={(e) => {
                const value = normalize(e.target.value);
                if (!allowedPattern.test(value)) return;
                const newOtp = otpDigits.map((d, i) => (i === idx ? value : d));
                onInputChange(ref, newOtp.join(''));
                if (value && idx < otpLength - 1) focusDigit(idx + 1);
              }}
              onKeyDown={(e) => {
                if (e.key === 'Backspace' && !otpDigits[idx].trim() && idx > 0) focusDigit(idx - 1);
              }}
              onPaste={(e) => {
                e.preventDefault();
                const digits = normalize(e.clipboardData.getData('text/plain'))
                  .replace(disallowedPattern, '')
                  .slice(0, otpLength);
                onInputChange(ref, digits);
                focusDigit(Math.min(digits.length, otpLength - 1));
              }}
              error={hasError}
              disabled={isLoading}
              variant="outlined"
              sx={{width: 48, '& input': {padding: '12px 8px'}}}
            />
            {isDeviceUserCode && idx === 3 && (
              <Typography component="span" aria-hidden="true" sx={{alignSelf: 'center'}}>
                -
              </Typography>
            )}
          </Fragment>
        ))}
      </Box>
      {hasError && (
        <Typography variant="caption" color="error" sx={{mt: 0.5, ml: 1.75}}>
          {fieldErrors?.[ref]}
        </Typography>
      )}
    </FormControl>
  );
}
