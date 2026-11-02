// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@testing-library/react';
import {useContext} from 'react';
import {describe, it, expect} from 'vitest';
import UIPanelContext from '../UIPanelContext';

describe('UIPanelContext', () => {
  it('should have correct displayName', () => {
    expect(UIPanelContext.displayName).toBe('UIPanelContext');
  });

  it('should default to undefined when used without a provider', () => {
    const {result} = renderHook(() => useContext(UIPanelContext));

    expect(result.current).toBeUndefined();
  });
});
