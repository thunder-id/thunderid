// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import {clampToInteger, getTemplateScenarioLabel, getTemplateScenarioOptions, parseCommaSeparated} from '../utils';

describe('parseCommaSeparated', () => {
  it('should parse comma-separated values', () => {
    expect(parseCommaSeparated('a, b, c')).toEqual(['a', 'b', 'c']);
  });

  it('should trim whitespace from values', () => {
    expect(parseCommaSeparated('  foo ,  bar  , baz  ')).toEqual(['foo', 'bar', 'baz']);
  });

  it('should filter out empty strings', () => {
    expect(parseCommaSeparated('a,,b,,,c')).toEqual(['a', 'b', 'c']);
  });

  it('should return empty array for empty string', () => {
    expect(parseCommaSeparated('')).toEqual([]);
  });

  it('should return single item for no commas', () => {
    expect(parseCommaSeparated('single')).toEqual(['single']);
  });

  it('should handle trailing comma', () => {
    expect(parseCommaSeparated('a, b,')).toEqual(['a', 'b']);
  });

  it('should handle leading comma', () => {
    expect(parseCommaSeparated(',a, b')).toEqual(['a', 'b']);
  });

  it('should handle whitespace-only values as empty', () => {
    expect(parseCommaSeparated('a,   , b')).toEqual(['a', 'b']);
  });
});

describe('clampToInteger', () => {
  it('should pass through a value within bounds', () => {
    expect(clampToInteger('15', 1, 20)).toBe('15');
  });

  it('should clamp to the minimum', () => {
    expect(clampToInteger('-5', 0)).toBe('0');
  });

  it('should clamp to the maximum', () => {
    expect(clampToInteger('99', 1, 20)).toBe('20');
  });

  it('should floor decimals', () => {
    expect(clampToInteger('3.7', 0)).toBe('3');
  });

  it('should fall back to the minimum for an empty value', () => {
    expect(clampToInteger('', 4, 10)).toBe('4');
  });

  it('should fall back to the minimum for a whitespace-only value', () => {
    expect(clampToInteger('   ', 30)).toBe('30');
  });

  it('should reject a non-numeric value', () => {
    expect(clampToInteger('abc', 0)).toBeNull();
  });

  it('should leave the value unbounded above when no maximum is given', () => {
    expect(clampToInteger('9999', 1)).toBe('9999');
  });
});

describe('getTemplateScenarioOptions', () => {
  it('should offer the supported scenarios', () => {
    expect(getTemplateScenarioOptions('')).toEqual([
      'USER_INVITE',
      'MAGIC_LINK',
      'SELF_REGISTRATION',
      'OTP',
      'PASSWORD_RECOVERY',
      'CIBA_NOTIFICATION',
    ]);
  });

  it('should not duplicate a known current value', () => {
    expect(getTemplateScenarioOptions('OTP')).toHaveLength(6);
  });

  it('should keep an unknown current value so it is not blanked', () => {
    expect(getTemplateScenarioOptions('CUSTOM_SCENARIO')).toContain('CUSTOM_SCENARIO');
  });
});

describe('getTemplateScenarioLabel', () => {
  const translate = (key: string): string => `translated:${key}`;

  // Mirrors i18next resolving a missing key to the supplied default.
  const translateMissing = (_key: string, defaultValue: string): string => defaultValue;

  it('should translate a known scenario', () => {
    expect(getTemplateScenarioLabel('USER_INVITE', translate)).toBe(
      'translated:flows:core.executions.templateScenarios.userInvite',
    );
  });

  it('should fall back to the readable label when the key has no translation', () => {
    expect(getTemplateScenarioLabel('USER_INVITE', translateMissing)).toBe('User Invite');
    expect(getTemplateScenarioLabel('OTP', translateMissing)).toBe('OTP Verification');
    expect(getTemplateScenarioLabel('CIBA_NOTIFICATION', translateMissing)).toBe('CIBA Notification');
  });

  it('should fall back to the raw value for an unknown scenario', () => {
    expect(getTemplateScenarioLabel('CUSTOM_SCENARIO', translate)).toBe('CUSTOM_SCENARIO');
  });

  it('should fall back to an empty label for an empty value', () => {
    expect(getTemplateScenarioLabel('', translate)).toBe('');
  });
});
