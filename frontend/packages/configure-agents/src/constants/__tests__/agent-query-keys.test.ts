// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import AgentQueryKeys from '../agent-query-keys';

describe('AgentQueryKeys', () => {
  it('should export valid query keys object', () => {
    expect(AgentQueryKeys).toBeDefined();
    expect(typeof AgentQueryKeys).toBe('object');
  });

  it('should have agents key', () => {
    expect(AgentQueryKeys).toHaveProperty('AGENTS');
    expect(AgentQueryKeys.AGENTS).toBe('agents');
  });

  it('should have agent key', () => {
    expect(AgentQueryKeys).toHaveProperty('AGENT');
    expect(AgentQueryKeys.AGENT).toBe('agent');
  });
});
