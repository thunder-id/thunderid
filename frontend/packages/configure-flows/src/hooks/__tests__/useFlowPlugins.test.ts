// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@testing-library/react';
import {describe, it, expect} from 'vitest';
import useFlowPlugins from '../useFlowPlugins';

describe('useFlowPlugins', () => {
  it('should throw when used outside a FlowBuilderCoreProvider', () => {
    expect(() => renderHook(() => useFlowPlugins())).toThrow(
      'useFlowPlugins must be used within a FlowBuilderCoreProvider',
    );
  });
});
