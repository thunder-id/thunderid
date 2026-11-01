// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import { NextResponse } from 'next/server';
import { getRequestUser, hasScope, WayfinderUser } from '@/lib/auth';
import config from '@/lib/config';
import { callManagementApi } from '@/lib/management';

// The ThunderID admin components reach users, groups, roles, connections, flows and user types here
// instead of on ThunderID: the app passes these URLs as their `endpoints` (see src/app/layout.tsx), so
// no user token is ever sent to the ThunderID management API or needs a ThunderID permission. This
// route checks that the caller's token holds the app's permission for the call, lets through only the
// calls the admin pages make, and pins each one to the caller's own company before forwarding it with
// the app's backend client, using a token issued for that company.

type Params = { params: Promise<{ path: string[] }> };

const ENTERPRISE_IDP_VARIABLE = 'enterpriseIdp';

// The Wayfinder resource the permission for each collection belongs to. Reads need `<resource>:view`
// and changes need `<resource>:manage`.
const PERMISSION_RESOURCES: Record<string, string> = {
  connections: 'sso',
  flows: 'sso',
  groups: 'group',
  roles: 'role',
  'user-types': 'user',
  users: 'user',
};

const notFound = () => NextResponse.json({ error: 'Not found.' }, { status: 404 });

const ouFilter = (user: WayfinderUser) => `ouId eq "${user.ouId}"`;

/** Copies the paging and display parameters the components send, and nothing else. */
const pick = (search: URLSearchParams, names: string[]): URLSearchParams => {
  const picked = new URLSearchParams();
  names.forEach((name) => {
    const value = search.get(name);
    if (value !== null) {
      picked.set(name, value);
    }
  });
  return picked;
};

const readJson = async (request: Request): Promise<Record<string, unknown>> => {
  try {
    const body = await request.json();
    return body && typeof body === 'object' ? body : {};
  } catch {
    return {};
  }
};

const handle = async (request: Request, { params }: Params): Promise<Response> => {
  const user = await getRequestUser(request);
  if (!user) {
    return NextResponse.json({ error: 'Sign in again.' }, { status: 401 });
  }

  const { path } = await params;
  const search = new URL(request.url).searchParams;
  const method = request.method;
  const [collection, id, ...rest] = path;
  const subPath = rest.join('/');

  const resource = PERMISSION_RESOURCES[collection];
  if (!resource) {
    return notFound();
  }
  if (!hasScope(user, `${resource}:${method === 'GET' ? 'view' : 'manage'}`)) {
    return NextResponse.json({ error: 'You do not have permission to do this.' }, { status: 403 });
  }

  if (collection === 'users' && id === undefined) {
    if (method === 'GET') {
      const query = pick(search, ['include', 'limit', 'offset']);
      query.set('filter', ouFilter(user));
      return callManagementApi(user.ouId, 'GET', 'users', query);
    }
    if (method === 'POST') {
      // Users of the app's own type only, created in the caller's company.
      const { type, attributes } = await readJson(request);
      if (type !== config.userType) {
        return NextResponse.json({ error: 'Choose the Wayfinder user type.' }, { status: 400 });
      }
      return callManagementApi(user.ouId, 'POST', 'users', undefined, { type, attributes, ouId: user.ouId });
    }
    return notFound();
  }

  if (collection === 'groups') {
    if (id === undefined) {
      if (method === 'GET') {
        const query = pick(search, ['include', 'limit', 'offset']);
        query.set('filter', ouFilter(user));
        return callManagementApi(user.ouId, 'GET', 'groups', query);
      }
      if (method === 'POST') {
        const { name, description } = await readJson(request);
        return callManagementApi(user.ouId, 'POST', 'groups', undefined, { name, description, ouId: user.ouId });
      }
      return notFound();
    }

    // Next.js decodes path segments, so an encoded id could otherwise climb out of /groups.
    if (id === '.' || id === '..') {
      return notFound();
    }
    const groupPath = `groups/${encodeURIComponent(id)}`;

    // A company reads and changes only its own groups. ThunderID accepts only members of the group's company.
    const group = await callManagementApi(user.ouId, 'GET', groupPath, pick(search, ['include']));
    if (!group.ok || (await group.clone().json()).ouId !== user.ouId) {
      return group.ok ? notFound() : group;
    }
    if (method === 'GET' && subPath === '') {
      return group;
    }
    if (method === 'GET' && subPath === 'members') {
      return callManagementApi(user.ouId, 'GET', `${groupPath}/members`, pick(search, ['include', 'limit', 'offset']));
    }
    if (method === 'POST' && (subPath === 'members/add' || subPath === 'members/remove')) {
      const { members } = await readJson(request);
      return callManagementApi(user.ouId, 'POST', `${groupPath}/${subPath}`, undefined, { members });
    }
    return notFound();
  }

  if (collection === 'roles') {
    const roleIds = [config.adminRoleId, config.memberRoleId];

    if (method === 'GET' && id === undefined) {
      const query = pick(search, ['limit', 'offset']);
      query.set('filter', ouFilter(user));
      const response = await callManagementApi(user.ouId, 'GET', 'roles', query);
      if (!response.ok) {
        return response;
      }
      // Lists only the two roles the app manages.
      const page = await response.json();
      const roles = (page.roles ?? []).filter((role: { id: string }) => roleIds.includes(role.id));
      return NextResponse.json({ ...page, roles, count: roles.length, totalResults: roles.length });
    }

    if (!roleIds.includes(id)) {
      return notFound();
    }
    if (method === 'GET' && subPath === '') {
      return callManagementApi(user.ouId, 'GET', `roles/${id}`, new URLSearchParams({ ouId: user.ouId }));
    }
    if (method === 'GET' && subPath === 'assignments') {
      const query = pick(search, ['include', 'type', 'limit', 'offset']);
      query.set('filter', ouFilter(user));
      return callManagementApi(user.ouId, 'GET', `roles/${id}/assignments`, query);
    }
    if (method === 'POST' && (subPath === 'assignments/add' || subPath === 'assignments/remove')) {
      // ThunderID accepts only assignees the company owns or was shared.
      const { assignments } = await readJson(request);
      return callManagementApi(user.ouId, 'POST', `roles/${id}/${subPath}`, undefined, { assignments, ouId: user.ouId });
    }
    return notFound();
  }

  if (collection === 'connections') {
    if (method === 'GET' && id === undefined) {
      const query = pick(search, ['category', 'limit', 'offset']);
      query.set('filter', ouFilter(user));
      return callManagementApi(user.ouId, 'GET', 'connections', query);
    }
    if (method === 'POST' && id === 'oidc' && subPath === '') {
      const connection = await readJson(request);
      return callManagementApi(user.ouId, 'POST', 'connections/oidc', undefined, { ...connection, ouId: user.ouId });
    }
    return notFound();
  }

  if (collection === 'user-types' && method === 'GET' && id === undefined) {
    // Admins create users of the app's own user type only.
    const response = await callManagementApi(user.ouId, 'GET', 'user-types', new URLSearchParams({ limit: '100' }));
    if (!response.ok) {
      return response;
    }
    const page = await response.json();
    const types = (page.types ?? []).filter((type: { handle: string }) => type.handle === config.userType);
    return NextResponse.json({ ...page, types, count: types.length, totalResults: types.length });
  }

  if (collection === 'flows' && id === config.signInFlowId && subPath === '' && method === 'GET') {
    // The sign-in flow as the company sees it: the variables it may set, with their labels and descriptions.
    return callManagementApi(user.ouId, 'GET', `flows/${id}`, new URLSearchParams({ ouId: user.ouId }));
  }

  if (collection === 'flows' && id === config.signInFlowId && subPath === 'overlay-rules' && method === 'GET') {
    // What the company may do with each variable of the sign-in flow, from the flow's sharing policy.
    return callManagementApi(user.ouId, 'GET', `flows/${id}/overlay-rules`, new URLSearchParams({ ouId: user.ouId }));
  }

  if (collection === 'flows' && id === config.signInFlowId && subPath === 'variables') {
    if (method === 'GET') {
      return callManagementApi(user.ouId, 'GET', `flows/${id}/variables`, new URLSearchParams({ ouId: user.ouId }));
    }
    if (method === 'PUT') {
      // An admin sets only the identity provider, and only for their own company.
      const { values } = await readJson(request);
      const connectionId = (values as Record<string, unknown> | undefined)?.[ENTERPRISE_IDP_VARIABLE];
      return callManagementApi(user.ouId, 'PUT', `flows/${id}/variables`, undefined, {
        ouId: user.ouId,
        values: typeof connectionId === 'string' ? { [ENTERPRISE_IDP_VARIABLE]: connectionId } : {},
      });
    }
  }

  return notFound();
};

export { handle as GET, handle as POST, handle as PUT };
