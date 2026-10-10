// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import type {AgentTypeListResponse} from '../responses';

describe('agent-types response types', () => {
  it('accepts AgentTypeListResponse shape', () => {
    const list: AgentTypeListResponse = {
      totalResults: 1,
      startIndex: 0,
      count: 1,
      types: [{id: 'a1', handle: 'default', displayName: 'Default', ouId: 'ou1'}],
    };
    expect(list.types).toHaveLength(1);
  });

  it('accepts AgentTypeListResponse with optional pagination links', () => {
    const list: AgentTypeListResponse = {
      totalResults: 0,
      startIndex: 0,
      count: 0,
      types: [],
      links: [{href: 'https://example.com/agent-types?offset=10', rel: 'next'}],
    };
    expect(list.links).toHaveLength(1);
  });
});
