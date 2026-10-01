// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, act} from '@testing-library/react';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';

const mockNavigate = vi.fn();
const mockShowToast = vi.fn();
const mockSessionStorageSetItem = vi.fn();

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({
      config: {
        brand: {product_name: 'ThunderID'},
      },
    }),
    useToast: () => ({showToast: mockShowToast}),
  };
});

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {...actual, useNavigate: () => mockNavigate};
});

import useWelcomeClose from '../useWelcomeClose';

describe('useWelcomeClose', () => {
  beforeEach(() => {
    vi.stubGlobal('sessionStorage', {
      setItem: mockSessionStorageSetItem,
      getItem: vi.fn(),
      removeItem: vi.fn(),
      clear: vi.fn(),
    });
  });

  afterEach(() => {
    vi.clearAllMocks();
    vi.unstubAllGlobals();
  });

  it('returns a function', () => {
    const {result} = renderHook(() => useWelcomeClose());
    expect(typeof result.current).toBe('function');
  });

  it('sets sessionStorage dismissed key on call', () => {
    const {result} = renderHook(() => useWelcomeClose());

    act(() => {
      result.current();
    });

    expect(mockSessionStorageSetItem).toHaveBeenCalledWith('thunderid:welcome:dismissed', 'true');
  });

  it('navigates to /home on call', () => {
    const {result} = renderHook(() => useWelcomeClose());

    act(() => {
      result.current();
    });

    expect(mockNavigate).toHaveBeenCalledWith('/home');
  });

  it('shows an info toast on call', () => {
    const {result} = renderHook(() => useWelcomeClose());

    act(() => {
      result.current();
    });

    expect(mockShowToast).toHaveBeenCalledWith('common:welcome.dismissed', 'info');
  });
});
