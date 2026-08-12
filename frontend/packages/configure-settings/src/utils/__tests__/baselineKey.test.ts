// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import {AllowedOriginTypes} from '../../models/allowedOriginRow';
import {createRow} from '../allowedOriginRows';
import baselineKey from '../baselineKey';

const origin = (value: string) => createRow(AllowedOriginTypes.ORIGIN, value);
const regex = (value: string) => createRow(AllowedOriginTypes.REGEX, value);

describe('baselineKey', () => {
  it('is equal for rows that normalize to the same entries', () => {
    expect(baselineKey([origin('HTTPS://App.IO/'), origin('')])).toBe(baselineKey([origin('https://app.io')]));
  });

  it('ignores row ids, which are client-only', () => {
    expect(baselineKey([origin('https://a.io')])).toBe(baselineKey([origin('https://a.io')]));
  });

  it('differs when the values differ', () => {
    expect(baselineKey([origin('https://a.io')])).not.toBe(baselineKey([origin('https://b.io')]));
  });

  it('differs when only the type changed', () => {
    expect(baselineKey([origin('https://a.io')])).not.toBe(baselineKey([regex('https://a.io')]));
  });

  it('is order-sensitive', () => {
    expect(baselineKey([origin('https://a.io'), origin('https://b.io')])).not.toBe(
      baselineKey([origin('https://b.io'), origin('https://a.io')]),
    );
  });
});
