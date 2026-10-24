// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import {ElementTypes} from '../../models/elements';
import {toApiInputType, toElementType} from '../inputTypeMapping';

describe('inputTypeMapping', () => {
  it('maps the checkbox element to the API boolean input type', () => {
    expect(toApiInputType(ElementTypes.Checkbox)).toBe('BOOLEAN_INPUT');
  });

  it('leaves element types that already match the API unchanged', () => {
    expect(toApiInputType(ElementTypes.TextInput)).toBe('TEXT_INPUT');
    expect(toApiInputType(ElementTypes.PasswordInput)).toBe('PASSWORD_INPUT');
  });

  it('maps the API boolean input type back to the checkbox element', () => {
    expect(toElementType('BOOLEAN_INPUT')).toBe(ElementTypes.Checkbox);
  });

  it('leaves API input types with no Console counterpart unchanged', () => {
    expect(toElementType('TEXT_INPUT')).toBe('TEXT_INPUT');
  });
});
