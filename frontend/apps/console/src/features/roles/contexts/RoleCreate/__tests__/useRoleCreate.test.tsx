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
import {render, screen} from '@thunderid/test-utils';
import React from 'react';
import {describe, expect, it, vi} from 'vitest';
import RoleCreateProvider from '../RoleCreateProvider';
import useRoleCreate from '../useRoleCreate';

function renderUseRoleCreate() {
  return renderHook(() => useRoleCreate(), {
    wrapper: ({children}: {children: React.ReactNode}) => <RoleCreateProvider>{children}</RoleCreateProvider>,
  });
}

function TestConsumer() {
  const context = useRoleCreate();

  return <div data-testid="context-available">{typeof context}</div>;
}

function TestConsumerWithoutProvider() {
  const context = useRoleCreate();

  return <div data-testid="context">{JSON.stringify(context)}</div>;
}

function TestWrapper({children}: {children: React.ReactNode}) {
  return children;
}

describe('useRoleCreate', () => {
  it('returns context when used within RoleCreateProvider', () => {
    render(
      <TestWrapper>
        <RoleCreateProvider>
          <TestConsumer />
        </RoleCreateProvider>
      </TestWrapper>,
    );

    expect(screen.getByTestId('context-available')).toHaveTextContent('object');
  });

  it('throws error when used outside RoleCreateProvider', () => {
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {
      /* noop */
    });

    expect(() => {
      render(<TestConsumerWithoutProvider />);
    }).toThrow('useRoleCreate must be used within a RoleCreateProvider');

    errorSpy.mockRestore();
  });

  it('provides all required context properties', () => {
    function TestContextProperties() {
      const context = useRoleCreate();

      const requiredProperties = [
        'currentStep',
        'setCurrentStep',
        'name',
        'setName',
        'ouId',
        'setOuId',
        'error',
        'setError',
        'permissions',
        'setPermissions',
        'reset',
      ];

      const missingProperties = requiredProperties.filter((prop) => !(prop in context));

      return (
        <div>
          <div data-testid="missing-properties">{JSON.stringify(missingProperties)}</div>
          <div data-testid="has-all-properties">{missingProperties.length === 0 ? 'true' : 'false'}</div>
        </div>
      );
    }

    render(
      <TestWrapper>
        <RoleCreateProvider>
          <TestContextProperties />
        </RoleCreateProvider>
      </TestWrapper>,
    );

    expect(screen.getByTestId('has-all-properties')).toHaveTextContent('true');
    expect(screen.getByTestId('missing-properties')).toHaveTextContent('[]');
  });

  it('returns same context reference across multiple hook calls', () => {
    function TestMultipleHookCalls() {
      const context1 = useRoleCreate();
      const context2 = useRoleCreate();

      return (
        <div>
          <div data-testid="same-reference">{(context1 === context2).toString()}</div>
        </div>
      );
    }

    render(
      <TestWrapper>
        <RoleCreateProvider>
          <TestMultipleHookCalls />
        </RoleCreateProvider>
      </TestWrapper>,
    );

    expect(screen.getByTestId('same-reference')).toHaveTextContent('true');
  });

  it('provides functions that are properly typed', () => {
    function TestFunctionTypes() {
      const {setCurrentStep, setName, setOuId, setError, setPermissions, reset} = useRoleCreate();

      return (
        <div>
          <div data-testid="setCurrentStep-type">{typeof setCurrentStep}</div>
          <div data-testid="setName-type">{typeof setName}</div>
          <div data-testid="setOuId-type">{typeof setOuId}</div>
          <div data-testid="setError-type">{typeof setError}</div>
          <div data-testid="setPermissions-type">{typeof setPermissions}</div>
          <div data-testid="reset-type">{typeof reset}</div>
        </div>
      );
    }

    render(
      <TestWrapper>
        <RoleCreateProvider>
          <TestFunctionTypes />
        </RoleCreateProvider>
      </TestWrapper>,
    );

    expect(screen.getByTestId('setCurrentStep-type')).toHaveTextContent('function');
    expect(screen.getByTestId('setName-type')).toHaveTextContent('function');
    expect(screen.getByTestId('setOuId-type')).toHaveTextContent('function');
    expect(screen.getByTestId('setError-type')).toHaveTextContent('function');
    expect(screen.getByTestId('setPermissions-type')).toHaveTextContent('function');
    expect(screen.getByTestId('reset-type')).toHaveTextContent('function');
  });

  it('throws descriptive error message when used outside provider', () => {
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {
      /* noop */
    });

    let thrownError: Error | null = null;

    try {
      render(<TestConsumerWithoutProvider />);
    } catch (error) {
      thrownError = error as Error;
    }

    expect(thrownError).toBeInstanceOf(Error);
    expect(thrownError?.message).toBe('useRoleCreate must be used within a RoleCreateProvider');

    errorSpy.mockRestore();
  });

  it('exposes permissions state, defaults to empty, and resets it', () => {
    const {result} = renderUseRoleCreate();
    expect(result.current.permissions).toEqual([]);

    act(() => {
      result.current.setPermissions([{resourceServerId: 'rs-1', permissions: ['bookings']}]);
    });
    expect(result.current.permissions).toEqual([{resourceServerId: 'rs-1', permissions: ['bookings']}]);

    act(() => {
      result.current.reset();
    });
    expect(result.current.permissions).toEqual([]);
  });

  it('has exactly 11 properties in the context interface', () => {
    function TestContextProperties() {
      const context = useRoleCreate();

      return (
        <div>
          <div data-testid="property-count">{Object.keys(context).length}</div>
        </div>
      );
    }

    render(
      <TestWrapper>
        <RoleCreateProvider>
          <TestContextProperties />
        </RoleCreateProvider>
      </TestWrapper>,
    );

    expect(screen.getByTestId('property-count')).toHaveTextContent('11');
  });
});
