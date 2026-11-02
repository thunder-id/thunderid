// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AppliedResource, ValueReference} from '../models/deploy';

type Fields = Record<string, unknown>;

const isObject = (value: unknown): value is Fields =>
  value !== null && typeof value === 'object' && !Array.isArray(value);
const text = (value: unknown): string | undefined => (typeof value === 'string' && value !== '' ? value : undefined);
const list = (value: unknown): unknown[] => (Array.isArray(value) ? (value as unknown[]) : []);

/** The lanes of the explorer, top to bottom. */
export type LaneId =
  | 'organizationUnits'
  | 'userTypes'
  | 'identities'
  | 'roles'
  | 'agents'
  | 'applications'
  | 'resourceServers'
  | 'flows'
  | 'connections'
  | 'design';

/** How a lane's tiles are drawn. */
export type TileShape = 'tile' | 'pill' | 'circle';

export interface Lane {
  id: LaneId;
  /** The lane's title as an i18n key and its default. */
  title: [key: string, defaultValue: string];
  /** The resource types shown in the lane, as the applied configuration names them. */
  resourceTypes: string[];
  color: string;
  /** The icon drawn in a tile, by the name the components map to an icon. */
  icon: string;
  shape: TileShape;
}

export const LANES: readonly Lane[] = [
  {
    id: 'organizationUnits',
    title: ['deploy:explorer.lanes.organizationUnits', 'Organization units'],
    resourceTypes: ['organization_unit'],
    color: '#3688FF',
    icon: 'building',
    shape: 'tile',
  },
  {
    id: 'userTypes',
    title: ['deploy:explorer.lanes.userTypes', 'User types'],
    resourceTypes: ['user_type', 'agent_type'],
    color: '#38BDF8',
    icon: 'userCog',
    shape: 'tile',
  },
  {
    id: 'identities',
    title: ['deploy:explorer.lanes.identities', 'Users and groups'],
    resourceTypes: ['group', 'user'],
    color: '#A78BFA',
    icon: 'users',
    shape: 'tile',
  },
  {
    id: 'roles',
    title: ['deploy:explorer.lanes.roles', 'Roles'],
    resourceTypes: ['role'],
    color: '#F472B6',
    icon: 'shield',
    shape: 'tile',
  },
  {
    id: 'agents',
    title: ['deploy:explorer.lanes.agents', 'Agents'],
    resourceTypes: ['agent'],
    color: '#2DD4BF',
    icon: 'bot',
    shape: 'tile',
  },
  {
    id: 'applications',
    title: ['deploy:explorer.lanes.applications', 'Applications'],
    resourceTypes: ['application'],
    color: '#34D399',
    icon: 'app',
    shape: 'tile',
  },
  {
    id: 'resourceServers',
    title: ['deploy:explorer.lanes.resourceServers', 'Resource servers'],
    resourceTypes: ['resource_server'],
    color: '#FBBF24',
    icon: 'cloud',
    shape: 'tile',
  },
  {
    id: 'flows',
    title: ['deploy:explorer.lanes.flows', 'Flows'],
    resourceTypes: ['flow'],
    color: '#22D3EE',
    icon: 'flow',
    shape: 'pill',
  },
  {
    id: 'connections',
    title: ['deploy:explorer.lanes.connections', 'Connections'],
    resourceTypes: ['connection'],
    color: '#FB923C',
    icon: 'layers',
    shape: 'circle',
  },
  {
    id: 'design',
    title: ['deploy:explorer.lanes.design', 'Design'],
    resourceTypes: ['theme', 'layout'],
    color: '#C084FC',
    icon: 'palette',
    shape: 'tile',
  },
];

export const LANE_BY_ID: Readonly<Record<LaneId, Lane>> = Object.fromEntries(
  LANES.map((lane) => [lane.id, lane]),
) as Record<LaneId, Lane>;

const laneOfType = (resourceType: string): Lane | undefined =>
  LANES.find((lane: Lane) => lane.resourceTypes.includes(resourceType));

/** The geometry of the stage, from the design: lanes of a fixed height with tiles at a fixed step. */
export const STAGE = {
  width: 980,
  laneHeight: 138,
  laneGap: 12,
  top: 20,
  firstColumn: 100,
  columnStep: 180,
  /** How far below a lane's top a tile's centre line sits. */
  tileOffset: 74,
  tileSize: 56,
} as const;

/** How many of a lane's resources are laid out; the rest are counted on a last tile. */
export const LANE_CAPACITY = 50;

export const laneTop = (index: number): number => STAGE.top + index * (STAGE.laneHeight + STAGE.laneGap);

/** A translatable label: an i18n key, its default, and the values it interpolates. */
export interface Label {
  key: string;
  defaultValue: string;
  values?: Record<string, string>;
}

/** One resource on the map. */
export interface GraphNode {
  /** `resourceType/id`, the key the control plane's changes use. */
  key: string;
  id: string;
  resourceType: string;
  lane: LaneId;
  name: string;
  /** What kind of thing it is, shown as the panel's eyebrow. */
  kind: Label;
  /** A short tag on an application's tile: its type. */
  badge?: string;
  /** A letter drawn in place of the lane's icon, for a connection whose vendor is known by one. */
  letter?: string;
  /** The icon, when it is not the lane's. */
  icon?: string;
  /** A connection that signs users in through another provider, which has to be told where to return. */
  federated?: boolean;
  /** The values the gateway holds for this resource, as the version refers to them. */
  values: ValueReference[];
  /** The tile's position on the stage. */
  x: number;
  y: number;
}

/** A tile that stands for the resources a lane could not show. */
export interface OverflowNode {
  lane: LaneId;
  count: number;
  x: number;
  y: number;
}

/** A line between two resources, with what it means. */
export interface GraphEdge {
  from: string;
  to: string;
  label: Label;
}

export interface LaneSummary {
  lane: Lane;
  count: number;
  /** How many values its resources refer to, variables and secrets together. */
  values: number;
}

export interface Graph {
  nodes: GraphNode[];
  overflow: OverflowNode[];
  edges: GraphEdge[];
  lanes: LaneSummary[];
  /** References naming no resource on the map, grouped by the type they name. */
  unplaced: ValueReference[];
  width: number;
  height: number;
}

const NAME_FIELDS: readonly string[] = ['name', 'displayName', 'handle', 'language', 'title'];

function nameOf(fields: Fields, resourceType: string, id: string): string {
  if (resourceType === 'user') {
    const attributes = isObject(fields['attributes']) ? fields['attributes'] : {};
    return text(attributes['username']) ?? text(attributes['email']) ?? text(attributes['name']) ?? id;
  }
  if (resourceType === 'agent') {
    const attributes = isObject(fields['attributes']) ? fields['attributes'] : {};
    return text(attributes['name']) ?? text(fields['name']) ?? id;
  }
  for (const field of NAME_FIELDS) {
    const name = text(fields[field]);
    if (name) return name;
  }
  return id;
}

const KIND_LABELS: Readonly<Record<string, string>> = {
  organization_unit: 'Organization unit',
  user_type: 'User type',
  agent_type: 'Agent type',
  group: 'Group',
  user: 'User',
  role: 'Role',
  agent: 'Agent',
  application: 'Application',
  resource_server: 'Resource server',
  flow: 'Flow',
  connection: 'Connection',
  theme: 'Theme',
  layout: 'Layout',
};

const kindOf = (resourceType: string): Label => ({
  key: `deploy:explorer.kinds.${resourceType}`,
  defaultValue: KIND_LABELS[resourceType] ?? resourceType,
});

/** An application's badge: its type as the design abbreviates it. */
function badgeOf(fields: Fields): string | undefined {
  const type = text(fields['type'])?.toLowerCase();
  const template = text(fields['template'])?.toLowerCase();
  if (template === 'system' || text(fields['name']) === 'Console') return 'SYSTEM';
  switch (type) {
    case 'browser':
      return 'SPA';
    case 'native':
    case 'mobile':
      return 'NATIVE';
    case 'm2m':
      return 'M2M';
    case 'web':
    case 'traditional':
      return 'WEB';
    default:
      return type ? type.toUpperCase() : undefined;
  }
}

/** The connection types that send messages rather than sign users in through another provider. */
const SENDER_TYPES: readonly string[] = ['twilio', 'vonage', 'sms-gateway', 'email-smtp'];

/** A connection's letter or icon, by the vendor its type names, and whether it signs users in. */
function connectionLook(fields: Fields): {letter?: string; icon?: string; federated: boolean} {
  const type = text(fields['type']) ?? '';
  return {...connectionIcon(type), federated: type !== '' && !SENDER_TYPES.includes(type)};
}

function connectionIcon(type: string): {letter?: string; icon?: string} {
  switch (type) {
    case 'google':
      return {letter: 'G'};
    case 'github':
      return {icon: 'github'};
    case 'twilio':
    case 'vonage':
    case 'sms-gateway':
      return {icon: 'phone'};
    case 'email-smtp':
      return {icon: 'mail'};
    case 'oidc':
    case 'oauth':
      return {icon: 'lock'};
    default:
      return {};
  }
}

const key = (resourceType: string, id: string): string => `${resourceType}/${id}`;

/** The flow a field names, by the role the field gives it. */
const FLOW_FIELDS: readonly [field: string, key: string, defaultValue: string][] = [
  ['authFlowId', 'signsIn', 'Signs users in with {{name}}'],
  ['registrationFlowId', 'registers', 'Registers users with {{name}}'],
  ['recoveryFlowId', 'recovers', 'Recovers passwords with {{name}}'],
  ['signOutFlowId', 'signsOut', 'Signs users out with {{name}}'],
];

/** The connection a flow node's executor names, under any of the keys the executors use. */
const CONNECTION_FIELDS: readonly string[] = ['connectionId', 'idpId', 'senderId', 'connection', 'idpName'];

/**
 * Builds the map of what a gateway runs: a node per resource in a lane, a line per relationship the
 * resources' own fields describe, and the values each resource refers to. The rules are the ones the
 * configuration's pages use to link things: an organization unit's parent, where a user type keeps its
 * users, a group's members, a role's assignments and permissions, an application's user types, flows,
 * theme and layout, a flow's connections step by step, and a connection's provisioning user type.
 */
export function buildGraph(resources: readonly AppliedResource[], references: readonly ValueReference[] = []): Graph {
  const byKey = new Map<string, GraphNode>();
  const columns = new Map<LaneId, number>();
  const overflow: OverflowNode[] = [];
  const laneIndex = (lane: LaneId): number => LANES.findIndex((entry: Lane) => entry.id === lane);
  const fieldsOf = new Map<string, Fields>();

  // Resources are placed in the order they arrive, so the map reads as the configuration lists it.
  resources.forEach((applied: AppliedResource) => {
    const lane = laneOfType(applied.resourceType);
    if (!lane) return;
    const fields: Fields = isObject(applied.resource) ? applied.resource : {};
    const id = text(fields['id']) ?? applied.id;
    const column = columns.get(lane.id) ?? 0;
    columns.set(lane.id, column + 1);
    if (column >= LANE_CAPACITY) {
      const last = overflow.find((entry: OverflowNode) => entry.lane === lane.id);
      if (last) {
        last.count += 1;
      } else {
        overflow.push({
          lane: lane.id,
          count: 1,
          x: STAGE.firstColumn + LANE_CAPACITY * STAGE.columnStep,
          y: laneTop(laneIndex(lane.id)) + STAGE.tileOffset,
        });
      }
      return;
    }
    const node: GraphNode = {
      key: key(applied.resourceType, id),
      id,
      resourceType: applied.resourceType,
      lane: lane.id,
      name: nameOf(fields, applied.resourceType, id),
      kind: kindOf(applied.resourceType),
      values: [],
      x: STAGE.firstColumn + column * STAGE.columnStep,
      y: laneTop(laneIndex(lane.id)) + STAGE.tileOffset,
    };
    if (applied.resourceType === 'application') node.badge = badgeOf(fields);
    if (applied.resourceType === 'user') node.icon = 'user';
    if (applied.resourceType === 'connection') Object.assign(node, connectionLook(fields));
    byKey.set(node.key, node);
    fieldsOf.set(node.key, {...fields, ...(applied.parts ?? {})});
  });

  // Secondary indexes for the fields that name a resource by something other than its id.
  const userTypeByHandle = new Map<string, GraphNode>();
  const connectionByName = new Map<string, GraphNode>();
  byKey.forEach((node: GraphNode) => {
    const fields = fieldsOf.get(node.key) ?? {};
    if (node.resourceType === 'user_type' || node.resourceType === 'agent_type') {
      const handle = text(fields['handle']);
      if (handle) userTypeByHandle.set(handle, node);
      userTypeByHandle.set(node.id, node);
    }
    if (node.resourceType === 'connection') {
      connectionByName.set(node.name, node);
      connectionByName.set(node.id, node);
    }
  });
  const find = (resourceType: string, id: unknown): GraphNode | undefined => {
    const name = text(id);
    return name ? byKey.get(key(resourceType, name)) : undefined;
  };
  const findIdentity = (entry: unknown): GraphNode | undefined => {
    if (!isObject(entry)) return undefined;
    const type = text(entry['type']);
    return type ? find(type, entry['id']) : undefined;
  };

  const edges: GraphEdge[] = [];
  const seen = new Set<string>();
  const link = (from: GraphNode, to: GraphNode | undefined, labelKey: string, defaultValue: string): void => {
    if (!to || to === from) return;
    const id = `${from.key}>${to.key}>${labelKey}`;
    if (seen.has(id)) return;
    seen.add(id);
    edges.push({
      from: from.key,
      to: to.key,
      label: {key: `deploy:explorer.edges.${labelKey}`, defaultValue, values: {name: to.name}},
    });
  };

  byKey.forEach((node: GraphNode) => {
    const fields = fieldsOf.get(node.key) ?? {};
    const ou = find('organization_unit', fields['ouId']);
    switch (node.resourceType) {
      case 'organization_unit':
        link(node, find('organization_unit', fields['parent']), 'childUnit', 'Child unit of {{name}}');
        break;
      case 'user_type':
      case 'agent_type':
        link(node, ou, 'usersStoredIn', 'Users stored in {{name}}');
        break;
      case 'group':
        link(node, ou, 'groupIn', 'Group in {{name}}');
        list(fields['members']).forEach((member: unknown) => {
          const other = findIdentity(member);
          if (other) link(other, node, 'memberOf', 'Member of {{name}}');
        });
        break;
      case 'user':
        link(node, userTypeByHandle.get(text(fields['type']) ?? ''), 'userOfType', 'User of type {{name}}');
        link(node, ou, 'userIn', 'User in {{name}}');
        break;
      case 'role':
        link(node, ou, 'roleIn', 'Role in {{name}}');
        list(fields['assignments']).forEach((assignment: unknown) =>
          link(node, findIdentity(assignment), 'assignedTo', 'Assigned to {{name}}'),
        );
        list(fields['permissions']).forEach((permission: unknown) => {
          if (!isObject(permission)) return;
          const server = find('resource_server', permission['resourceServerId']);
          const scopes = list(permission['permissions']).filter(
            (scope: unknown): scope is string => typeof scope === 'string',
          );
          if (!server) return;
          const id = `${node.key}>${server.key}>grants`;
          if (seen.has(id)) return;
          seen.add(id);
          edges.push({
            from: node.key,
            to: server.key,
            label: {
              key: 'deploy:explorer.edges.grants',
              defaultValue: 'Grants {{scopes}}',
              values: {name: server.name, scopes: scopes.join(', ')},
            },
          });
        });
        break;
      case 'agent':
        link(node, ou, 'agentIn', 'Agent in {{name}}');
        list(fields['allowedUserTypes']).forEach((handle: unknown) =>
          link(node, userTypeByHandle.get(text(handle) ?? ''), 'allowsUserType', 'Allows user type {{name}}'),
        );
        break;
      case 'application':
        link(node, ou, 'applicationIn', 'Application in {{name}}');
        list(fields['allowedUserTypes']).forEach((handle: unknown) =>
          link(node, userTypeByHandle.get(text(handle) ?? ''), 'allowsUserType', 'Allows user type {{name}}'),
        );
        list(fields['allowedAgentTypes']).forEach((handle: unknown) =>
          link(node, userTypeByHandle.get(text(handle) ?? ''), 'allowsAgentType', 'Allows agent type {{name}}'),
        );
        FLOW_FIELDS.forEach(([field, labelKey, defaultValue]: [string, string, string]) =>
          link(node, find('flow', fields[field]), labelKey, defaultValue),
        );
        link(node, find('theme', fields['themeId']), 'styledWith', 'Styled with {{name}}');
        link(node, find('layout', fields['layoutId']), 'laidOutWith', 'Laid out with {{name}}');
        list(fields['allowedResourceServers'] ?? fields['resourceServers']).forEach((server: unknown) =>
          link(
            node,
            find('resource_server', isObject(server) ? server['id'] : server),
            'requests',
            'Requests {{name}}',
          ),
        );
        break;
      case 'resource_server':
        link(node, ou, 'resourceServerIn', 'Resource server in {{name}}');
        break;
      case 'flow':
        list(fields['nodes']).forEach((step: unknown, index: number) => {
          if (!isObject(step)) return;
          // A step names its connection in its own properties, as the flow builder writes it; an
          // executor's properties and the step itself are read too.
          const executor = isObject(step['executor']) ? step['executor'] : {};
          const own = isObject(step['properties']) ? step['properties'] : {};
          const properties = isObject(executor['properties']) ? executor['properties'] : {};
          CONNECTION_FIELDS.forEach((field: string) => {
            const named = text(own[field]) ?? text(properties[field]) ?? text(step[field]);
            const connection = named ? (connectionByName.get(named) ?? find('connection', named)) : undefined;
            if (!connection) return;
            const id = `${node.key}>${connection.key}>step`;
            if (seen.has(id)) return;
            seen.add(id);
            edges.push({
              from: node.key,
              to: connection.key,
              label: {
                key: 'deploy:explorer.edges.step',
                defaultValue: 'Step {{step}}: {{name}}',
                values: {name: connection.name, step: String(index + 1)},
              },
            });
          });
        });
        break;
      case 'connection': {
        // A connection provisions users as the user type its resolution falls back to, and as each
        // user type it maps attributes for.
        const provisioning = isObject(fields['attributeConfiguration']) ? fields['attributeConfiguration'] : {};
        const resolution = isObject(provisioning['user_type_resolution']) ? provisioning['user_type_resolution'] : {};
        const handles = [
          text(fields['userType']),
          text(resolution['default']),
          ...list(provisioning['user_type_attribute_mappings']).map((mapping: unknown) =>
            isObject(mapping) ? text(mapping['user_type']) : undefined,
          ),
        ].filter((handle): handle is string => Boolean(handle));
        handles.forEach((handle: string) =>
          link(node, userTypeByHandle.get(handle), 'provisionsAs', 'Provisions users as {{name}}'),
        );
        break;
      }
      default:
        break;
    }
  });

  const unplaced: ValueReference[] = [];
  references.forEach((reference: ValueReference) => {
    const node = reference.resourceId ? byKey.get(key(reference.resourceType, reference.resourceId)) : undefined;
    if (node) {
      node.values.push(reference);
    } else {
      unplaced.push(reference);
    }
  });

  const nodes = [...byKey.values()];
  const lanes: LaneSummary[] = LANES.map((lane: Lane) => {
    const own = nodes.filter((node: GraphNode) => node.lane === lane.id);
    const more = overflow.find((entry: OverflowNode) => entry.lane === lane.id)?.count ?? 0;
    return {
      lane,
      count: own.length + more,
      values: own.reduce((sum: number, node: GraphNode) => sum + node.values.length, 0),
    };
  });
  const widest = Math.max(
    0,
    ...nodes.map((node: GraphNode) => node.x),
    ...overflow.map((entry: OverflowNode) => entry.x),
  );
  return {
    nodes,
    overflow,
    edges,
    lanes,
    unplaced,
    width: Math.max(STAGE.width, widest + STAGE.firstColumn),
    height: STAGE.top + LANES.length * (STAGE.laneHeight + STAGE.laneGap),
  };
}

/** The SVG path of an edge: horizontal between tiles of one lane, otherwise from below the upper tile's label to the top of the lower tile. */
export function edgePath(from: GraphNode, to: GraphNode, curved = false): string {
  const half = STAGE.tileSize / 2;
  if (from.lane === to.lane) {
    const [left, right] = from.x < to.x ? [from, to] : [to, from];
    return `M${left.x + half + 2},${left.y} L${right.x - half - 2},${right.y}`;
  }
  const [upper, lower] = from.y < to.y ? [from, to] : [to, from];
  const y1 = upper.y + STAGE.tileSize;
  const y2 = lower.y - half - 2;
  if (!curved) return `M${upper.x},${y1} L${lower.x},${y2}`;
  const middle = (y1 + y2) / 2;
  return `M${upper.x},${y1} C${upper.x},${middle} ${lower.x},${middle} ${lower.x},${y2}`;
}

/** The nodes an edge touches, and the edges a node touches, for focusing one resource. */
export function neighbours(graph: Graph, nodeKey: string): Set<string> {
  const related = new Set<string>([nodeKey]);
  graph.edges.forEach((edge: GraphEdge) => {
    if (edge.from === nodeKey) related.add(edge.to);
    if (edge.to === nodeKey) related.add(edge.from);
  });
  return related;
}
