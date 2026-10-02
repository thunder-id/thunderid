// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {apiError} from '../../__tests__/http';
import formatTimestamp from '../formatTimestamp';
import generateGatewayKey from '../generateGatewayKey';
import getApplyErrorMessage from '../getApplyErrorMessage';
import getGatewayValuesErrorMessage, {isGatewayUnreachable} from '../getGatewayValuesErrorMessage';
import {hasMissingValues, isMissingValuesError} from '../missingValues';
import {
  validateGatewayValueDescription,
  validateGatewayValueName,
  validateGatewayValueValue,
} from '../validateGatewayValue';

const t = (key: string, options?: Record<string, unknown>): string => {
  if (key === 'errors.GTW-5001') return 'The gateway could not apply the configuration.';
  if (key.includes('errors.')) return '';
  return `${key}|${options?.['defaultValue'] as string}`;
};

describe('formatTimestamp', () => {
  it('renders a dash when there is no timestamp', () => {
    expect(formatTimestamp(undefined)).toBe('-');
  });

  it('formats a valid timestamp for the locale', () => {
    expect(formatTimestamp('2026-01-02T03:04:05Z')).toBe(new Date('2026-01-02T03:04:05Z').toLocaleString());
  });

  it('keeps a value that is not a date as it is', () => {
    expect(formatTimestamp('not-a-date')).toBe('not-a-date');
  });
});

describe('generateGatewayKey', () => {
  it('generates distinct url-safe keys from 32 random bytes', () => {
    const first = generateGatewayKey();
    const second = generateGatewayKey();

    expect(first).toMatch(/^[A-Za-z0-9_-]{43}$/);
    expect(first).not.toBe(second);
  });
});

describe('getApplyErrorMessage', () => {
  it('says the gateway could not be reached for a bare 502', () => {
    expect(getApplyErrorMessage(apiError(502), t)).toMatch(/^apply\.unreachable\|The gateway could not be reached/);
  });

  it('resolves a coded 502 through the error catalog', () => {
    expect(getApplyErrorMessage(apiError(502, 'GTW-5001'), t)).toBe('The gateway could not apply the configuration.');
  });

  it('falls back to the generic message for any other failure', () => {
    expect(getApplyErrorMessage(new Error('boom'), t)).toBe(
      'apply.error|The configuration could not be applied. Please try again.',
    );
  });
});

describe('missing values', () => {
  it('finds missing values only when a name is listed', () => {
    expect(hasMissingValues(undefined)).toBe(false);
    expect(hasMissingValues({variables: [], secrets: []})).toBe(false);
    expect(hasMissingValues({secrets: ['S']})).toBe(true);
    expect(hasMissingValues({variables: ['A']})).toBe(true);
  });

  it('recognises an apply refused for missing values', () => {
    expect(isMissingValuesError(apiError(409, 'GTW-1017'))).toBe(true);
    expect(isMissingValuesError(apiError(409, 'GTW-1004'))).toBe(false);
    expect(isMissingValuesError(new Error('boom'))).toBe(false);
    expect(isMissingValuesError(null)).toBe(false);
  });
});

describe('getGatewayValuesErrorMessage', () => {
  it('says the gateway could not be reached for any 502', () => {
    expect(isGatewayUnreachable(apiError(502, 'GTW-5001'))).toBe(true);
    expect(getGatewayValuesErrorMessage(apiError(502, 'GTW-5001'), t, 'variables.error', 'x')).toMatch(
      /^values\.unreachable\|The gateway could not be reached/,
    );
  });

  it('resolves any other failure through the error catalog', () => {
    expect(getGatewayValuesErrorMessage(apiError(404), t, 'variables.error', 'Failed')).toBe('variables.error|Failed');
  });
});

describe('validateGatewayValue', () => {
  it('applies the name rules of the gateway store', () => {
    expect(validateGatewayValueName('')).toBe('required');
    expect(validateGatewayValueName('_OK_1')).toBeUndefined();
    expect(validateGatewayValueName('1BAD')).toBe('pattern');
    expect(validateGatewayValueName('has-dash')).toBe('pattern');
    expect(validateGatewayValueName('A'.repeat(256))).toBe('tooLong');
  });

  it('applies the value and description limits of the gateway store', () => {
    expect(validateGatewayValueValue('')).toBe('required');
    expect(validateGatewayValueValue('x'.repeat(8192))).toBeUndefined();
    expect(validateGatewayValueValue('x'.repeat(8193))).toBe('tooLong');
    expect(validateGatewayValueDescription('')).toBeUndefined();
    expect(validateGatewayValueDescription('x'.repeat(1001))).toBe('tooLong');
  });
});
