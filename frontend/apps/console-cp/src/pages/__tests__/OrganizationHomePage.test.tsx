// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent, waitFor, within} from '@thunderid/test-utils';
import type {ReactNode} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import OrganizationHomePage from '../OrganizationHomePage';

const navigate = vi.fn();
// The pipeline's stages; a test may leave only the first.
const pipeline = vi.hoisted(() => ({only: false, fails: false, loading: false}));
const pipelineRefetch = vi.hoisted(() => vi.fn());
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
vi.mock('@thunderid/configure-projects', () => ({
  useProjectSelection: () => ({
    projects: [
      {id: 'p-shop', handle: 'shop', displayName: 'Shop'},
      {id: 'p-bill', handle: 'billing', displayName: 'Billing'},
    ],
    selectProject,
  }),
}));
vi.mock('@thunderid/configure-gateways', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/configure-gateways')>();
  return {
    gatewayColor: actual.gatewayColor,
    usePipeline: () => {
      const stages = [
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
      ];
      if (pipeline.loading) return {isLoading: true, refetch: pipelineRefetch};
      if (pipeline.fails) return {isLoading: false, isError: true, error: new Error('down'), refetch: pipelineRefetch};
      return {isLoading: false, data: {stages: pipeline.only ? stages.slice(0, 1) : stages}};
    },
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
    expect(within(path).getAllByText('Deploy')).toHaveLength(1);
    expect(within(path).getAllByText('Promote')).toHaveLength(1);
    expect(within(path).queryByTestId('organization-gateway-more')).not.toBeInTheDocument();
    // A gateway's page is the organization's, the same as the project home opens.
    const open = within(screen.getByTestId('organization-gateway-gw-dev')).getByRole('link', {
      name: 'Open Development (dev.auth.test)',
    });
    expect(open).toHaveAttribute('href', '/deploy/gateways/gw-dev');
    expect(open).toHaveTextContent('Open gateway');
    // The tiles lead the page, without a heading of their own.
    expect(screen.queryByRole('heading', {name: 'Applications and agents'})).not.toBeInTheDocument();
    expect(
      screen.getByTestId('organization-applications-and-agents').compareDocumentPosition(path) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    // Nothing is waiting, so no deploy is offered.
    expect(screen.queryByTestId('organization-pending')).not.toBeInTheDocument();
    expect(within(path).getByText('Gateways come from the deployment configuration.')).toBeInTheDocument();
  });

  it('says configuration deploys to the one gateway, and offers to add another', () => {
    pipeline.only = true;
    render(<OrganizationHomePage />);
    pipeline.only = false;

    expect(
      screen.getByText('Configure here, then deploy to your gateway. It has its own users and apps.'),
    ).toBeInTheDocument();
    expect(screen.getByTestId('organization-gateway-more')).toHaveTextContent('Add a gateway');
  });

  it('says when the gateways could not be read, and reads them again on retry', async () => {
    pipeline.fails = true;
    render(<OrganizationHomePage />);

    const how = screen.getByTestId('organization-how-it-works');
    expect(within(how).getByRole('alert')).toBeInTheDocument();
    await userEvent.click(within(how).getByRole('button', {name: /refresh/i}));
    pipeline.fails = false;
    expect(pipelineRefetch).toHaveBeenCalled();
  });

  it("holds the diagram's place with a spinner while the gateways are read", () => {
    pipeline.loading = true;
    render(<OrganizationHomePage />);
    pipeline.loading = false;

    expect(screen.getByRole('progressbar', {name: 'Reading the gateways'})).toBeInTheDocument();
    expect(screen.queryByRole('region', {name: 'Configuration flow diagram'})).not.toBeInTheDocument();
  });

  it('links to where the gateways are managed, when the deployment says where', () => {
    // A first render has the test providers set their whole runtime configuration, which the link adds to.
    render(<div />).unmount();
    const previous = window.__THUNDERID_RUNTIME_CONFIG__;
    window.__THUNDERID_RUNTIME_CONFIG__ = {
      ...previous!,
      deploy: {manage_environments_url: 'https://platform.test/environments'},
    };
    render(<OrganizationHomePage />);
    window.__THUNDERID_RUNTIME_CONFIG__ = previous;

    const manage = within(screen.getByTestId('organization-how-it-works')).getByTestId('organization-manage-runtime');
    expect(manage).toHaveAttribute('href', 'https://platform.test/environments');
    expect(manage).toHaveAttribute('target', '_blank');
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

    expect(selectProject).toHaveBeenCalledWith('shop');
  });

  it('says what each gateway is for along the path', () => {
    render(<OrganizationHomePage />);

    const roles = screen.getAllByTestId('organization-gateway-role').map((role) => role.textContent);
    // Which gateway follows which belongs to each project's pipeline, so only the first is named.
    expect(roles).toEqual(['Deploy here first', '']);
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

    expect(selectProject).toHaveBeenCalledWith('billing', '/applications/types');
  });

  it('opens a new agent in the chosen project', async () => {
    render(<OrganizationHomePage />);

    await userEvent.click(within(screen.getByTestId('organization-agents')).getByRole('button', {name: 'New agent'}));
    await userEvent.click(screen.getByTestId('project-picker-continue'));

    expect(selectProject).toHaveBeenCalledWith('shop', '/agents/create');
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
