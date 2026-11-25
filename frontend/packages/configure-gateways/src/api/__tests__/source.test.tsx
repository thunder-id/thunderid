// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent, waitFor, within} from '@thunderid/test-utils';
import {describe, expect, it, vi} from 'vitest';
import type {Call} from '../../__tests__/fakeHttp';
import GatewayValuesPage from '../../pages/GatewayValuesPage';
import {DeploySourceProvider, type DeploySource} from '../source';

const request = vi.fn((call: Call) => Promise.reject(new Error(`unexpected request: ${call.url}`)));
vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/react')>();
  return {...actual, useThunderID: () => ({http: {request}})};
});
vi.mock('react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router')>();
  return {...actual, useNavigate: () => vi.fn(), useParams: () => ({gatewayId: 'env-stage'})};
});

const listValues = vi.fn((_targetId: string, kind: string) => {
  const values = kind === 'variable' ? [{name: 'APP_ORDERS_CLIENT_ID', value: 'orders-stage'}] : [];
  return Promise.resolve({values, totalResults: values.length});
});
const setValue = vi.fn((_targetId: string, _kind: string, name: string) => Promise.resolve({name}));

/** A source standing for somewhere other than a control plane's gateways, such as environments. */
function environments(): DeploySource {
  const unused = (): Promise<never> => Promise.reject(new Error('not used here'));
  return {
    targets: () => Promise.resolve([{id: 'env-stage', name: 'Staging', baseUrl: 'https://stage.example.com'}]),
    versions: unused,
    version: unused,
    appliedVersion: unused,
    appliedConfiguration: unused,
    master: unused,
    masterHistory: unused,
    resourceHistory: unused,
    applyCheck: unused,
    apply: unused,
    revert: unused,
    listValues,
    setValue,
    deleteValue: unused,
    pipeline: unused,
    projects: unused,
    autoApply: unused,
    setAutoApply: unused,
    setProjectAutoSync: unused,
    projectStatus: unused,
    runningReferences: () =>
      Promise.resolve([
        {
          name: 'APP_ORDERS_CLIENT_ID',
          kind: 'variable',
          resourceType: 'application',
          resourceName: 'Orders',
          field: 'clientId',
        },
        {
          name: 'APP_ORDERS_SECRET',
          kind: 'secret',
          resourceType: 'application',
          resourceName: 'Orders',
          field: 'clientSecret',
        },
      ]),
  };
}

describe('DeploySourceProvider', () => {
  it('serves the pages from the source it provides, without calling the control plane', async () => {
    render(
      <DeploySourceProvider source={environments()}>
        <GatewayValuesPage />
      </DeploySourceProvider>,
    );

    expect(await screen.findByText('Configure Staging')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('tab', {name: /Secrets/}));
    await userEvent.click(await screen.findByRole('button', {name: 'Set APP_ORDERS_SECRET'}));
    await userEvent.type(screen.getByLabelText('APP_ORDERS_SECRET'), 's3cret');
    await userEvent.click(screen.getByRole('button', {name: 'Save'}));

    await waitFor(() =>
      expect(setValue).toHaveBeenCalledWith('env-stage', 'secret', 'APP_ORDERS_SECRET', {
        value: 's3cret',
        description: undefined,
      }),
    );
    expect(listValues).toHaveBeenCalledWith('env-stage', 'variable', {
      names: ['APP_ORDERS_CLIENT_ID'],
      limit: 100,
      offset: 0,
    });
    expect(listValues).toHaveBeenCalledWith('env-stage', 'variable', {limit: 10, offset: 0});
    expect(within(screen.getByTestId('values-secret')).getByText('Applications: Orders')).toBeInTheDocument();
    expect(request).not.toHaveBeenCalled();
  });
});
