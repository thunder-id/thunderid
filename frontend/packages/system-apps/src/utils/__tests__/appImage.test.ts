// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import resolveAppImage from '../appImage';

describe('resolveAppImage', () => {
  it('takes the dark image in the dark scheme, and the light one otherwise', () => {
    const image = {dark: 'https://cdn.example.com/dark.svg', light: 'https://cdn.example.com/light.svg'};

    expect(resolveAppImage(image, 'dark')).toBe(image.dark);
    expect(resolveAppImage(image, 'light')).toBe(image.light);
  });

  it('falls back to the light image when there is no dark one', () => {
    expect(resolveAppImage({light: 'https://cdn.example.com/light.svg'}, 'dark')).toBe(
      'https://cdn.example.com/light.svg',
    );
  });

  it('has nothing to show without an image', () => {
    expect(resolveAppImage(undefined, 'light')).toBeUndefined();
    expect(resolveAppImage({dark: 'https://cdn.example.com/dark.svg'}, 'light')).toBeUndefined();
  });
});
