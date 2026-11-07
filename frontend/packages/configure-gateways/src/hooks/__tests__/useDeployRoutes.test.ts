// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {
  defaultDeployRoutePaths,
  gatewaySectionOf,
  projectHomeOf,
  resourceEditOf,
  userRoutesOf,
} from '../useDeployRoutes';

describe('resourceEditOf', () => {
  it.each([
    ['application', 'a 1', '/applications/a%201'],
    ['agent', 'g-1', '/agents/g-1'],
    ['agent_type', 't-1', '/agent-types/t-1'],
    ['flow', 'f-1', '/flows/f-1'],
    ['connection', 'c-1', '/connections'],
    ['resource_server', 'r-1', '/resource-servers/r-1'],
    ['translation', 'en-US', '/translations/en-US'],
    ['theme', 'th-1', '/design/themes/th-1'],
    ['layout', 'l-1', '/design/layouts/l-1'],
    ['user', 'u-1', '/users/u-1'],
    ['group', 'gr-1', '/groups/gr-1'],
    ['role', 'ro-1', '/roles/ro-1'],
    ['user_type', 'ut-1', '/user-types/ut-1'],
    ['organization_unit', 'ou-1', '/organization-units/ou-1'],
    ['notification_template', 'n-1', '/notification-templates'],
    ['server_config', 'gateway', '/settings'],
    ['credential_configuration', 'vc-1', '/verifiable-credentials/vc-1'],
    ['presentation_definition', 'vp-1', '/verifiable-presentations/vp-1'],
  ])('edits a %s on its own page', (resourceType, id, path) => {
    expect(resourceEditOf(defaultDeployRoutePaths, resourceType, id)).toBe(path);
  });

  it('offers nothing for a kind the console does not edit', () => {
    expect(resourceEditOf(defaultDeployRoutePaths, 'authorization_detail_type', 'x')).toBeUndefined();
    expect(resourceEditOf(defaultDeployRoutePaths, 'project', 'p')).toBeUndefined();
  });

  it("uses the console's own mapping when it routes resources", () => {
    const routes = {...defaultDeployRoutePaths, deploy: {...defaultDeployRoutePaths.deploy, resource: () => '/mine'}};
    expect(resourceEditOf(routes, 'flow', 'f-1')).toBe('/mine');
  });
});

describe('gateway, project and administrator routes', () => {
  it("places a gateway's sections under it, within a project when given", () => {
    expect(gatewaySectionOf(defaultDeployRoutePaths, 'gw 1', undefined, 'overview')).toBe('/deploy/gateways/gw%201');
    expect(gatewaySectionOf(defaultDeployRoutePaths, 'gw-1', 'p-1', 'values')).toBe(
      '/projects/p-1/deploy/gateways/gw-1/values',
    );
  });

  it("finds a project's home as the console routes it, or by default", () => {
    expect(projectHomeOf(defaultDeployRoutePaths, 'p 1')).toBe('/projects/p%201');
    const deploy = {...defaultDeployRoutePaths.deploy, project: undefined};
    expect(projectHomeOf({...defaultDeployRoutePaths, deploy}, 'p-1')).toBe('/projects/p-1');
    const routed = {...defaultDeployRoutePaths, deploy: {...deploy, project: (id: string) => `/p/${id}`}};
    expect(projectHomeOf(routed, 'p-1')).toBe('/p/p-1');
  });

  it('finds the administrators as the console routes them, or by default', () => {
    expect(userRoutesOf(defaultDeployRoutePaths).detail('u 1')).toBe('/users/u%201');
    expect(userRoutesOf(defaultDeployRoutePaths).list()).toBe('/users');
    const users = {list: () => '/admins', detail: (id: string) => `/admins/${id}`};
    expect(userRoutesOf({...defaultDeployRoutePaths, users}).detail('u-1')).toBe('/admins/u-1');
  });
});
