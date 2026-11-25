// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {isValidProcessorEndpoint, processorEndpointProblem} from '../authorization-detail-schema';

describe('processorEndpointProblem', () => {
  it('accepts the endpoints the server accepts', () => {
    for (const endpoint of [
      'https://api.example.com/authorize',
      'https://api.example.com:8443/a/b?x=1&y=2',
      'https://api.example.com/a#frag',
      'https://api.example.com/p with space',
      'https://api.example.com/p?q=%zz',
      'https://api.example.com/p%20q',
      'http://localhost:9099/p',
      'http://[::1]:9099/p',
      'https://bücher.example/p',
      'HTTPS://API.EXAMPLE.COM/P',
    ]) {
      expect(processorEndpointProblem(endpoint), endpoint).toBeNull();
      expect(isValidProcessorEndpoint(endpoint), endpoint).toBe(true);
    }
  });

  it.each([
    ['https://api.example.com/%zz', 'invalid'],
    ['https://api.example.com/a#%zz', 'invalid'],
    ['https://exa mple.com/p', 'invalid'],
    ['https://exa%20mple.com/p', 'invalid'],
    ['https://api.example.com/a\tb', 'invalid'],
    ['https:/api.example.com', 'invalid'],
    ['https://', 'invalid'],
    ['api.example.com/p', 'invalid'],
    ['ftp://api.example.com/p', 'scheme'],
    ['http://example.com/p', 'scheme'],
    ['http://127.1/p', 'scheme'],
    ['https://user:pw@api.example.com/p', 'credentials'],
    ['https://api.example.com:0/p', 'port'],
    ['https://api.example.com:65536/p', 'port'],
    ['https://api.example.com:80a/p', 'port'],
  ])('refuses %s as %s, as the server does', (endpoint, problem) => {
    expect(processorEndpointProblem(endpoint)).toBe(problem);
    expect(isValidProcessorEndpoint(endpoint)).toBe(false);
  });
});
