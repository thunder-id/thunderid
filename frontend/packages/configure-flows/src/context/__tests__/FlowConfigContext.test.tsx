// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@testing-library/react';
import {useContext} from 'react';
import {describe, it, expect} from 'vitest';
import FlowConfigContext from '../FlowConfigContext';

describe('FlowConfigContext', () => {
  it('should have correct displayName', () => {
    expect(FlowConfigContext.displayName).toBe('FlowConfigContext');
  });

  it('should default to undefined when used without a provider', () => {
    const {result} = renderHook(() => useContext(FlowConfigContext));

    expect(result.current).toBeUndefined();
  });
});
