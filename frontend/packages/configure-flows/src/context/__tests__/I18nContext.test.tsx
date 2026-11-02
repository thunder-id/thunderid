// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@testing-library/react';
import {useContext} from 'react';
import {describe, it, expect} from 'vitest';
import I18nContext from '../I18nContext';

describe('I18nContext', () => {
  it('should have correct displayName', () => {
    expect(I18nContext.displayName).toBe('I18nContext');
  });

  it('should default to undefined when used without a provider', () => {
    const {result} = renderHook(() => useContext(I18nContext));

    expect(result.current).toBeUndefined();
  });
});
