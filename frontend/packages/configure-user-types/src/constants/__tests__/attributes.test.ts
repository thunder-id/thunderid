// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import ATTRIBUTES from '../attributes';

describe('ATTRIBUTES', () => {
  it('should have unique attribute names', () => {
    const names = ATTRIBUTES.map((attr) => attr.name);
    expect(new Set(names).size).toBe(names.length);
  });

  describe('mobile_number', () => {
    const mobileNumber = ATTRIBUTES.find((attr) => attr.name === 'mobile_number');

    it('should be a string attribute named Mobile Number', () => {
      expect(mobileNumber).toBeDefined();
      expect(mobileNumber?.type).toBe('string');
      expect(mobileNumber?.displayName).toBe('Mobile Number');
    });

    it.each(['+12345678920', '+94771234567'])('should accept E.164 number %s', (value) => {
      expect(new RegExp(mobileNumber?.regex ?? '').test(value)).toBe(true);
    });

    it.each(['0771234567', '+0123456789', '+1 234 567', '+1234567890123456', ''])(
      'should reject non E.164 value "%s"',
      (value) => {
        expect(new RegExp(mobileNumber?.regex ?? '').test(value)).toBe(false);
      },
    );
  });
});
