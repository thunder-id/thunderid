/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {describe, it, expect} from 'vitest';
import type {AgentTypeListResponse} from '../responses';

describe('agent-types response types', () => {
  it('accepts AgentTypeListResponse shape', () => {
    const list: AgentTypeListResponse = {
      totalResults: 1,
      startIndex: 0,
      count: 1,
      types: [{id: 'a1', name: 'default', ouId: 'ou1'}],
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
