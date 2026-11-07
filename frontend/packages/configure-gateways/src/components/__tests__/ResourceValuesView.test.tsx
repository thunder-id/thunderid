// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {renderWithProviders, screen, waitFor, within} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {createHttpRouter, SERVER_URL, type HttpCall} from '../../__tests__/http';
import type {ValueReference} from '../../models/deploy';
import ResourceValuesView from '../ResourceValuesView';

const mockHttpRequest = vi.fn<(call: HttpCall) => Promise<unknown>>();
vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/react')>();
  return {...actual, useThunderID: () => ({http: {request: mockHttpRequest}})};
});

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useConfig: () => ({getServerUrl: () => SERVER_URL})};
});

const references: ValueReference[] = [
  {
    name: 'APP_ONE_CLIENT_SECRET',
    kind: 'secret',
    resourceType: 'application',
    resourceId: 'app-1',
    resourceName: 'Customer portal',
    field: 'clientSecret',
  },
  {
    name: 'APP_ONE_CLIENT_ID',
    kind: 'variable',
    resourceType: 'application',
    resourceId: 'app-1',
    resourceName: 'Customer portal',
    field: 'clientId',
  },
  {
    name: 'USER_ADMIN_PASSWORD',
    kind: 'secret',
    resourceType: 'user',
    resourceId: 'u-1',
    resourceName: 'admin',
    field: 'password',
  },
];

beforeEach(() => {
  mockHttpRequest.mockReset();
});

describe('ResourceValuesView', () => {
  it('lists each resource with its values named for a person, set or not', () => {
    renderWithProviders(
      <ResourceValuesView
        gatewayId="gw-1"
        name="Dev"
        references={references}
        variables={[{name: 'APP_ONE_CLIENT_ID', value: 'client-1'}]}
        secrets={[]}
      />,
    );

    const app = screen.getByTestId('resource-card-application/app-1');
    expect(within(app).getByText('Customer portal')).toBeInTheDocument();
    expect(within(app).getByText('Client secret')).toBeInTheDocument();
    expect(within(app).getByText('Client ID')).toBeInTheDocument();
    expect(within(app).getByText('1 to set')).toBeInTheDocument();
    expect(within(screen.getByTestId('resource-value-APP_ONE_CLIENT_ID')).getByText('Set')).toBeInTheDocument();
    expect(within(screen.getByTestId('resource-value-APP_ONE_CLIENT_SECRET')).getByText('Not set')).toBeInTheDocument();
    expect(screen.getByRole('region', {name: 'Applications'})).toBeInTheDocument();
    expect(screen.getByRole('region', {name: 'Users and groups'})).toBeInTheDocument();
    // A secret's value is never shown, and a variable's is not shown on the card either.
    expect(screen.queryByText('client-1')).not.toBeInTheDocument();
  });

  it('sets a value in place', async () => {
    const user = userEvent.setup();
    const calls: HttpCall[] = [];
    mockHttpRequest.mockImplementation(
      createHttpRouter({
        'PUT /gateways/gw-1/secrets/APP_ONE_CLIENT_SECRET': (call: HttpCall) => {
          calls.push(call);
          return {name: 'APP_ONE_CLIENT_SECRET'};
        },
      }),
    );
    renderWithProviders(
      <ResourceValuesView gatewayId="gw-1" name="Dev" references={references} variables={[]} secrets={[]} />,
    );

    await user.click(screen.getByTestId('resource-value-edit-APP_ONE_CLIENT_SECRET'));
    await user.type(screen.getByLabelText('Client secret'), 's3cret');
    await user.click(screen.getByTestId('resource-value-save-APP_ONE_CLIENT_SECRET'));

    await waitFor(() => expect(calls).toHaveLength(1));
    expect(calls[0].data).toMatchObject({value: 's3cret'});
  });

  it('says when nothing uses a value', () => {
    renderWithProviders(<ResourceValuesView gatewayId="gw-1" name="Dev" references={[]} variables={[]} secrets={[]} />);

    expect(screen.getByText('Nothing Dev runs uses a variable or a secret.')).toBeInTheDocument();
  });
});
