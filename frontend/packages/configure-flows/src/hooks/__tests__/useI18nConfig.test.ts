// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@testing-library/react';
import {describe, it, expect} from 'vitest';
import useI18nConfig from '../useI18nConfig';

describe('useI18nConfig', () => {
  it('should throw when used outside a FlowBuilderCoreProvider', () => {
    expect(() => renderHook(() => useI18nConfig())).toThrow(
      'useI18nConfig must be used within a FlowBuilderCoreProvider',
    );
  });
});
