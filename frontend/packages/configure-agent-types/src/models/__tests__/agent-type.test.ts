// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import type {AgentTypeListItem, ApiAgentType, SystemAttributes} from '../agent-type';

describe('agent-type types', () => {
  it('accepts SystemAttributes shape', () => {
    const sys: SystemAttributes = {display: 'username'};
    expect(sys.display).toBe('username');
  });

  it('accepts ApiAgentType shape', () => {
    const t: ApiAgentType = {
      id: 'a1',
      handle: 'default',
      displayName: 'Default',
      ouId: 'ou1',
      schema: {foo: {type: 'string'}},
    };
    expect(t.handle).toBe('default');
    expect(t.displayName).toBe('Default');
  });

  it('accepts ApiAgentType with optional ouHandle and systemAttributes', () => {
    const t: ApiAgentType = {
      id: 'a1',
      handle: 'default',
      displayName: 'Default',
      ouId: 'ou1',
      ouHandle: 'default',
      systemAttributes: {display: 'username'},
      schema: {},
    };
    expect(t.ouHandle).toBe('default');
    expect(t.systemAttributes?.display).toBe('username');
  });

  it('accepts AgentTypeListItem shape', () => {
    const item: AgentTypeListItem = {
      id: 'a1',
      handle: 'default',
      displayName: 'Default',
      ouId: 'ou1',
    };
    expect(item.id).toBe('a1');
  });
});
