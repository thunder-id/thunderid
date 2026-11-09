// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import executors from '../executors.json';
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
  config?: {nodes?: Node[]};
}

const WRITING_EXECUTORS = ['ProvisioningExecutor', 'SharingExecutor', 'RoleExecutor'];

const template = (templates as Template[]).find((t) => t.flowType === 'INVITATION' && t.type === 'DEFAULT');
const nodes = new Map((template?.config?.nodes ?? []).map((node) => [node.id, node]));

const node = (id: string): Node => {
  const found = nodes.get(id);

  expect(found, `${id} should exist`).toBeDefined();

  return found ?? {id, type: ''};
};

const targetsOf = (n: Node): string[] =>
  [
    n.next,
    n.onSuccess,
    n.onFailure,
    n.onIncomplete,
    ...(n.prompts ?? []).map((prompt) => prompt.action?.nextNode),
  ].filter((target): target is string => Boolean(target));

/** The nodes reachable from start without passing through the nodes in avoid. */
const reachableFrom = (start: string, avoid: string[] = []): Set<string> => {
  const seen = new Set<string>();
  const queue = [start];
  while (queue.length > 0) {
    const current = node(queue.shift() ?? '');
    if (!seen.has(current.id) && !avoid.includes(current.id)) {
      seen.add(current.id);
      queue.push(...targetsOf(current));
    }
  }
  return seen;
};

const actionTarget = (promptID: string, ref: string): string | undefined =>
  node(promptID).prompts?.find((prompt) => prompt.action?.ref === ref)?.action?.nextNode;

describe('invitation template', () => {
  it('should exist as the default template of its flow type', () => {
    expect(template).toBeDefined();
  });

  // The designer neither draws nor saves node conditions, so the template branches only on edges it shows.
  it('should branch only on edges the designer draws', () => {
    nodes.forEach((n) => expect(n.condition, n.id).toBeUndefined());
    nodes.forEach((n) => {
      targetsOf(n).forEach((target) => expect(nodes.has(target), `${n.id} -> ${target}`).toBe(true));
    });
    expect([...reachableFrom('start')].sort()).toEqual([...nodes.keys()].sort());
  });

  it('should check the starter holds the permission it asks for first', () => {
    expect(node('start').onSuccess).toBe('permission_validator');
    expect(node('permission_validator').executor).toEqual({name: 'PermissionValidator'});
    expect(node('permission_validator').properties).toEqual({requiredScopes: ['system', 'org']});
    expect(node('permission_validator').onSuccess).toBe('resolve_user_type');
  });

  it('should name its user type before asking for the email', () => {
    expect(node('resolve_user_type').executor).toEqual({name: 'UserTypeResolver'});
    expect(node('resolve_user_type').properties).toHaveProperty('allowedUserTypes');
    expect(node('resolve_user_type').onSuccess).toBe('prompt_email');
  });

  // The flow validator sends a task's onIncomplete only to a prompt, which here asks the admin to confirm.
  it('should branch on whether the email names a user', () => {
    expect(node('identify_user').executor).toMatchObject({name: 'IdentifyingExecutor', mode: 'identify'});
    expect(node('identify_user').properties).toEqual({boundToUserType: true, continueOnNoMatch: true});
    expect(node('identify_user').onSuccess).toBe('validate_existing');
    expect(node('identify_user').onIncomplete).toBe('prompt_new_user');
    expect(node('identify_user').onFailure).toBe('prompt_email');
    expect(node('prompt_new_user').type).toBe('PROMPT');
    expect(actionTarget('prompt_new_user', 'action_invite_new')).toBe('validate_new');
    expect(actionTarget('prompt_new_user', 'action_change_email')).toBe('prompt_email');
  });

  it('should authorize the share into the token unit on both branches', () => {
    expect(node('validate_existing').properties).toEqual({
      resourceType: 'user',
      resourceId: '{{ctx(userID)}}',
      targetOuId: '{{ctx(ouId)}}',
    });
    expect(node('validate_new').properties).toEqual({
      resourceType: 'user',
      owningOuId: '{{ctx(defaultOUID)}}',
      targetOuId: '{{ctx(ouId)}}',
    });
    for (const id of ['validate_existing', 'validate_new']) {
      expect(node(id).executor).toEqual({name: 'SharingExecutor', mode: 'validate'});
      expect(node(id).onFailure).toBe('prompt_email');
    }
  });

  it('should create a user only on the new-user branch, on acceptance', () => {
    expect(actionTarget('prompt_accept_existing', 'action_accept_existing')).toBe('share_user');
    expect(actionTarget('prompt_accept_new', 'action_accept_new')).toBe('create_user');
    expect(node('create_user').properties).toMatchObject({provisioningOU: 'userType'});
    expect(node('create_user').onSuccess).toBe('share_user');

    // A refused share goes back to the email prompt, where the admin starts over.
    const existing = reachableFrom('validate_existing', ['share_user', 'prompt_email']);
    existing.forEach((id) => expect(node(id).executor?.name, id).not.toBe('ProvisioningExecutor'));
    expect(reachableFrom('validate_new', ['create_user', 'prompt_email']).has('share_user')).toBe(false);
  });

  it('should send its own invitation email on both branches', () => {
    for (const branch of ['existing', 'new']) {
      expect(node(`send_invite_email_${branch}`).properties).toMatchObject({emailTemplate: 'invitation'});
    }
  });

  it('should share, then assign the roles as the admin', () => {
    expect(node('share_user').executor).toEqual({name: 'SharingExecutor', mode: 'share'});
    expect(node('share_user').onSuccess).toBe('assign_roles');
    expect(node('assign_roles').executor).toEqual({name: 'RoleExecutor', mode: 'role_assignment'});
  });

  it('should offer every executor and mode it uses on the resource panel', () => {
    const offered = new Set(
      (executors as {display?: {showOnResourcePanel?: boolean}; data?: {action?: {executor?: Node['executor']}}}[])
        .filter((entry) => entry.display?.showOnResourcePanel !== false)
        .map((entry) => `${entry.data?.action?.executor?.name}/${entry.data?.action?.executor?.mode ?? ''}`),
    );
    for (const id of ['permission_validator', 'validate_existing', 'select_roles_new', 'share_user', 'assign_roles']) {
      const used = node(id).executor;

      expect(offered.has(`${used?.name}/${used?.mode ?? ''}`), id).toBe(true);
    }
  });

  it('should write nothing when the invitee declines', () => {
    for (const branch of ['existing', 'new']) {
      const decline = actionTarget(`prompt_accept_${branch}`, `action_decline_${branch}`);

      expect(decline, branch).toBeDefined();

      const path = reachableFrom(decline ?? '');
      path.forEach((id) => expect(WRITING_EXECUTORS).not.toContain(node(id).executor?.name));
      expect(path.has('end')).toBe(true);
    }
  });
});
