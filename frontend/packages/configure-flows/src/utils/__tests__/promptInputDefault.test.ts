// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import {ElementCategories, ElementTypes, type Element} from '../../models/elements';
import {getPromptInputDefault} from '../promptInputDefault';

const input = (type: string, value?: unknown): Element =>
  ({id: 'input-1', type, category: ElementCategories.Field, value}) as unknown as Element;

describe('getPromptInputDefault', () => {
  it.each([
    ElementTypes.TextInput,
    ElementTypes.EmailInput,
    ElementTypes.PhoneInput,
    ElementTypes.NumberInput,
    ElementTypes.DateInput,
  ])('should return the value of a %s input', (type) => {
    expect(getPromptInputDefault(input(type, 'x'))).toBe('x');
  });

  it.each([ElementTypes.PasswordInput, ElementTypes.OtpInput, ElementTypes.Checkbox, ElementTypes.Select])(
    'should ignore the value of a %s input',
    (type) => {
      expect(getPromptInputDefault(input(type, 'x'))).toBeUndefined();
    },
  );

  it.each([[undefined], [''], [5], [true]])('should return nothing for the value %s', (value) => {
    expect(getPromptInputDefault(input(ElementTypes.TextInput, value))).toBeUndefined();
  });
});
