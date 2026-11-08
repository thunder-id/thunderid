// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import templates from '../templates.json';

interface Node {
  id: string;
  type: string;
  next?: string;
  onSuccess?: string;
  onFailure?: string;
  onIncomplete?: string;
  condition?: unknown;
  properties?: Record<string, unknown>;
  executor?: {name: string; mode?: string};
  prompts?: {action?: {ref?: string; nextNode?: string}}[];
}

interface Template {
  type?: string;
  flowType?: string;
  configureBeforeSave?: boolean;
  config?: {nodes?: Node[]};
}

const find = (type: string, flowType: string): Template | undefined =>
  (templates as Template[]).find((t) => t.type === type && t.flowType === flowType);

const nodesOf = (template: Template | undefined): Map<string, Node> =>
  new Map((template?.config?.nodes ?? []).map((n) => [n.id, n]));

const targetsOf = (n: Node): string[] =>
  [
    n.next,
    n.onSuccess,
    n.onFailure,
    n.onIncomplete,
    ...(n.prompts ?? []).map((prompt) => prompt.action?.nextNode),
  ].filter((target): target is string => Boolean(target));

/** The executor nodes on the path the flow takes when every task succeeds, from start to end. */
const successPath = (nodes: Map<string, Node>): string[] => {
  const path: string[] = [];
  let current = nodes.get('start');
  while (current && current.type !== 'END' && path.length <= nodes.size) {
    if (current.type === 'TASK_EXECUTION') path.push(current.id);
    const next: string | undefined =
      current.type === 'PROMPT' ? current.prompts?.[0]?.action?.nextNode : (current.onSuccess ?? current.next);
    current = next ? nodes.get(next) : undefined;
  }
  return path;
};

const signUps = [
  {type: 'ORGANIZATION_SIGNUP', flowType: 'REGISTRATION'},
  {type: 'ORGANIZATION_SIGNUP_SHARED_ACCOUNT', flowType: 'REGISTRATION'},
];
const all = [...signUps, {type: 'BASIC_ENTERPRISE_SSO', flowType: 'AUTHENTICATION'}];

describe('organization templates', () => {
  it.each(all)('$type should exist and open unsaved, since it names values only the deployment has', (key) => {
    const template = find(key.type, key.flowType);

    expect(template).toBeDefined();
    expect(template?.configureBeforeSave).toBe(true);
  });

  // The designer neither draws nor saves node conditions, so the templates branch only on edges it shows.
  it.each(all)('$type should branch only on edges the designer draws, all of which lead somewhere', (key) => {
    const nodes = nodesOf(find(key.type, key.flowType));
    const seen = new Set<string>();
    const queue = ['start'];
    while (queue.length > 0) {
      const id = queue.shift() ?? '';
      if (!seen.has(id)) {
        seen.add(id);
        const n = nodes.get(id);

        expect(n, id).toBeDefined();
        expect(n?.condition, id).toBeUndefined();
        queue.push(...(n ? targetsOf(n) : []));
      }
    }

    expect([...seen].sort()).toEqual([...nodes.keys()].sort());
  });

  it.each(signUps)('$type should create the organization before the user, and assign roles last', (key) => {
    const nodes = nodesOf(find(key.type, key.flowType));
    const path = successPath(nodes);

    expect(path.indexOf('create_organization')).toBeLessThan(path.indexOf('provisioning'));
    expect(path[path.length - 1]).toBe('assign_roles');
    expect(nodes.get('create_organization')?.executor?.name).toBe('OUExecutor');
    // The handle is optional: left empty, it is made from the organization's name.
    expect(nodes.get('create_organization')?.properties).toEqual({generateOuHandle: true});
    expect(nodes.get('assign_roles')?.executor).toEqual({name: 'RoleExecutor', mode: 'role_assignment'});
    // The roles are the deployment's; the designer names them before the flow is saved.
    expect(nodes.get('assign_roles')?.properties).toEqual({roles: ''});
  });

  it('should provision the user straight into the new organization', () => {
    const nodes = nodesOf(find('ORGANIZATION_SIGNUP', 'REGISTRATION'));

    expect(nodes.get('provisioning')?.properties).toBeUndefined();
    expect(successPath(nodes)).not.toContain('share_user');
  });

  it('should create the user in its user type unit and share it into the new organization before assigning', () => {
    const nodes = nodesOf(find('ORGANIZATION_SIGNUP_SHARED_ACCOUNT', 'REGISTRATION'));

    expect(nodes.get('provisioning')?.properties).toEqual({provisioningOU: 'userType'});
    expect(nodes.get('provisioning')?.onSuccess).toBe('share_user');
    expect(nodes.get('share_user')?.executor).toEqual({name: 'SharingExecutor', mode: 'share'});
    expect(nodes.get('share_user')?.properties).toEqual({
      resourceType: 'user',
      resourceId: '{{ctx(userID)}}',
      targetOuId: '{{ctx(ouId)}}',
    });
    expect(nodes.get('share_user')?.onSuccess).toBe('assign_roles');
  });

  it('should decide the organization before enterprise SSO and after a password sign-in', () => {
    const nodes = nodesOf(find('BASIC_ENTERPRISE_SSO', 'AUTHENTICATION'));
    const resolver = {resolveFrom: 'prompt', parentOuId: '', directChildrenOnly: true};

    expect(nodes.get('resolve_organization')?.properties).toEqual(resolver);
    expect(nodes.get('resolve_organization')?.onSuccess).toBe('organization_settings');
    expect(nodes.get('organization_settings')?.type).toBe('VARIABLES');
    expect(nodes.get('enterprise_auth')?.properties).toEqual({
      idpId: '{{var(enterpriseIdp)}}',
      allowAuthenticationWithoutLocalUser: true,
    });
    expect(nodes.get('credentials_auth')?.onSuccess).toBe('resolve_user_organization');
    expect(nodes.get('resolve_user_organization')?.properties).toEqual(resolver);
    expect(nodes.get('resolve_user_organization')?.onSuccess).toBe('authorization_check');
  });
});
