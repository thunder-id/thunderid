// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@testing-library/react';
import {describe, it, expect} from 'vitest';
import useFlowConfig from '../useFlowConfig';

describe('useFlowConfig', () => {
  it('should throw when used outside a FlowBuilderCoreProvider', () => {
    expect(() => renderHook(() => useFlowConfig())).toThrow(
      'useFlowConfig must be used within a FlowBuilderCoreProvider',
    );
  });
});
