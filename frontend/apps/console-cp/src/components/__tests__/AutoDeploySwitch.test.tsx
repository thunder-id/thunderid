// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import AutoDeploySwitch from '../AutoDeploySwitch';

const showToast = vi.hoisted(() => vi.fn());
vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useToast: () => ({showToast}),
}));

vi.mock('@thunderid/configure-projects', () => ({
  useProjectSelection: () => ({projects: [{id: 'p-shop', displayName: 'Shop'}]}),
}));

const state = vi.hoisted(() => ({
  enabled: false,
  pending: 0,
  setAutoApply: vi.fn(),
  applyVersion: vi.fn(),
}));
vi.mock('@thunderid/configure-gateways', () => ({
  useAutoApply: () => ({isLoading: false, isError: false, data: {enabled: state.enabled, locked: false}}),
  useSetAutoApply: () => ({mutateAsync: state.setAutoApply, isPending: false}),
  useApplyVersion: () => ({mutateAsync: state.applyVersion, isPending: false}),
  useProjectsPending: () => ({byProject: {'p-shop': state.pending}, shared: 0, isLoading: false, isError: false}),
  usePipeline: () => ({
    data: {stages: [{name: 'Development', gateway: {id: 'gw-dev', name: 'dev', baseUrl: 'https://dev.test'}}]},
  }),
}));

describe('AutoDeploySwitch', () => {
  beforeEach(() => {
    state.enabled = false;
    state.pending = 0;
    state.setAutoApply.mockReset().mockResolvedValue(undefined);
    state.applyVersion.mockReset().mockResolvedValue({});
    showToast.mockReset();
  });

  it('asks first, saying it reaches every project, then turns auto deploy on', async () => {
    render(<AutoDeploySwitch />);

    await userEvent.click(screen.getByRole('checkbox', {name: 'Auto deploy configuration to Development'}));
    expect(screen.getByText('Turn on auto deploy to Development?')).toBeInTheDocument();
    expect(screen.getByText('This applies to all projects in the organization.')).toBeInTheDocument();
    await userEvent.click(screen.getByTestId('auto-deploy-confirm-button'));

    expect(state.setAutoApply).toHaveBeenCalledWith(true);
    expect(state.applyVersion).not.toHaveBeenCalled();
  });

  it('deploys what is waiting first, and only then turns auto deploy on', async () => {
    state.pending = 2;
    render(<AutoDeploySwitch />);

    await userEvent.click(screen.getByRole('checkbox', {name: 'Auto deploy configuration to Development'}));
    expect(screen.getByText('2 changes behind')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', {name: 'Deploy and turn on'}));

    expect(state.applyVersion).toHaveBeenCalledWith({gatewayId: 'gw-dev', data: {version: 'latest'}});
    expect(state.setAutoApply).toHaveBeenCalledWith(true);
  });

  it('tells in a toast when what is waiting could not be deployed, leaving auto deploy off and the dialog open', async () => {
    state.pending = 2;
    state.applyVersion.mockRejectedValue(new Error('missing values'));
    render(<AutoDeploySwitch />);

    await userEvent.click(screen.getByRole('checkbox', {name: 'Auto deploy configuration to Development'}));
    await userEvent.click(screen.getByRole('button', {name: 'Deploy and turn on'}));

    expect(showToast).toHaveBeenCalledWith(
      'The pending changes could not be deployed to Development, so auto deploy is still off.',
      'error',
    );
    expect(state.setAutoApply).not.toHaveBeenCalled();
    expect(screen.getByText('Turn on auto deploy to Development?')).toBeInTheDocument();
  });

  it('asks before turning auto deploy off', async () => {
    state.enabled = true;
    render(<AutoDeploySwitch />);

    await userEvent.click(screen.getByRole('checkbox', {name: 'Auto deploy configuration to Development'}));
    expect(screen.getByText('Turn off auto deploy to Development?')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', {name: 'Turn off'}));

    expect(state.setAutoApply).toHaveBeenCalledWith(false);
  });
});
