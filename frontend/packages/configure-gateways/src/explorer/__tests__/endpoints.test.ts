// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import type {AppliedResource} from '../../models/deploy';
import {endpointsOf, gatewayEndpoints} from '../endpoints';
import {buildGraph, type GraphNode} from '../model';

const nodeOf = (resource: AppliedResource): GraphNode => {
  const node = buildGraph([resource], []).nodes.find((entry: GraphNode) => entry.id === resource.id);
  if (!node) throw new Error('no node');
  return node;
};
const urls = (node: GraphNode, baseUrl?: string): string[] => endpointsOf(node, baseUrl).map((e) => e.url);

describe('endpointsOf', () => {
  it('gives an application and an agent the sign-in and token endpoints of the gateway they run on', () => {
    const app = nodeOf({resourceType: 'application', id: 'a-1', resource: {id: 'a-1', name: 'Orders'}});
    expect(urls(app, 'https://dev.example.com/')).toEqual([
      'https://dev.example.com/.well-known/openid-configuration',
      'https://dev.example.com/oauth2/authorize',
      'https://dev.example.com/oauth2/token',
      'https://dev.example.com/oauth2/userinfo',
      'https://dev.example.com/oauth2/jwks',
      'https://dev.example.com/oauth2/logout',
      'https://dev.example.com/oauth2/introspect',
      'https://dev.example.com/oauth2/revoke',
    ]);
    const agent = nodeOf({resourceType: 'agent', id: 'g-1', resource: {id: 'g-1', name: 'Helper'}});
    expect(urls(agent, 'https://prod.example.com')).toContain('https://prod.example.com/oauth2/token');
  });

  it('gives a connection that signs users in the redirect URI its provider returns to, and a sender none', () => {
    const google = nodeOf({
      resourceType: 'connection',
      id: 'c-1',
      resource: {id: 'c-1', name: 'Google', type: 'google'},
    });
    expect(google.federated).toBe(true);
    expect(urls(google, 'https://dev.example.com')).toEqual(['https://dev.example.com/gate/callback']);
    const sms = nodeOf({resourceType: 'connection', id: 'c-2', resource: {id: 'c-2', name: 'SMS', type: 'twilio'}});
    expect(sms.federated).toBe(false);
    expect(urls(sms, 'https://dev.example.com')).toEqual([]);
  });

  it('gives other resources none, and nothing without the gateway`s address', () => {
    const role = nodeOf({resourceType: 'role', id: 'r-1', resource: {id: 'r-1', name: 'Admin'}});
    expect(urls(role, 'https://dev.example.com')).toEqual([]);
    const app = nodeOf({resourceType: 'application', id: 'a-1', resource: {id: 'a-1', name: 'Orders'}});
    expect(urls(app, undefined)).toEqual([]);
    expect(gatewayEndpoints(undefined)).toEqual([]);
  });

  it('describes a gateway by where it is discovered and its tokens verified', () => {
    expect(gatewayEndpoints('https://dev.example.com/').map((e) => e.url)).toEqual([
      'https://dev.example.com/.well-known/openid-configuration',
      'https://dev.example.com/oauth2/jwks',
    ]);
  });
});
