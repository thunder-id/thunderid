// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@thunderid/test-utils';
import {describe, expect, it, vi} from 'vitest';
import {fakeHttp, SERVER, type Call} from '../../__tests__/fakeHttp';
import ResourceNamesProvider from '../../components/ResourceNamesProvider';
import useChangeName, {type Named} from '../useChangeName';

const http = fakeHttp({
  'GET /gateways/gw-dev/applied-configuration': {
    gatewayId: 'gw-dev',
    resources: [
      {
        resourceType: 'theme',
        id: '01a1b0f8-daf7-4c1e',
        resource: {id: '01a1b0f8-daf7-4c1e', displayName: 'Shared Teal'},
      },
      {resourceType: 'agent', id: 'ag-77', resource: {id: 'ag-77', attributes: {name: 'Calendar Agent'}}},
      {resourceType: 'role', id: 'r-9', resource: {id: 'r-9'}},
    ],
  },
});
vi.mock('@thunderid/react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/react')>()),
  useThunderID: () => ({http: {request: (call: Call) => http.request(call)}}),
}));
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useConfig: () => ({...actual.useConfig(), getServerUrl: () => SERVER})};
});

function Names({named}: {named: Named[]}) {
  const nameOf = useChangeName();
  return (
    <ul>
      {named.map((entry: Named) => (
        <li key={entry.key} data-testid={entry.key}>
          {nameOf(entry)}
        </li>
      ))}
    </ul>
  );
}

const named: Named[] = [
  {key: 'theme/01a1b0f8-daf7-4c1e', resourceType: 'theme', id: '01a1b0f8-daf7-4c1e'},
  {key: 'agent/ag-77', resourceType: 'agent'},
  {key: 'role/r-9', resourceType: 'role', id: 'r-9'},
  {key: 'user_type/9f3c2d1e-aaaa', resourceType: 'user_type', id: '9f3c2d1e-aaaa'},
  {key: 'layout/l-1', resourceType: 'layout', id: 'l-1', name: 'Centered'},
];

describe('useChangeName', () => {
  it("names a resource as the gateways' maps do, else by its kind and the start of its id", async () => {
    render(
      <ResourceNamesProvider targetIds={['gw-dev', '']}>
        <Names named={named} />
      </ResourceNamesProvider>,
    );

    expect(await screen.findByText('Shared Teal')).toBeInTheDocument();
    expect(screen.getByTestId('agent/ag-77')).toHaveTextContent('Calendar Agent');
    // A map that names a resource only by its id does not name it.
    expect(screen.getByTestId('role/r-9')).toHaveTextContent('Role r-9');
    expect(screen.getByTestId('user_type/9f3c2d1e-aaaa')).toHaveTextContent('User type 9f3c2d1e');
    expect(screen.getByTestId('layout/l-1')).toHaveTextContent('Centered');
  });

  it('names by kind and id outside a provider', () => {
    render(<Names named={[{key: 'flow/f-12345678-9', resourceType: 'flow'}]} />);

    expect(screen.getByTestId('flow/f-12345678-9')).toHaveTextContent('Flow f-123456');
  });
});
