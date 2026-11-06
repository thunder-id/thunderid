// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent} from '@thunderid/test-utils';
import {describe, expect, it, vi} from 'vitest';
import {DeployStatusProvider, type DeployStatus} from '../DeployStatusContext';
import ResourceDeployStatus, {type ResourceDeployStatusProps} from '../ResourceDeployStatus';

// This package's tests run without the app's i18n, so the default text is filled in here.
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (_key: string, fallback: string, options: Record<string, string> = {}) =>
      fallback.replace(/{{(\w+)}}/g, (_match: string, name: string) => options[name] ?? ''),
  }),
}));

function renderStatus(status: DeployStatus | undefined, props: Partial<ResourceDeployStatusProps> = {}) {
  const openDeploy = vi.fn();
  const statusOf = vi.fn(() => status);
  render(
    <DeployStatusProvider value={{statusOf, openDeploy}}>
      <ResourceDeployStatus resourceType="application" id="app-1" {...props} />
    </DeployStatusProvider>,
  );
  return {openDeploy, statusOf};
}

describe('ResourceDeployStatus', () => {
  it('renders nothing in an app that deploys nothing', () => {
    render(<ResourceDeployStatus resourceType="application" id="app-1" />);

    expect(screen.queryByTestId('resource-deploy-status')).not.toBeInTheDocument();
  });

  it('asks the source about the resource and says it is not deployed, opening the deploy path on a click', async () => {
    const {openDeploy, statusOf} = renderStatus({state: 'notDeployed', target: 'Dev'});

    expect(statusOf).toHaveBeenCalledWith('application', 'app-1');
    const chip = screen.getByTestId('resource-deploy-status');
    expect(chip).toHaveTextContent('Not deployed');
    await userEvent.click(chip);
    expect(openDeploy).toHaveBeenCalled();
  });

  it('says which environment refused a resource and why, on its page', () => {
    renderStatus({state: 'refused', target: 'Dev', reason: 'AGT-1039: Owner not found.'}, {variant: 'notice'});

    const notice = screen.getByTestId('resource-deploy-status');
    expect(notice).toHaveTextContent('Refused by Dev: AGT-1039: Owner not found. Fix it, then deploy it to Dev again.');
    expect(notice).toHaveClass('MuiAlert-standardError');
  });

  it('opens the next step for the resource alone when the source can', async () => {
    const openResource = vi.fn();
    render(
      <DeployStatusProvider
        value={{statusOf: () => ({state: 'notPromoted', target: 'Stage'}), openDeploy: vi.fn(), openResource}}
      >
        <ResourceDeployStatus resourceType="application" id="app-1" variant="notice" />
      </DeployStatusProvider>,
    );

    await userEvent.click(screen.getByTestId('resource-deploy-status-action'));
    expect(openResource).toHaveBeenCalledWith('application', 'app-1', 'Stage');
  });

  it('names the environment a resource is not promoted to yet', () => {
    renderStatus({state: 'notPromoted', target: 'Prod'});

    expect(screen.getByTestId('resource-deploy-status')).toHaveTextContent('Not in Prod');
  });

  it('shows nothing in a list row for a resource every environment runs, nor while the status is unknown', () => {
    renderStatus({state: 'live'});
    expect(screen.queryByTestId('resource-deploy-status')).not.toBeInTheDocument();
  });

  it('explains the next step on a resource page, with the action that takes it', async () => {
    const {openDeploy} = renderStatus({state: 'changed', target: 'Dev'}, {variant: 'notice'});

    const notice = screen.getByTestId('resource-deploy-status');
    expect(notice).toHaveTextContent('Changes not deployed');
    expect(notice).toHaveTextContent('Dev still runs the earlier form until you deploy.');
    await userEvent.click(screen.getByRole('button', {name: 'Deploy'}));
    expect(openDeploy).toHaveBeenCalled();
  });

  it('says on a resource page that every environment runs it', () => {
    renderStatus({state: 'live'}, {variant: 'notice'});

    expect(screen.getByTestId('resource-deploy-status')).toHaveTextContent('Live');
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });
});
