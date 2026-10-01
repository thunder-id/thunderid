// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import getTemplateFieldConstraints from '../getTemplateFieldConstraints';

describe('getTemplateFieldConstraints', () => {
  it('returns null for undefined input', () => {
    expect(getTemplateFieldConstraints(undefined)).toBeNull();
  });

  it('returns null for an empty string', () => {
    expect(getTemplateFieldConstraints('')).toBeNull();
  });

  it('returns null for an unknown template ID', () => {
    expect(getTemplateFieldConstraints('unknown-template')).toBeNull();
  });

  it('returns non-null fieldConstraints for the "react" template', () => {
    const constraints = getTemplateFieldConstraints('react');

    expect(constraints).not.toBeNull();
  });

  it('returns the same constraints for "react-embedded" as for "react" (normalized)', () => {
    const reactConstraints = getTemplateFieldConstraints('react');
    const embeddedConstraints = getTemplateFieldConstraints('react-embedded');

    expect(embeddedConstraints).toEqual(reactConstraints);
  });

  it('returns non-null fieldConstraints for the "browser" template', () => {
    const constraints = getTemplateFieldConstraints('browser');

    expect(constraints).not.toBeNull();
  });

  it('returns publicClient constraint as readOnly true with value true for the "react" template', () => {
    const constraints = getTemplateFieldConstraints('react');

    expect(constraints?.oauth2?.publicClient).toEqual({readOnly: true, value: true});
  });

  it('returns pkceRequired constraint as readOnly true with value true for the "react" template', () => {
    const constraints = getTemplateFieldConstraints('react');

    expect(constraints?.oauth2?.pkceRequired).toEqual({readOnly: true, value: true});
  });

  it('returns tokenEndpointAuthMethod constraint for the "react" template', () => {
    const constraints = getTemplateFieldConstraints('react');

    expect(constraints?.oauth2?.tokenEndpointAuthMethod).toBeDefined();
  });

  it('returns null for the "custom" template (no field constraints)', () => {
    expect(getTemplateFieldConstraints('custom')).toBeNull();
  });

  it('returns pkceRequired constraint as readOnly true with value true for the "mcp-client" template', () => {
    const constraints = getTemplateFieldConstraints('mcp-client');

    expect(constraints?.oauth2?.pkceRequired).toEqual({readOnly: true, value: true});
  });
});
