// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useRuntimeEnvironments, type RuntimeEnvironmentsContextType} from '@thunderid/contexts';
import {render, screen, waitFor} from '@thunderid/test-utils';
import {useEffect} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {DeploySourceProvider, type DeploySource} from '../../api/source';
import {GATEWAY_COLORS} from '../../constants/gateway-colors';
import type {ValueReference} from '../../models/deploy';
import RuntimeEnvironmentsProvider from '../RuntimeEnvironmentsProvider';

vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/react')>();
  return {
    ...actual,
    useThunderID: () => ({http: {request: () => Promise.reject(new Error('the control plane is not called'))}}),
  };
});

const clientIdOf = (resourceId: string, name: string): ValueReference => ({
  name,
  kind: 'variable',
  resourceType: 'application',
  resourceId,
  field: 'clientId',
});

// What each environment runs and holds: Development runs Orders with a client ID, QA runs it without one,
// and Production does not run it.
const references: Record<string, ValueReference[]> = {
  'env-dev': [clientIdOf('app-orders', 'APPLICATION__ORDERS__CLIENT_ID')],
  'env-qa': [clientIdOf('app-orders', 'APPLICATION__ORDERS__CLIENT_ID')],
  'env-prod': [clientIdOf('app-billing', 'APPLICATION__BILLING__CLIENT_ID')],
};
const stored: Record<string, {name: string; value?: string}[]> = {
  'env-dev': [{name: 'APPLICATION__ORDERS__CLIENT_ID', value: 'orders-dev'}],
  'env-qa': [],
  'env-prod': [{name: 'APPLICATION__BILLING__CLIENT_ID', value: 'billing-prod'}],
};

const runningReferences = vi.fn((targetId: string) => Promise.resolve(references[targetId] ?? []));
const values = vi.fn((targetId: string) => Promise.resolve(stored[targetId] ?? []));

/** A source with four stages, the third of which names no base URL. */
function source(pipelineFails = false): DeploySource {
  const unused = (): Promise<never> => Promise.reject(new Error('not used here'));
  return {
    targets: unused,
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
    values,
    setValue: unused,
    deleteValue: unused,
    pipeline: () =>
      pipelineFails
        ? Promise.reject(new Error('the pipeline cannot be read'))
        : Promise.resolve({
            stages: [
              {name: 'Development', gateway: {id: 'env-dev', name: 'dev', baseUrl: 'https://dev.example.com/'}},
              {name: 'QA', gateway: {id: 'env-qa', name: 'qa', baseUrl: 'https://qa.example.com'}},
              {name: 'Staging', gateway: {id: 'env-stage', name: 'stage', baseUrl: ''}},
              {name: 'Production', gateway: {id: 'env-prod', name: 'prod', baseUrl: 'https://prod.example.com'}},
            ],
          }),
    projects: unused,
    autoApply: unused,
    setAutoApply: unused,
    setProjectAutoSync: unused,
    projectStatus: unused,
    runningReferences,
  };
}

const onProvided = vi.fn<(provided: RuntimeEnvironmentsContextType | undefined) => void>();

/** The environments the provider gives the page below it, as the last render saw them. */
const provided = (): RuntimeEnvironmentsContextType | undefined => onProvided.mock.lastCall?.[0];

function Environments() {
  const environments = useRuntimeEnvironments();
  useEffect(() => onProvided(environments), [environments]);
  return (
    <ul>
      {environments?.environments.map((environment) => (
        <li key={environment.id}>{`${environment.name} ${environment.runtimeUrl}`}</li>
      ))}
    </ul>
  );
}

function renderProvider(pipelineFails = false) {
  render(
    <DeploySourceProvider source={source(pipelineFails)}>
      <RuntimeEnvironmentsProvider>
        <Environments />
      </RuntimeEnvironmentsProvider>
    </DeploySourceProvider>,
  );
}

describe('RuntimeEnvironmentsProvider', () => {
  beforeEach(() => {
    onProvided.mockReset();
  });

  it('provides the pipeline stages with a base URL as environments, in order', async () => {
    renderProvider();

    await waitFor(() => expect(screen.getAllByRole('listitem')).toHaveLength(3));
    expect(screen.getAllByRole('listitem').map((item) => item.textContent)).toEqual([
      'Development https://dev.example.com',
      'QA https://qa.example.com',
      'Production https://prod.example.com',
    ]);
  });

  it('keeps the color of each environment at its place in the pipeline', async () => {
    renderProvider();

    await waitFor(() => expect(provided()?.environments).toHaveLength(3));
    expect(provided()?.environments.map((environment) => environment.color)).toEqual([
      GATEWAY_COLORS[0],
      GATEWAY_COLORS[1],
      GATEWAY_COLORS[3],
    ]);
  });

  it('reads the value a field holds in an environment under the name its running version refers to', async () => {
    renderProvider();
    await waitFor(() => expect(provided()?.environments).toHaveLength(3));

    await expect(
      provided()!.getValue('env-dev', {resourceType: 'application', resourceId: 'app-orders', field: 'clientId'}),
    ).resolves.toEqual({status: 'set', value: 'orders-dev'});
    expect(runningReferences).toHaveBeenCalledWith('env-dev');
    expect(values).toHaveBeenCalledWith('env-dev', 'variable');
  });

  it('tells an environment that runs the resource without a value from one that does not run it', async () => {
    renderProvider();
    await waitFor(() => expect(provided()?.environments).toHaveLength(3));
    const field = {resourceType: 'application', resourceId: 'app-orders', field: 'clientId'};

    await expect(provided()!.getValue('env-qa', field)).resolves.toEqual({status: 'unset'});
    await expect(provided()!.getValue('env-prod', field)).resolves.toEqual({status: 'absent'});
  });

  it('says so in a toast when the pipeline cannot be read, and provides no environments', async () => {
    renderProvider(true);

    expect(
      await screen.findByText(
        'The environments could not be loaded. The values shown are for the default environment.',
      ),
    ).toBeInTheDocument();
    expect(provided()?.environments).toEqual([]);
  });
});
