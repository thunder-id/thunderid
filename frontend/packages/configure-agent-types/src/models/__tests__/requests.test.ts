// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import type {AgentTypeListParams, UpdateAgentTypeRequest} from '../requests';

describe('agent-types request types', () => {
  it('accepts UpdateAgentTypeRequest shape', () => {
    const req: UpdateAgentTypeRequest = {
      displayName: 'Default',
      ouId: 'ou1',
      schema: {foo: {type: 'string'}},
    };
    expect(req.displayName).toBe('Default');
  });

  it('accepts UpdateAgentTypeRequest with optional systemAttributes', () => {
    const req: UpdateAgentTypeRequest = {
      displayName: 'Default',
      ouId: 'ou1',
      systemAttributes: {display: 'name'},
      schema: {},
    };
    expect(req.systemAttributes?.display).toBe('name');
  });

  it('accepts AgentTypeListParams pagination shape', () => {
    const params: AgentTypeListParams = {limit: 10, offset: 0};
    expect(params.limit).toBe(10);
    expect(params.offset).toBe(0);
  });

  it('accepts empty AgentTypeListParams', () => {
    const params: AgentTypeListParams = {};
    expect(params).toEqual({});
  });
});
