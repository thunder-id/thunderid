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
import TemplateConstants from '../template-constants';

describe('TemplateConstants', () => {
  it('should export valid constants object', () => {
    expect(TemplateConstants).toBeDefined();
    expect(typeof TemplateConstants).toBe('object');
  });

  it('should have EMBEDDED_SUFFIX constant', () => {
    expect(TemplateConstants).toHaveProperty('EMBEDDED_SUFFIX');
    expect(TemplateConstants.EMBEDDED_SUFFIX).toBe('-embedded');
  });

  it('should have MCP_CLIENT_TEMPLATE_ID constant', () => {
    expect(TemplateConstants).toHaveProperty('MCP_CLIENT_TEMPLATE_ID');
    expect(TemplateConstants.MCP_CLIENT_TEMPLATE_ID).toBe('mcp-client');
  });

  it('should have MCP_CLIENT_ALLOWED_GRANT_TYPES constant', () => {
    expect(TemplateConstants).toHaveProperty('MCP_CLIENT_ALLOWED_GRANT_TYPES');
    expect(TemplateConstants.MCP_CLIENT_ALLOWED_GRANT_TYPES).toEqual([
      'authorization_code',
      'refresh_token',
      'client_credentials',
    ]);
  });
});
