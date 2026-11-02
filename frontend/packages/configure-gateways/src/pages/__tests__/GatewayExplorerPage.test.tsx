// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen, userEvent, waitFor, within} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {fakeHttp, SERVER, type Call} from '../../__tests__/fakeHttp';
import GatewayExplorerPage from '../GatewayExplorerPage';

let http = fakeHttp({});
vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/react')>();
  return {...actual, useThunderID: () => ({http: {request: (call: Call) => http.request(call)}})};
});
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useConfig: () => ({...actual.useConfig(), getServerUrl: () => SERVER})};
});
const navigate = vi.fn();
let gatewayId = 'gw-dev';
vi.mock('react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router')>();
  return {...actual, useNavigate: () => navigate, useParams: () => ({gatewayId})};
});

const hash = '2f2d727aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa';
const gateways = [
  {id: 'gw-dev', name: 'Dev', baseUrl: 'https://localhost:8090', isDefault: true},
  {id: 'gw-prod', name: 'Prod', baseUrl: 'https://localhost:8097'},
];
const configuration = {
  gatewayId: 'gw-dev',
  version: hash,
  appliedAt: '2026-10-12T13:55:00Z',
  resources: [
    {resourceType: 'organization_unit', id: 'root', resource: {id: 'root', name: 'ThunderID'}},
    {
      resourceType: 'user_type',
      id: 'ut-1',
      resource: {id: 'ut-1', handle: 'person', displayName: 'Person', ouId: 'root'},
    },
    {
      resourceType: 'application',
      id: 'app-1',
      resource: {id: 'app-1', name: 'Orders', type: 'browser', allowedUserTypes: ['person'], authFlowId: 'f-1'},
    },
    {
      resourceType: 'flow',
      id: 'f-1',
      resource: {id: 'f-1', name: 'Sign in', nodes: [{id: 'g', executor: {properties: {idpName: 'Google'}}}]},
    },
    {resourceType: 'connection', id: 'c-1', resource: {id: 'c-1', name: 'Google', type: 'google'}},
  ],
};
const references = [
  {name: 'APP_ORDERS_CLIENT_ID', kind: 'variable', resourceType: 'application', resourceId: 'app-1', field: 'clientId'},
  {
    name: 'APP_ORDERS_REDIRECT_URIS',
    kind: 'variable',
    resourceType: 'application',
    resourceId: 'app-1',
    field: 'redirectUris',
    list: true,
  },
  {
    name: 'CONNECTION_GOOGLE_CLIENT_SECRET',
    kind: 'secret',
    resourceType: 'connection',
    resourceId: 'c-1',
    field: 'clientSecret',
  },
];
const page = (kind: string, values: unknown[]) => ({totalResults: values.length, [kind]: values});
let routes: Record<string, unknown>;

beforeEach(() => {
  navigate.mockReset();
  gatewayId = 'gw-dev';
  routes = {
    'GET /gateways': gateways,
    'GET /configuration-versions': [{version: hash, createdAt: '2026-10-12T13:52:47Z'}],
    [`GET /configuration-versions/${hash}`]: {version: hash, createdAt: '2026-10-12T13:52:47Z', references},
    'GET /gateways/gw-dev/applied-configuration': configuration,
    'GET /gateways/gw-dev/applied-version': {gatewayId: 'gw-dev', appliedVersion: hash},
    'GET /gateways/gw-dev/variables': page('variables', [
      {name: 'APP_ORDERS_CLIENT_ID', value: 'orders'},
      {name: 'APP_ORDERS_REDIRECT_URIS', value: '["https://a.test/cb","https://a.test/alt"]'},
    ]),
    'GET /gateways/gw-dev/secrets': page('secrets', [{name: 'CONNECTION_GOOGLE_CLIENT_SECRET', exists: true}]),
    'PUT /gateways/gw-dev/variables/APP_ORDERS_REDIRECT_URIS': {name: 'APP_ORDERS_REDIRECT_URIS'},
    'PUT /gateways/gw-dev/secrets/CONNECTION_GOOGLE_CLIENT_SECRET': {
      name: 'CONNECTION_GOOGLE_CLIENT_SECRET',
      exists: true,
    },
  };
  http = fakeHttp(routes);
});

describe('GatewayExplorerPage', () => {
  it('pans by dragging and by the wheel, zooms by its buttons, and keeps a selection a drag ends on', async () => {
    render(<GatewayExplorerPage />);
    await screen.findByTestId('explorer-node-application/app-1');
    const canvas = screen.getByTestId('explorer-canvas');
    const drawing = canvas.firstElementChild as HTMLElement;
    const translation = (): string => getComputedStyle(drawing).transform;

    // The zoom buttons change the scale, shown as a percentage.
    const initial = screen.getByTestId('explorer-zoom').textContent;
    expect(initial).toMatch(/^\d+%$/);
    await userEvent.click(screen.getByRole('button', {name: 'Zoom in'}));
    expect(screen.getByTestId('explorer-zoom')).not.toHaveTextContent(initial ?? '');
    await userEvent.click(screen.getByRole('button', {name: 'Zoom out'}));
    expect(screen.getByTestId('explorer-zoom')).toHaveTextContent(initial ?? '');

    // A drag on the canvas moves the drawing, and the click that ends it leaves the selection alone.
    await userEvent.click(screen.getByTestId('explorer-node-application/app-1'));
    expect(screen.getByTestId('explorer-panel')).toHaveTextContent('Orders');
    const before = translation();
    fireEvent.pointerDown(canvas, {button: 0, pointerId: 1, clientX: 100, clientY: 100});
    fireEvent.pointerMove(canvas, {pointerId: 1, clientX: 140, clientY: 130});
    fireEvent.pointerUp(canvas, {pointerId: 1, clientX: 140, clientY: 130});
    fireEvent.click(canvas);
    expect(translation()).not.toBe(before);
    expect(translation()).toContain('40, 30');
    expect(screen.getByTestId('explorer-panel')).toHaveTextContent('Orders');

    // A press that does not move is a click, which clears the selection.
    fireEvent.pointerDown(canvas, {button: 0, pointerId: 1, clientX: 100, clientY: 100});
    fireEvent.pointerUp(canvas, {pointerId: 1, clientX: 101, clientY: 100});
    fireEvent.click(canvas);
    expect(screen.getByTestId('explorer-panel')).not.toHaveTextContent('Orders');

    // The wheel pans; with the control key it zooms. A press on a button starts no drag.
    const panned = translation();
    fireEvent.wheel(canvas, {deltaX: 10, deltaY: 20});
    expect(translation()).not.toBe(panned);
    fireEvent.wheel(canvas, {deltaY: -100, ctrlKey: true});
    expect(screen.getByTestId('explorer-zoom')).not.toHaveTextContent(initial ?? '');
    const zoomIn = screen.getByRole('button', {name: 'Zoom in'});
    const held = translation();
    fireEvent.pointerDown(zoomIn, {button: 0, pointerId: 2, clientX: 10, clientY: 10});
    fireEvent.pointerMove(canvas, {pointerId: 2, clientX: 60, clientY: 60});
    fireEvent.pointerUp(canvas, {pointerId: 2, clientX: 60, clientY: 60});
    expect(translation()).toBe(held);

    // Fit to width puts the drawing back where it started.
    await userEvent.click(screen.getByRole('button', {name: 'Fit to width'}));
    expect(translation()).toBe(before);
  });

  it('fills the screen with the canvas and the panel together', async () => {
    const requested: HTMLElement[] = [];
    const requestFullscreen = vi.spyOn(HTMLElement.prototype, 'requestFullscreen').mockImplementation(function (
      this: HTMLElement,
    ) {
      requested.push(this);
      return Promise.resolve();
    });
    render(<GatewayExplorerPage />);
    await screen.findByTestId('explorer-node-application/app-1');

    await userEvent.click(screen.getByRole('button', {name: 'Full screen'}));

    expect(requested).toHaveLength(1);
    const stage = screen.getByTestId('explorer-stage');
    expect(requested[0]).toBe(stage);
    expect(within(stage).getByTestId('explorer-canvas')).toBeInTheDocument();
    expect(within(stage).getByTestId('explorer-panel')).toBeInTheDocument();
    requestFullscreen.mockRestore();
  });

  it('maps what the gateway runs into lanes with lines between them, and sums it up', async () => {
    render(<GatewayExplorerPage />);

    expect(await screen.findByTestId('explorer-node-application/app-1')).toBeInTheDocument();
    expect(screen.getByTestId('explorer-lane-applications')).toHaveTextContent('Applications');
    expect(screen.getByTestId('explorer-node-application/app-1')).toHaveTextContent('Orders');
    expect(screen.getByTestId('explorer-node-application/app-1')).toHaveTextContent('SPA');
    expect(screen.getByTestId('explorer-node-connection/c-1')).toHaveTextContent('G');
    // Four relationships: user type in OU, app allows type, app signs in with flow, flow step to Google.
    expect(screen.getAllByTestId(/^explorer-edge-/)).toHaveLength(4);
    const panel = screen.getByTestId('explorer-panel');
    expect(within(panel).getByText('Dev')).toBeInTheDocument();
    expect(within(panel).getByText('v1')).toBeInTheDocument();
    expect(await within(panel).findByText('2 variables, 1 secrets')).toBeInTheDocument();
    expect(within(panel).getByText('1 · 2 values')).toBeInTheDocument();
    expect(within(panel).getAllByText('1 · shared').length).toBeGreaterThanOrEqual(3);
  });

  it('names a relationship when its line is hovered', async () => {
    render(<GatewayExplorerPage />);
    await screen.findByTestId('explorer-node-application/app-1');

    fireEvent.mouseEnter(screen.getByTestId('explorer-edge-2'), {clientX: 300, clientY: 400});
    const tip = await screen.findByRole('tooltip');
    expect(tip).toHaveTextContent('Orders → Sign in');
    expect(tip).toHaveTextContent('Signs users in with Sign in');
  });

  it('shows the selected resource"s values and saves what changed to the gateway', async () => {
    render(<GatewayExplorerPage />);
    await userEvent.click(await screen.findByTestId('explorer-node-application/app-1'));

    const panel = screen.getByTestId('explorer-panel');
    expect(within(panel).getByRole('heading', {name: 'Orders'})).toBeInTheDocument();
    // What can be set here is set apart from what is only read.
    const editable = within(panel).getByTestId('explorer-editable');
    expect(within(editable).getByText('Values for Dev')).toBeInTheDocument();
    expect(within(editable).getByText('Editable. Changes apply to Dev only.')).toBeInTheDocument();
    expect(within(editable).getByLabelText('APP_ORDERS_CLIENT_ID')).toBeInTheDocument();
    expect(within(editable).queryByTestId('explorer-endpoints')).not.toBeInTheDocument();
    // Where Orders signs users in and gets tokens on Dev, each one copyable.
    const endpoints = within(panel).getByTestId('explorer-endpoints');
    expect(within(endpoints).getByText('https://localhost:8090/oauth2/token')).toBeInTheDocument();
    expect(within(endpoints).getByText('https://localhost:8090/oauth2/authorize')).toBeInTheDocument();
    expect(within(endpoints).getAllByRole('button', {name: 'Copy the URL'}).length).toBeGreaterThan(0);
    expect(within(panel).getByLabelText('APP_ORDERS_CLIENT_ID')).toHaveValue('orders');
    const list = within(panel).getByLabelText('APP_ORDERS_REDIRECT_URIS');
    expect(list).toHaveValue('https://a.test/cb\nhttps://a.test/alt');
    expect(within(panel).getByText('Relationships · 2')).toBeInTheDocument();
    expect(within(panel).getByRole('button', {name: 'Save changes'})).toBeDisabled();
    // Everything but the selection fades.
    await waitFor(() =>
      expect(screen.getByTestId('explorer-node-organization_unit/root')).toHaveStyle({opacity: '0.2'}),
    );
    expect(screen.getByTestId('explorer-node-flow/f-1')).toHaveStyle({opacity: '1'});

    await userEvent.clear(list);
    await userEvent.type(list, 'https://b.test/cb');
    expect(within(panel).getByText('Edited')).toBeInTheDocument();
    await userEvent.click(within(panel).getByRole('button', {name: 'Save changes'}));

    await waitFor(() => expect(within(panel).queryByText('Edited')).not.toBeInTheDocument());
    const writes = http.calls.filter((call: Call) => call.method === 'PUT');
    expect(writes.map((call: Call) => [call.url.replace(SERVER, ''), call.data])).toEqual([
      ['/gateways/gw-dev/variables/APP_ORDERS_REDIRECT_URIS', {value: '["https://b.test/cb"]', description: undefined}],
    ]);
    expect(await screen.findByText('Saved 1 values to Dev.')).toBeInTheDocument();
  });

  it('keeps what saved when a later value fails, and sends only what failed again', async () => {
    routes['PUT /gateways/gw-dev/variables/APP_ORDERS_CLIENT_ID'] = {name: 'APP_ORDERS_CLIENT_ID'};
    routes['PUT /gateways/gw-dev/variables/APP_ORDERS_REDIRECT_URIS'] = new Error('down');
    render(<GatewayExplorerPage />);
    await userEvent.click(await screen.findByTestId('explorer-node-application/app-1'));
    const panel = screen.getByTestId('explorer-panel');

    const id = within(panel).getByLabelText('APP_ORDERS_CLIENT_ID');
    await userEvent.clear(id);
    await userEvent.type(id, 'orders-2');
    const list = within(panel).getByLabelText('APP_ORDERS_REDIRECT_URIS');
    await userEvent.clear(list);
    await userEvent.type(list, 'https://b.test/cb');
    await userEvent.click(within(panel).getByRole('button', {name: 'Save changes'}));

    expect(await screen.findByText('Saved 1 of 2 values to Dev. The values could not be saved.')).toBeInTheDocument();
    expect(within(panel).getByLabelText('APP_ORDERS_REDIRECT_URIS')).toHaveValue('https://b.test/cb');

    await userEvent.click(within(panel).getByRole('button', {name: 'Save changes'}));
    await waitFor(() =>
      expect(
        http.calls.filter((call: Call) => call.method === 'PUT').map((call: Call) => call.url.replace(SERVER, '')),
      ).toEqual([
        '/gateways/gw-dev/variables/APP_ORDERS_CLIENT_ID',
        '/gateways/gw-dev/variables/APP_ORDERS_REDIRECT_URIS',
        '/gateways/gw-dev/variables/APP_ORDERS_REDIRECT_URIS',
      ]),
    );
  });

  it('replaces a secret without ever showing it, and follows a relationship to another resource', async () => {
    render(<GatewayExplorerPage />);
    await userEvent.click(await screen.findByTestId('explorer-node-connection/c-1'));

    const panel = screen.getByTestId('explorer-panel');
    expect(within(panel).getByText('Stored')).toBeInTheDocument();
    const secret = within(panel).getByLabelText('CONNECTION_GOOGLE_CLIENT_SECRET');
    expect(secret).toHaveAttribute('type', 'password');
    expect(secret).toHaveValue('');
    await userEvent.type(secret, 'new-secret');
    await userEvent.click(within(panel).getByRole('button', {name: 'Show'}));
    expect(secret).toHaveAttribute('type', 'text');
    await userEvent.click(within(panel).getByRole('button', {name: 'Save changes'}));
    expect(await within(panel).findByText('Updated, not captured')).toBeInTheDocument();
    expect(http.calls.filter((call: Call) => call.method === 'PUT').map((call: Call) => call.data)).toEqual([
      {value: 'new-secret', description: undefined},
    ]);

    await userEvent.click(within(panel).getByRole('button', {name: /Sign in/}));
    expect(within(panel).getByRole('heading', {name: 'Sign in'})).toBeInTheDocument();
    expect(
      within(panel).getByText(
        'This resource has no gateway-specific values. Its configuration is the same on every gateway and changes through a captured version.',
      ),
    ).toBeInTheDocument();
  });

  it('switches to another gateway and goes back to Deploy', async () => {
    render(<GatewayExplorerPage />);
    await screen.findByTestId('explorer-node-application/app-1');

    await userEvent.click(screen.getByRole('button', {name: 'Prod'}));
    expect(navigate).toHaveBeenCalledWith('/deploy/gateways/gw-prod/explore');
    await userEvent.click(screen.getByText('Deploy'));
    expect(navigate).toHaveBeenCalledWith('/deploy');
  });

  it('says when nothing is deployed, or the gateway is unknown', async () => {
    routes['GET /gateways/gw-dev/applied-configuration'] = {gatewayId: 'gw-dev', resources: []};
    const {unmount} = render(<GatewayExplorerPage />);
    expect(await screen.findByText('Nothing has been deployed to Dev yet.')).toBeInTheDocument();
    unmount();

    gatewayId = 'gw-gone';
    render(<GatewayExplorerPage />);
    expect(await screen.findByText('No gateway is registered under this id.')).toBeInTheDocument();
  });

  it('says when the values the gateway refers to could not be read, and keeps the map', async () => {
    routes[`GET /configuration-versions/${hash}`] = new Error('down');
    render(<GatewayExplorerPage />);

    expect(
      await screen.findByText('Which values Dev refers to could not be read, so none are shown.'),
    ).toBeInTheDocument();
    expect(screen.getByTestId('explorer-node-application/app-1')).toBeInTheDocument();
  });
});
