// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import getInitials from '../getInitials';

describe('getInitials', () => {
  it('should return "?" for an empty or missing name', () => {
    expect(getInitials()).toBe('?');
    expect(getInitials('   ')).toBe('?');
  });

  it('should return the first letter of the first two words', () => {
    expect(getInitials('john doe')).toBe('JD');
  });

  it('should return the first two letters of a single word', () => {
    expect(getInitials('admin')).toBe('AD');
  });

  it('should keep a leading emoji whole instead of splitting its surrogate pair', () => {
    expect(getInitials('😈 Bob')).toBe('😈B');
  });

  it('should keep a trailing emoji whole in a multi-word name', () => {
    expect(getInitials('Bob 😈')).toBe('B😈');
  });

  it('should keep an emoji whole in a single-word name', () => {
    expect(getInitials('a😈')).toBe('A😈');
  });

  it('should return only well-formed strings', () => {
    for (const name of ['😈 Bob', 'Bob 😈', 'a😈', '😈😈', '𝒜lice']) {
      expect(() => encodeURIComponent(getInitials(name))).not.toThrow();
    }
  });
});
