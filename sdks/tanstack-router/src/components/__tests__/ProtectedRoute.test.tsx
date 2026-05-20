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

import {cleanup, render, screen} from '@testing-library/react';
import {useThunderID} from '@thunderid/react';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import ProtectedRoute from '../ProtectedRoute';

vi.mock('@thunderid/react', () => ({
  ThunderIDRuntimeError: class ThunderIDRuntimeError extends Error {
    code: string;

    component: string;

    traceId: string | undefined;

    constructor(message: string, code: string, component: string, traceId?: string) {
      super(message);
      this.name = 'ThunderIDRuntimeError';
      this.code = code;
      this.component = component;
      this.traceId = traceId;
    }
  },
  useThunderID: vi.fn(),
}));

vi.mock('@tanstack/react-router', () => ({
  Navigate: ({to}: {to: string}): JSX.Element => <div data-testid="navigate">Navigate to: {to}</div>,
}));

describe('ProtectedRoute', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
  });

  it('should render loader when isLoading is true', () => {
    vi.mocked(useThunderID).mockReturnValue({
      isLoading: true,
      isSignedIn: false,
    } as any);

    render(
      <ProtectedRoute redirectTo="/signin" loader={<div data-testid="loader">Loading...</div>}>
        <div data-testid="protected-content">Protected Content</div>
      </ProtectedRoute>,
    );

    expect(screen.getByTestId('loader')).toBeDefined();
    expect(screen.queryByTestId('protected-content')).toBeNull();
  });

  it('should render children when user is authenticated', () => {
    vi.mocked(useThunderID).mockReturnValue({
      isLoading: false,
      isSignedIn: true,
    } as any);

    render(
      <ProtectedRoute redirectTo="/signin">
        <div data-testid="protected-content">Protected Content</div>
      </ProtectedRoute>,
    );

    expect(screen.getByTestId('protected-content')).toBeDefined();
  });

  it('should render fallback when user is not authenticated and fallback is provided', () => {
    vi.mocked(useThunderID).mockReturnValue({
      isLoading: false,
      isSignedIn: false,
    } as any);

    render(
      <ProtectedRoute redirectTo="/signin" fallback={<div data-testid="fallback">Access Denied</div>}>
        <div data-testid="protected-content">Protected Content</div>
      </ProtectedRoute>,
    );

    expect(screen.getByTestId('fallback')).toBeDefined();
    expect(screen.queryByTestId('protected-content')).toBeNull();
  });

  it('should navigate to redirectTo when user is not authenticated and no fallback is provided', () => {
    vi.mocked(useThunderID).mockReturnValue({
      isLoading: false,
      isSignedIn: false,
    } as any);

    render(
      <ProtectedRoute redirectTo="/signin">
        <div data-testid="protected-content">Protected Content</div>
      </ProtectedRoute>,
    );

    const navigate: HTMLElement = screen.getByTestId('navigate');
    expect(navigate).toBeDefined();
    expect(navigate.textContent).toBe('Navigate to: /signin');
    expect(screen.queryByTestId('protected-content')).toBeNull();
  });

  it('should throw error when neither fallback nor redirectTo is provided', () => {
    vi.mocked(useThunderID).mockReturnValue({
      isLoading: false,
      isSignedIn: false,
    } as any);

    expect(() => {
      render(
        <ProtectedRoute>
          <div data-testid="protected-content">Protected Content</div>
        </ProtectedRoute>,
      );
    }).toThrow('"fallback" or "redirectTo" prop is required.');
  });

  it('should render null loader by default when isLoading is true and no loader is provided', () => {
    vi.mocked(useThunderID).mockReturnValue({
      isLoading: true,
      isSignedIn: false,
    } as any);

    const {container} = render(
      <ProtectedRoute redirectTo="/signin">
        <div data-testid="protected-content">Protected Content</div>
      </ProtectedRoute>,
    );

    expect(container.textContent).toBe('');
    expect(screen.queryByTestId('protected-content')).toBeNull();
  });

  it('should prioritize fallback over redirectTo when both are provided', () => {
    vi.mocked(useThunderID).mockReturnValue({
      isLoading: false,
      isSignedIn: false,
    } as any);

    render(
      <ProtectedRoute redirectTo="/signin" fallback={<div data-testid="fallback">Custom Fallback</div>}>
        <div data-testid="protected-content">Protected Content</div>
      </ProtectedRoute>,
    );

    expect(screen.getByTestId('fallback')).toBeDefined();
    expect(screen.queryByTestId('navigate')).toBeNull();
    expect(screen.queryByTestId('protected-content')).toBeNull();
  });
});
