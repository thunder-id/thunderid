// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@testing-library/react';
import {useContext} from 'react';
import {describe, it, expect} from 'vitest';
import InteractionContext from '../InteractionContext';

describe('InteractionContext', () => {
  it('should have correct displayName', () => {
    expect(InteractionContext.displayName).toBe('InteractionContext');
  });

  it('should default to undefined when used without a provider', () => {
    const {result} = renderHook(() => useContext(InteractionContext));

    expect(result.current).toBeUndefined();
  });
});
