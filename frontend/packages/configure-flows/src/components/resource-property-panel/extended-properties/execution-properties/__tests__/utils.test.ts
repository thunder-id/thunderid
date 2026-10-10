// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import type {NotificationTemplateSummary} from '../../../../../models/notification-templates';
import {buildTemplateLabels, buildTemplateOptions, clampToInteger, parseCommaSeparated} from '../utils';

const templates: NotificationTemplateSummary[] = [
  {id: 'id-1', handle: 'user-invite', displayName: 'User Invite', self: '/self/id-1'},
  {id: 'id-2', handle: 'password-recovery', displayName: 'Password Recovery', self: '/self/id-2'},
];

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

describe('buildTemplateOptions', () => {
  it('should offer the returned template handles', () => {
    expect(buildTemplateOptions(templates, '')).toEqual(['user-invite', 'password-recovery']);
  });

  it('should return an empty list when no templates are loaded yet', () => {
    expect(buildTemplateOptions(undefined, '')).toEqual([]);
  });

  it('should not duplicate a current value that is already returned', () => {
    expect(buildTemplateOptions(templates, 'user-invite')).toHaveLength(2);
  });

  it('should keep an unknown current value so it is not blanked', () => {
    expect(buildTemplateOptions(templates, 'custom-handle')).toContain('custom-handle');
  });
});

describe('buildTemplateLabels', () => {
  it('should map each handle to its display name', () => {
    expect(buildTemplateLabels(templates)).toEqual({
      'user-invite': 'User Invite',
      'password-recovery': 'Password Recovery',
    });
  });

  it('should return an empty map when no templates are loaded yet', () => {
    expect(buildTemplateLabels(undefined)).toEqual({});
  });
});
