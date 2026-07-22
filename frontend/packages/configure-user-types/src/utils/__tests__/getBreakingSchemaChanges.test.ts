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

import {describe, expect, it} from 'vitest';
import getBreakingSchemaChanges from '../getBreakingSchemaChanges';

describe('getBreakingSchemaChanges', () => {
  it('flags removed, newly-required, and tightened attributes', () => {
    const base = {
      removed: {type: 'string'},
      role: {type: 'string'},
      status: {type: 'string', enum: ['ACTIVE', 'INACTIVE']},
      code: {type: 'string'},
      id: {type: 'string'},
    };
    const next = {
      role: {type: 'number'}, // type change
      status: {type: 'string', enum: ['ACTIVE']}, // enum narrowed
      code: {type: 'string', regex: '^[0-9]+$'}, // regex added
      id: {type: 'string', unique: true}, // unique added
      added: {type: 'string', required: true}, // new required
    };
    expect(getBreakingSchemaChanges(base, next)).toEqual(['added', 'code', 'id', 'removed', 'role', 'status']);
  });

  it('ignores additive and loosening changes', () => {
    const base = {
      role: {type: 'string', required: true},
      status: {type: 'string', enum: ['ACTIVE']},
    };
    const next = {
      role: {type: 'string', required: false}, // relaxed
      status: {type: 'string', enum: ['ACTIVE', 'INACTIVE']}, // widened
      optional: {type: 'string'}, // new optional
    };
    expect(getBreakingSchemaChanges(base, next)).toEqual([]);
  });
});
