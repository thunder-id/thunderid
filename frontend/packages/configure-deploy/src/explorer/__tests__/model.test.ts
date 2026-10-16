// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import type {AppliedResource, ValueReference} from '../../models/deploy';
import {
  buildGraph,
  edgePath,
  LANE_CAPACITY,
  LANES,
  neighbours,
  STAGE,
  type GraphEdge,
  type GraphNode,
  type LaneSummary,
} from '../model';

const resource = (
  resourceType: string,
  id: string,
  fields: Record<string, unknown>,
  parts?: Record<string, unknown>,
): AppliedResource => ({
  resourceType,
  id,
  resource: {id, ...fields},
  parts,
});

const configuration: AppliedResource[] = [
  resource('organization_unit', 'root', {name: 'ThunderID'}),
  resource('organization_unit', 'eng', {name: 'Engineering', parent: 'root'}),
  resource('user_type', 'ut-person', {handle: 'person', displayName: 'Person', ouId: 'root'}),
  resource('user_type', 'ut-employee', {handle: 'employee', displayName: 'Employee', ouId: 'eng'}),
  resource('group', 'admins', {name: 'Administrators', ouId: 'eng'}, {members: [{id: 'alice', type: 'user'}]}),
  resource('user', 'alice', {type: 'employee', ouId: 'eng', attributes: {username: 'alice@example.com'}}),
  resource('role', 'administrator', {
    name: 'Administrator',
    ouId: 'eng',
    assignments: [
      {id: 'admins', type: 'group'},
      {id: 'helper', type: 'agent'},
    ],
    permissions: [{resourceServerId: 'orders', permissions: ['orders:read', 'orders:write']}],
  }),
  resource('agent', 'helper', {ouId: 'eng', attributes: {name: 'Support assistant'}, allowedUserTypes: ['employee']}),
  resource('application', 'console', {
    name: 'Console',
    ouId: 'root',
    type: 'browser',
    template: 'system',
    allowedUserTypes: ['employee'],
    authFlowId: 'signin',
    themeId: 'dark',
    layoutId: 'centered',
  }),
  resource('application', 'orders-worker', {
    name: 'Orders worker',
    type: 'm2m',
    registrationFlowId: 'reg',
    allowedResourceServers: ['orders'],
  }),
  resource('resource_server', 'orders', {
    name: 'Orders API',
    ouId: 'root',
    identifier: 'var:RESOURCE_SERVER_ORDERS_API_IDENTIFIER',
  }),
  resource('flow', 'signin', {
    name: 'Basic sign-in',
    nodes: [
      {id: 'start', type: 'START'},
      {
        id: 'google',
        type: 'TASK_EXECUTION',
        properties: {allowAuthenticationWithoutLocalUser: true, idpId: 'google-1'},
        executor: {name: 'GoogleOIDCAuthExecutor'},
      },
    ],
  }),
  resource('flow', 'reg', {name: 'Self registration', nodes: [{id: 'otp', executor: {properties: {senderId: 'sms'}}}]}),
  resource('connection', 'google-1', {
    name: 'Google',
    type: 'google',
    attributeConfiguration: {
      user_type_resolution: {default: 'person'},
      user_type_attribute_mappings: [
        {user_type: 'person', attributes: []},
        {user_type: 'employee', attributes: []},
      ],
    },
  }),
  resource('connection', 'sms', {name: 'SMS OTP', type: 'twilio'}),
  resource('theme', 'dark', {displayName: 'Dark theme'}),
  resource('layout', 'centered', {displayName: 'Centered'}),
  resource('server_config', 'cors', {merged: {}}),
];

const references: ValueReference[] = [
  {
    name: 'APPLICATION_CONSOLE_CLIENT_ID',
    kind: 'variable',
    resourceType: 'application',
    resourceId: 'console',
    field: 'clientId',
  },
  {
    name: 'APPLICATION_CONSOLE_REDIRECT_URIS',
    kind: 'variable',
    resourceType: 'application',
    resourceId: 'console',
    field: 'redirectUris',
    list: true,
  },
  {
    name: 'CONNECTION_GOOGLE_CLIENT_SECRET',
    kind: 'secret',
    resourceType: 'connection',
    resourceId: 'google-1',
    field: 'clientSecret',
  },
  {name: 'STRAY', kind: 'variable', resourceType: 'application', resourceId: 'gone'},
];

const graph = buildGraph(configuration, references);
const node = (key: string): GraphNode => {
  const found = graph.nodes.find((entry: GraphNode) => entry.key === key);
  if (!found) throw new Error(`no node ${key}`);
  return found;
};
const edgeLabels = (from: string): string[] =>
  graph.edges
    .filter((edge: GraphEdge) => edge.from === from)
    .map((edge: GraphEdge) =>
      Object.entries(edge.label.values ?? {}).reduce(
        (label: string, [name, value]: [string, string]) => label.replace(`{{${name}}}`, value),
        `${edge.to}: ${edge.label.defaultValue}`,
      ),
    );

describe('buildGraph', () => {
  it('places every resource of a known type in its lane, in order, and skips the rest', () => {
    expect(graph.nodes.map((entry: GraphNode) => entry.key)).not.toContain('server_config/cors');
    expect(node('organization_unit/root').lane).toBe('organizationUnits');
    expect(node('group/admins').lane).toBe('identities');
    expect(node('user/alice').lane).toBe('identities');
    expect(node('user/alice').icon).toBe('user');
    expect(node('theme/dark').lane).toBe('design');
    expect(node('layout/centered').lane).toBe('design');
    // The second resource of a lane sits one step to the right of the first.
    expect(node('organization_unit/eng').x - node('organization_unit/root').x).toBe(STAGE.columnStep);
    expect(node('organization_unit/root').y).toBe(STAGE.top + STAGE.tileOffset);
  });

  it('names resources as their pages do and tags applications and connections', () => {
    expect(node('user/alice').name).toBe('alice@example.com');
    expect(node('agent/helper').name).toBe('Support assistant');
    expect(node('user_type/ut-person').name).toBe('Person');
    expect(node('application/console').badge).toBe('SYSTEM');
    expect(node('application/orders-worker').badge).toBe('M2M');
    expect(node('connection/google-1').letter).toBe('G');
    expect(node('connection/sms').icon).toBe('phone');
    expect(node('role/administrator').kind).toEqual({key: 'deploy:explorer.kinds.role', defaultValue: 'Role'});
  });

  it('draws one line per relationship the resources describe', () => {
    expect(edgeLabels('organization_unit/eng')).toEqual(['organization_unit/root: Child unit of ThunderID']);
    expect(edgeLabels('user_type/ut-employee')).toEqual(['organization_unit/eng: Users stored in Engineering']);
    expect(edgeLabels('group/admins')).toEqual(['organization_unit/eng: Group in Engineering']);
    expect(edgeLabels('user/alice')).toEqual([
      'group/admins: Member of Administrators',
      'user_type/ut-employee: User of type Employee',
      'organization_unit/eng: User in Engineering',
    ]);
    expect(edgeLabels('role/administrator')).toEqual([
      'organization_unit/eng: Role in Engineering',
      'group/admins: Assigned to Administrators',
      'agent/helper: Assigned to Support assistant',
      'resource_server/orders: Grants orders:read, orders:write',
    ]);
    expect(edgeLabels('agent/helper')).toEqual([
      'organization_unit/eng: Agent in Engineering',
      'user_type/ut-employee: Allows user type Employee',
    ]);
    expect(edgeLabels('application/console')).toEqual([
      'organization_unit/root: Application in ThunderID',
      'user_type/ut-employee: Allows user type Employee',
      'flow/signin: Signs users in with Basic sign-in',
      'theme/dark: Styled with Dark theme',
      'layout/centered: Laid out with Centered',
    ]);
    expect(edgeLabels('application/orders-worker')).toEqual([
      'flow/reg: Registers users with Self registration',
      'resource_server/orders: Requests Orders API',
    ]);
    expect(edgeLabels('resource_server/orders')).toEqual(['organization_unit/root: Resource server in ThunderID']);
    expect(edgeLabels('flow/signin')).toEqual(['connection/google-1: Step 2: Google']);
    expect(edgeLabels('flow/reg')).toEqual(['connection/sms: Step 1: SMS OTP']);
    expect(edgeLabels('connection/google-1')).toEqual([
      'user_type/ut-person: Provisions users as Person',
      'user_type/ut-employee: Provisions users as Employee',
    ]);
  });

  it('gives each resource the values the version refers to for it, and keeps the rest apart', () => {
    expect(node('application/console').values.map((value: ValueReference) => value.name)).toEqual([
      'APPLICATION_CONSOLE_CLIENT_ID',
      'APPLICATION_CONSOLE_REDIRECT_URIS',
    ]);
    expect(node('connection/google-1').values.map((value: ValueReference) => value.kind)).toEqual(['secret']);
    expect(graph.unplaced.map((value: ValueReference) => value.name)).toEqual(['STRAY']);
  });

  it('summarises each lane with its resources and values', () => {
    const summary = Object.fromEntries(
      graph.lanes.map((entry: LaneSummary): [string, [number, number]] => [entry.lane.id, [entry.count, entry.values]]),
    ) as Record<string, [number, number]>;
    expect(summary.applications).toEqual([2, 2]);
    expect(summary.connections).toEqual([2, 1]);
    expect(summary.identities).toEqual([2, 0]);
    expect(graph.lanes.map((entry) => entry.lane.id)).toEqual(LANES.map((lane) => lane.id));
  });

  it('counts the resources beyond a lane"s capacity on one tile and widens the stage', () => {
    const many = Array.from({length: LANE_CAPACITY + 3}, (_, index: number) =>
      resource('user', `u-${index}`, {type: 'person'}),
    );
    const wide = buildGraph(many);
    expect(wide.nodes).toHaveLength(LANE_CAPACITY);
    expect(wide.overflow).toHaveLength(1);
    expect(wide.overflow[0]).toMatchObject({
      lane: 'identities',
      count: 3,
      x: STAGE.firstColumn + LANE_CAPACITY * STAGE.columnStep,
    });
    expect(wide.width).toBeGreaterThan(STAGE.width);
    expect(buildGraph([]).width).toBe(STAGE.width);
  });
});

describe('edgePath and neighbours', () => {
  it('runs horizontally within a lane and down between lanes, straight or curved', () => {
    const root = node('organization_unit/root');
    const eng = node('organization_unit/eng');
    const employee = node('user_type/ut-employee');
    expect(edgePath(eng, root)).toBe(`M${root.x + 30},${root.y} L${eng.x - 30},${eng.y}`);
    expect(edgePath(employee, eng)).toBe(`M${eng.x},${eng.y + 56} L${employee.x},${employee.y - 30}`);
    expect(edgePath(employee, eng, true)).toMatch(/^M\d+,\d+ C/);
  });

  it('relates a resource to everything a line joins it with', () => {
    expect([...neighbours(graph, 'group/admins')].sort()).toEqual([
      'group/admins',
      'organization_unit/eng',
      'role/administrator',
      'user/alice',
    ]);
  });
});
