// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {cn} from '@thunderid/utils';
import {Box, FormControl, FormLabel, TextField, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {FlowFieldProps} from '../../../models/flow';

const DEFAULT_OTP_LENGTH = 6;
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

  // The server reports the length of the code it generated, so the box count matches the OTP the
  // user received. Falls back to the common six digits when the step carries no length.
  const reportedLength = Number(additionalData?.['otpLength']);
  const otpLength = Number.isInteger(reportedLength) && reportedLength > 0 ? reportedLength : DEFAULT_OTP_LENGTH;

  // The server reports whether the code it generated is digits only. Anything else, including an
  // older server that omits the flag, keeps the numeric-only behaviour.
  const numericOnly = additionalData?.['otpNumericOnly'] !== 'false';
  const allowedPattern = numericOnly ? NUMERIC_OTP_CHARS : ALPHANUMERIC_OTP_CHARS;
  const disallowedPattern = numericOnly ? NON_NUMERIC_OTP_CHARS : NON_ALPHANUMERIC_OTP_CHARS;
  // Alphanumeric codes are minted from an uppercase charset and verified case-sensitively.
  const normalize = (input: string): string => (numericOnly ? input : input.toUpperCase());

  const hasError = !!(touched?.[ref] && fieldErrors?.[ref]);
  const otpValue = values[ref] ?? '';
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
          <TextField
            // eslint-disable-next-line react/no-array-index-key
            key={`${ref}-otp-${idx}`}
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
