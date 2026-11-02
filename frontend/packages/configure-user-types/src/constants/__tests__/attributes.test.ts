// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import ATTRIBUTES from '../attributes';

describe('ATTRIBUTES', () => {
  it('should have unique attribute names', () => {
    const names = ATTRIBUTES.map((attr) => attr.name);
    expect(new Set(names).size).toBe(names.length);
  });

  it('should not offer verification flags', () => {
    const names = ATTRIBUTES.map((attr) => attr.name);
    expect(names).not.toContain('email_verified');
    expect(names).not.toContain('phone_verified');
  });

  describe('email', () => {
    const email = ATTRIBUTES.find((attr) => attr.name === 'email');

    it('should be an email attribute', () => {
      expect(email?.type).toBe('email');
    });

    it('should accept a valid email', () => {
      expect(new RegExp(email?.regex ?? '').test('User+tag@example.com')).toBe(true);
    });

    it('should reject a display name email', () => {
      expect(new RegExp(email?.regex ?? '').test('Name <user@example.com>')).toBe(false);
    });
  });

  describe('mobile_number', () => {
    const mobileNumber = ATTRIBUTES.find((attr) => attr.name === 'mobile_number');

    it('should be a mobile attribute named Mobile Number', () => {
      expect(mobileNumber).toBeDefined();
      expect(mobileNumber?.type).toBe('mobile');
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
