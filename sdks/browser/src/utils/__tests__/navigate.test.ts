/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

/**
 * @vitest-environment jsdom
 */

import {vi, describe, it, expect, beforeEach, afterEach} from 'vitest';
import navigate from '../navigate';

describe('navigate', () => {
  const originalLocation: Location = window.location;

  beforeEach(() => {
    // @ts-ignore
    window.history.pushState = vi.fn();
    // @ts-ignore
    window.dispatchEvent = vi.fn();
    // @ts-ignore
    delete window.location;
    // @ts-ignore
    window.location = {
      ...originalLocation,
      assign: vi.fn(),
      href: 'https://localhost:5173/',
      origin: 'https://localhost:5173',
    };
  });

  afterEach(() => {
    vi.clearAllMocks();
    // @ts-ignore
    window.location = originalLocation;
  });

  it('should call window.history.pushState with the correct arguments for same-origin', () => {
    navigate('/test-url');
    expect(window.history.pushState).toHaveBeenCalledWith(null, '', '/test-url');
    expect(window.location.assign).not.toHaveBeenCalled();
  });

  it('should dispatch a PopStateEvent with state null for same-origin', () => {
    navigate('/test-url');
    expect(window.dispatchEvent).toHaveBeenCalledWith(
      expect.objectContaining({
        state: null,
        type: 'popstate',
      }),
    );
    expect(window.location.assign).not.toHaveBeenCalled();
  });

  it('should use window.location.assign for cross-origin URLs', () => {
    const crossOriginUrl = 'https://accounts.asgardeo.io/t/dxlab/accountrecoveryendpoint/register.do';
    navigate(crossOriginUrl);
    expect(window.location.assign).toHaveBeenCalledWith(crossOriginUrl);
    expect(window.history.pushState).not.toHaveBeenCalled();
    expect(window.dispatchEvent).not.toHaveBeenCalled();
  });

  it('should use window.location.assign for malformed URLs', () => {
    const malformedUrl = 'http://[::1'; // Invalid URL
    navigate(malformedUrl);
    expect(window.location.assign).toHaveBeenCalledWith(malformedUrl);
    expect(window.history.pushState).not.toHaveBeenCalled();
    expect(window.dispatchEvent).not.toHaveBeenCalled();
  });
});
