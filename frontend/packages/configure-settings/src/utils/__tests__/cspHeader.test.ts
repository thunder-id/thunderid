// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import {CSP_DEFAULT_SCOPE} from '../../models/cspDirectiveRow';
import {buildScopeRows, createCspRow} from '../cspDirectiveRows';
import buildCspHeader from '../cspHeader';

const BASELINE =
  "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; connect-src 'self'; " +
  "frame-ancestors 'none'; base-uri 'none'; form-action 'self'";

describe('buildCspHeader', () => {
  it('renders the baseline when nothing is configured', () => {
    expect(buildCspHeader(buildScopeRows(undefined, {}, CSP_DEFAULT_SCOPE), false)).toBe(
      `Content-Security-Policy: ${BASELINE}`,
    );
  });

  it('uses the report-only header name and appends the report URI', () => {
    expect(buildCspHeader(buildScopeRows(undefined, {}, CSP_DEFAULT_SCOPE), true, '/csp-report')).toBe(
      `Content-Security-Policy-Report-Only: ${BASELINE}; report-uri /csp-report`,
    );
  });

  it('renders overrides in place and other directives sorted at the end, skipping empty ones', () => {
    const rows = buildScopeRows(
      {
        directives: {'img-src': ["'self'", 'data:']},
        paths: [{location: '/console/', directives: {'worker-src': ["'self'", 'blob:']}}],
      },
      {
        [CSP_DEFAULT_SCOPE]: [createCspRow('font-src', ["'self'"])],
        '/console/': [createCspRow('connect-src', ["'self'", 'https://api.example.com']), createCspRow('media-src')],
      },
      '/console/',
    );

    expect(buildCspHeader(rows, false)).toBe(
      "Content-Security-Policy: default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; " +
        "connect-src 'self' https://api.example.com; frame-ancestors 'none'; base-uri 'none'; form-action 'self'; " +
        "font-src 'self'; worker-src 'self' blob:",
    );
  });
});
