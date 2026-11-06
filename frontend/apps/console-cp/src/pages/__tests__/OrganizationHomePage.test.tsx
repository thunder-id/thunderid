// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent, waitFor, within} from '@thunderid/test-utils';
import type {ReactNode} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import OrganizationHomePage from '../OrganizationHomePage';

const navigate = vi.fn();
const request = vi.fn();
vi.mock('react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-router')>()),
  useNavigate: () => navigate,
}));
vi.mock('@thunderid/react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/react')>()),
  useThunderID: () => ({http: {request}}),
  User: ({children}: {children: (user: unknown) => ReactNode}) => children({name: 'Jerad Smith'}),
}));
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({
      ...actual.useConfig(),
      getServerUrl: () => 'https://cp.test',
      config: {...actual.useConfig().config, documentation: {baseUrl: 'https://docs.test/v1/'}},
    }),
  };
});
const selectProject = vi.fn();
const clearSelection = vi.fn();
vi.mock('@thunderid/configure-projects', () => ({
  useProjectSelection: () => ({
    projects: [
      {id: 'p-shop', displayName: 'Shop'},
      {id: 'p-bill', displayName: 'Billing'},
    ],
    selectProject,
    clearSelection,
  }),
}));
vi.mock('@thunderid/configure-gateways', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/configure-gateways')>();
  return {
    gatewayColor: actual.gatewayColor,
    useAutoApply: () => ({data: {enabled: false}}),
    usePipeline: () => ({
      isLoading: false,
      data: {
        stages: [
          {
            name: 'Development',
            adminConsoleUrl: 'https://dev.auth.test/console',
            gateway: {id: 'gw-dev', name: 'dev', baseUrl: 'https://dev.auth.test'},
          },
          {
            name: 'Production',
            adminConsoleUrl: 'https://auth.test/console',
            gateway: {id: 'gw-prod', name: 'prod', baseUrl: 'https://auth.test'},
          },
        ],
      },
    }),
    useProjects: () => ({
      isLoading: false,
      isSuccess: true,
      data: [
        {id: 'p-shop', displayName: 'Shop'},
        {id: 'p-bill', displayName: 'Billing'},
      ],
    }),
  };
});

const totals: Record<string, number> = {
  '/applications?projectId=p-shop': 2,
  '/agents?projectId=p-shop': 1,
  '/applications?projectId=p-bill': 0,
  '/agents?projectId=p-bill': 0,
};

describe('OrganizationHomePage', () => {
  beforeEach(() => {
    navigate.mockReset();
    selectProject.mockReset();
    clearSelection.mockReset();
    request.mockImplementation((call: {url: string}) => {
      const path = Object.keys(totals).find((entry) =>
        call.url.includes(`${entry}${entry.includes('?') ? '&' : '?'}limit=1`),
      );
      return Promise.resolve({data: {totalResults: path ? totals[path] : 0}});
    });
  });

  it('greets the administrator and says how configuration moves through the gateways', () => {
    render(<OrganizationHomePage />);

    expect(screen.getByText('Hello, Jerad')).toBeInTheDocument();
    const path = screen.getByTestId('organization-how-it-works');
    expect(within(path).getByText('Configure here')).toBeInTheDocument();
    expect(within(path).getByText('Auto-sync')).toBeInTheDocument();
    expect(within(path).getByText('Promote')).toBeInTheDocument();
    const open = within(screen.getByTestId('organization-gateway-gw-dev')).getByRole('link', {
      name: 'Open DEVELOPMENT',
    });
    expect(open).toHaveAttribute('href', '/deploy/gateways/gw-dev');
    expect(open).not.toHaveAttribute('target');
    expect(within(path).getByText('Gateways come from the deployment configuration.')).toBeInTheDocument();
  });

  it('lists the projects with their applications and agents, and opens one', async () => {
    render(<OrganizationHomePage />);

    expect(screen.getByText('Projects · 2')).toBeInTheDocument();
    const shop = screen.getByTestId('organization-project-p-shop');
    await waitFor(() => expect(shop).toHaveTextContent('2 applications · 1 agent'));
    await waitFor(() =>
      expect(screen.getByTestId('organization-project-p-bill')).toHaveTextContent('No applications · No agents'),
    );
    await userEvent.click(shop);

    expect(navigate).toHaveBeenCalledWith('/projects/p-shop');
    expect(selectProject).toHaveBeenCalledWith('p-shop');
  });

  it('is the organization level: opening it leaves the selected project', () => {
    render(<OrganizationHomePage />);

    expect(clearSelection).toHaveBeenCalled();
  });

  it('says what each gateway is for along the path', () => {
    render(<OrganizationHomePage />);

    const roles = screen.getAllByTestId('organization-gateway-role').map((role) => role.textContent);
    expect(roles).toEqual(['Deploy here first', 'Promote from Development · Last stage']);
    expect(screen.getByTestId('organization-gateway-gw-dev')).toHaveTextContent('AppsTest users');
    expect(screen.getByTestId('organization-gateway-gw-prod')).toHaveTextContent('Live appsCustomers, employees');
  });

  it('counts the applications and agents across the projects', async () => {
    render(<OrganizationHomePage />);

    await waitFor(() => expect(screen.getByTestId('organization-applications')).toHaveTextContent('2applications'));
    expect(screen.getByTestId('organization-applications')).toHaveTextContent('Across 1 project');
    expect(screen.getByTestId('organization-agents')).toHaveTextContent('1agents');
    expect(screen.queryByText('Explore more')).not.toBeInTheDocument();
  });

  it('asks which project a new application goes in, then opens its creation in that project', async () => {
    render(<OrganizationHomePage />);

    await userEvent.click(
      within(screen.getByTestId('organization-applications')).getByRole('button', {name: 'New application'}),
    );
    const picker = await screen.findByTestId('project-picker');
    await userEvent.click(within(picker).getByTestId('project-picker-p-bill'));
    await userEvent.click(screen.getByTestId('project-picker-continue'));

    expect(selectProject).toHaveBeenCalledWith('p-bill');
    expect(navigate).toHaveBeenCalledWith('/applications/types');
  });

  it("asks which project to deploy, then opens that project's home", async () => {
    render(<OrganizationHomePage />);

    await userEvent.click(screen.getByTestId('organization-deploy'));
    expect(
      screen.getByText('Choose the project to deploy. Each project deploys through its own pipeline.'),
    ).toBeInTheDocument();
    await userEvent.click(screen.getByTestId('project-picker-continue'));

    expect(navigate).toHaveBeenCalledWith('/projects/p-shop');
  });

  it('opens a new agent in the chosen project', async () => {
    render(<OrganizationHomePage />);

    await userEvent.click(within(screen.getByTestId('organization-agents')).getByRole('button', {name: 'New agent'}));
    await userEvent.click(screen.getByTestId('project-picker-continue'));

    expect(navigate).toHaveBeenCalledWith('/agents/create');
  });

  it('offers no administrators section, and reads no administrator count', async () => {
    render(<OrganizationHomePage />);

    await screen.findByTestId('organization-projects');
    expect(screen.queryByTestId('organization-administrators')).not.toBeInTheDocument();
    expect(screen.queryByText('Invite administrator')).not.toBeInTheDocument();
    expect(screen.queryByText('Manage administrators')).not.toBeInTheDocument();
    expect(
      request.mock.calls.some((args: unknown[]) => String((args[0] as {url: string}).url).includes('/users')),
    ).toBe(false);
  });
});
