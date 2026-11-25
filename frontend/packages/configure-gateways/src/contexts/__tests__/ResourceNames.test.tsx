// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@thunderid/test-utils';
import {useContext} from 'react';
import {describe, expect, it} from 'vitest';
import ResourceNamesContext from '../ResourceNames';

describe('ResourceNamesContext', () => {
  it('names nothing outside a provider', () => {
    const {result} = renderHook(() => useContext(ResourceNamesContext));

    expect(result.current.size).toBe(0);
  });
});
