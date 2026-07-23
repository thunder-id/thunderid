/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {act, renderHook} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import useExternalLinkConfirmation from '../useExternalLinkConfirmation';

describe('useExternalLinkConfirmation', () => {
  beforeEach(() => {
    vi.spyOn(window, 'open').mockImplementation(() => null);
  });

  afterEach(() => {
    vi.mocked(window.open).mockRestore();
  });

  it('starts closed with no pending URL', () => {
    const {result} = renderHook(() => useExternalLinkConfirmation());

    expect(result.current.isOpen).toBe(false);
    expect(result.current.pendingUrl).toBeUndefined();
  });

  it('opens with the requested URL', () => {
    const {result} = renderHook(() => useExternalLinkConfirmation());

    act(() => result.current.requestNavigation('https://example.com'));

    expect(result.current.isOpen).toBe(true);
    expect(result.current.pendingUrl).toBe('https://example.com');
  });

  it('opens the URL and closes on confirm', () => {
    const {result} = renderHook(() => useExternalLinkConfirmation());

    act(() => result.current.requestNavigation('https://example.com'));
    act(() => result.current.confirm());

    expect(window.open).toHaveBeenCalledWith('https://example.com', '_blank', 'noopener,noreferrer');
    expect(result.current.isOpen).toBe(false);
    expect(result.current.pendingUrl).toBeUndefined();
  });

  it('closes without opening the URL on cancel', () => {
    const {result} = renderHook(() => useExternalLinkConfirmation());

    act(() => result.current.requestNavigation('https://example.com'));
    act(() => result.current.cancel());

    expect(window.open).not.toHaveBeenCalled();
    expect(result.current.isOpen).toBe(false);
    expect(result.current.pendingUrl).toBeUndefined();
  });
});
