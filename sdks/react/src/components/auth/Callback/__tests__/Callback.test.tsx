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
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
 * WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
 * License for the specific language governing permissions and limitations
 * under the License.
 */

import {render, screen} from '@testing-library/react';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import {Callback} from '../Callback';

vi.mock('../TokenCallback', () => ({
  TokenCallback: () => <div data-testid="token-callback">TokenCallback</div>,
}));

vi.mock('../OAuthCallback', () => ({
  OAuthCallback: () => <div data-testid="oauth-callback">OAuthCallback</div>,
}));

describe('Callback', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    window.history.replaceState({}, '', '/');
  });

  it('renders TokenCallback when token is present in the URL', () => {
    window.history.replaceState({}, '', '/callback?token=secret-token');
    render(<Callback />);
    expect(screen.getByTestId('token-callback')).toBeDefined();
    expect(screen.queryByTestId('oauth-callback')).toBeNull();
  });

  it('renders OAuthCallback when token is not present in the URL', () => {
    window.history.replaceState({}, '', '/callback?code=oauth-code');
    render(<Callback />);
    expect(screen.getByTestId('oauth-callback')).toBeDefined();
    expect(screen.queryByTestId('token-callback')).toBeNull();
  });

  it('maintains the initial flow type even if URL changes later', () => {
    // Start with token in URL
    window.history.replaceState({}, '', '/callback?token=secret-token');
    const {rerender} = render(<Callback />);
    expect(screen.getByTestId('token-callback')).toBeDefined();

    // Simulate URL change (like what TokenCallback does when it cleans the URL)
    window.history.replaceState({}, '', '/callback');
    rerender(<Callback />);

    // It should STILL render TokenCallback because flowType is locked in state
    expect(screen.getByTestId('token-callback')).toBeDefined();
    expect(screen.queryByTestId('oauth-callback')).toBeNull();
  });
});
