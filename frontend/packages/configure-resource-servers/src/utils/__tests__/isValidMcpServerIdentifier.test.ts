// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {isValidMcpServerIdentifier} from '../isValidMcpServerIdentifier';

describe('isValidMcpServerIdentifier', () => {
  it.each([
    'https://mcp.example.com/mcp',
    'http://localhost:8000/mcp',
    'https://mcp.example.com',
    'HTTPS://MCP.EXAMPLE.COM/mcp',
    'https://mcp.example.com/mcp?tenant=acme',
  ])('should accept the http or https URL %s', (value) => {
    expect(isValidMcpServerIdentifier(value)).toBe(true);
  });

  it.each(['my-mcp-server', 'not a uri', '/mcp', 'mcp.example.com', '//mcp.example.com/mcp'])(
    'should reject %s, which has no scheme',
    (value) => {
      expect(isValidMcpServerIdentifier(value)).toBe(false);
    },
  );

  it.each(['localhost:8000/mcp', 'mcp.example.com:8443/mcp', 'urn:example:mcp', 'ftp://mcp.example.com'])(
    'should reject %s, which is not an http or https URL',
    (value) => {
      expect(isValidMcpServerIdentifier(value)).toBe(false);
    },
  );

  it.each(['https:mcp.example.com', 'https:/mcp.example.com', 'https:///mcp.example.com', 'https://'])(
    'should reject %s, which has no authority',
    (value) => {
      expect(isValidMcpServerIdentifier(value)).toBe(false);
    },
  );

  it('should reject a percent-encoded host, which the backend refuses', () => {
    expect(isValidMcpServerIdentifier('https://%65xample.com/mcp')).toBe(false);
  });

  it('should accept percent-encoded userinfo with a plain host', () => {
    expect(isValidMcpServerIdentifier('https://user%40example.com@example.com/mcp')).toBe(true);
  });

  it.each(['https://mcp.example.com\\mcp', 'https://mcp.example.com/a\\b'])(
    'should reject %s, which has a backslash the browser rewrites to a slash',
    (value) => {
      expect(isValidMcpServerIdentifier(value)).toBe(false);
    },
  );

  it.each(['https://mcp.example.com/#x', 'https://mcp.example.com/mcp#section', 'https://mcp.example.com/mcp#'])(
    'should reject %s, which has a fragment',
    (value) => {
      expect(isValidMcpServerIdentifier(value)).toBe(false);
    },
  );

  it('should reject an empty value', () => {
    expect(isValidMcpServerIdentifier('')).toBe(false);
  });
});
