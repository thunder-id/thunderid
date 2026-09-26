// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
