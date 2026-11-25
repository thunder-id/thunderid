// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import type {CreateAgentTypeRequest, UpdateAgentTypeRequest} from '../requests';

describe('agent-types request types', () => {
  it('accepts CreateAgentTypeRequest shape', () => {
    const req: CreateAgentTypeRequest = {
      handle: 'assistant',
      displayName: 'Assistant',
      ouId: 'ou1',
      systemAttributes: {display: 'name'},
      schema: {name: {type: 'string'}},
    };
    expect(req.handle).toBe('assistant');
  });

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
});
