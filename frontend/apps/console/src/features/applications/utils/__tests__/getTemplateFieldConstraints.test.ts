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
});
